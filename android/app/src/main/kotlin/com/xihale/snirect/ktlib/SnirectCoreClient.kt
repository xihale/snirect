package com.xihale.snirect.ktlib

import android.content.Context
import core.Core
import core.EngineCallbacks
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.io.ByteArrayInputStream
import java.security.KeyStore
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

/**
 * Thread-safe, coroutine-aware entry point to the Snirect engine.
 *
 * - Every gomobile (JNI) call runs on [Dispatchers.IO]; the public API is
 *   `suspend` so none of it can pin the main thread.
 * - Engine lifecycle callbacks arrive on arbitrary Go runtime threads; they are
 *   marshalled onto [callbackDispatcher] (default `Main.immediate`) before
 *   touching [engineState], so state updates are applied in arrival order on a
 *   single dispatcher.
 * - [engineState] is the Kotlin-side single source of truth for the UI.
 */
class SnirectClient private constructor(
    private val dataDir: String,
    private val callbackDispatcher: CoroutineDispatcher,
) {
    private val stateLock = Any()

    private val _engineState = MutableStateFlow<EngineState>(EngineState.Idle)

    /** Kotlin-side engine state — the single source of truth for the UI. */
    val engineState: StateFlow<EngineState> = _engineState.asStateFlow()

    /** Serializes start/stop so concurrent callers cannot interleave engine ops. */
    private val engineMutex = Mutex()

    /** Marshals Go-thread callback arrivals onto [callbackDispatcher]. */
    private val callbackScope = CoroutineScope(SupervisorJob() + callbackDispatcher)

    init {
        SnirectCoreBridge.initialize(dataDir)
    }

    /**
     * Starts the engine. Non-blocking: returns as soon as the request has been
     * handed to the core (heavy init, incl. RSA CA keygen, happens on the Go
     * side); the outcome arrives via [engineState]. The state moves to
     * [EngineState.Starting] immediately (resetting any terminal state from a
     * previous run) — await the outcome with [awaitStartup].
     */
    suspend fun startEngine(
        tunFd: Long,
        config: SnirectEngineConfig,
        callbacks: SnirectEngineCallbacks,
    ) {
        engineMutex.withLock {
            // Apply Starting synchronously on this caller, BEFORE the JNI
            // hand-off. postState() launches onto Main and would race
            // awaitStartup: a leftover Running from the previous session
            // would match immediately and the host would treat a dead
            // engine as "up", then tear the new TUN down under it.
            synchronized(stateLock) {
                _engineState.value = EngineState.Starting
            }
            withContext(Dispatchers.IO) {
                try {
                    SnirectCoreBridge.startEngine(tunFd, config, callbacks, ::handleEngineEvent)
                } catch (t: Throwable) {
                    synchronized(stateLock) {
                        _engineState.value = EngineState.Failed(t.message ?: "startEngine failed")
                    }
                    throw t
                }
            }
        }
    }

    /**
     * Stops the engine. Idempotent: stopping when no engine is running is a
     * no-op. Runs on Dispatchers.IO — proxy shutdown waits at most 1s.
     * The resulting [EngineState.Stopped] arrives via [engineState].
     */
    suspend fun stopEngine() {
        engineMutex.withLock {
            withContext(Dispatchers.IO) {
                SnirectCoreBridge.stopEngine()
            }
        }
    }

    /**
     * Suspends until the engine leaves [EngineState.Starting], i.e. until it is
     * [EngineState.Running], [EngineState.Failed], or [EngineState.Stopped]
     * (an explicit stop that cancelled a still-starting engine). On [timeoutMs]
     * expiry the state is moved to `Failed("startup timed out")` (unless a
     * terminal state arrived concurrently) and returned — a startup watchdog.
     */
    suspend fun awaitStartup(timeoutMs: Long = DEFAULT_STARTUP_TIMEOUT_MS): EngineState {
        val outcome = withTimeoutOrNull(timeoutMs) {
            engineState.first {
                it is EngineState.Running ||
                    it is EngineState.Failed ||
                    it is EngineState.Stopped
            }
        }
        if (outcome != null) return outcome
        synchronized(stateLock) {
            val current = _engineState.value
            if (current is EngineState.Running ||
                current is EngineState.Failed ||
                current is EngineState.Stopped
            ) {
                return current
            }
            _engineState.value = EngineState.Failed("startup timed out")
        }
        return EngineState.Failed("startup timed out")
    }

    /**
     * Exports the CA certificate (PEM). Runs on Dispatchers.IO: on first run
     * the core performs RSA key generation — the suspend form makes sure that
     * can never run on the main thread.
     */
    suspend fun caCertificate(): ByteArray? = withContext(Dispatchers.IO) {
        SnirectCoreBridge.getCaCertificate()
    }

    /** See [caCertificate]: RSA keygen on first run — IO only. */
    suspend fun isCaCertificateInstalled(): Boolean = withContext(Dispatchers.IO) {
        SnirectCoreBridge.isCaCertificateInstalled()
    }

    /** Looks up the latest GitHub release. Does not download or install. */
    suspend fun checkUpdate(current: String): AppUpdate = withContext(Dispatchers.IO) {
        SnirectCoreBridge.checkUpdate(current)
    }

    /** Entry point for the gomobile adapter; called on Go runtime threads. */
    internal fun handleEngineEvent(event: EngineEvent) {
        callbackScope.launch {
            postStateLocked(
                when (event) {
                    EngineEvent.Started -> EngineState.Running
                    is EngineEvent.Error -> EngineState.Failed(event.reason)
                    is EngineEvent.Stopped -> EngineState.Stopped(event.reason)
                }
            )
        }
    }

    private fun postStateLocked(newState: EngineState) {
        synchronized(stateLock) {
            val current = _engineState.value
            // Failed is terminal until the next user-initiated start resets it.
            if (current is EngineState.Failed && newState !is EngineState.Starting) return
            _engineState.value = newState
        }
    }

    companion object {
        const val DEFAULT_STARTUP_TIMEOUT_MS = 15_000L

        @Volatile
        private var sharedClient: SnirectClient? = null

        @Volatile
        private var sharedDataDir: String? = null

        fun from(context: Context): SnirectClient = fromDataDir(context.filesDir.absolutePath)

        fun fromDataDir(dataDir: String): SnirectClient {
            val current = sharedClient
            if (current != null && sharedDataDir == dataDir) {
                return current
            }

            synchronized(this) {
                val latest = sharedClient
                if (latest != null && sharedDataDir == dataDir) {
                    return latest
                }

                return SnirectClient(dataDir, Dispatchers.Main.immediate).also {
                    sharedClient = it
                    sharedDataDir = dataDir
                }
            }
        }
    }
}

/** Lifecycle events forwarded from the gomobile callbacks to the state machine. */
internal sealed interface EngineEvent {
    data object Started : EngineEvent
    data class Error(val reason: String) : EngineEvent
    data class Stopped(val reason: String) : EngineEvent
}

@Serializable
internal data class CoreConfig(
    @SerialName("nameservers") val nameservers: List<String>? = null,
    @SerialName("bootstrap_dns") val bootstrapDns: List<String>? = null,
    @SerialName("check_hostname") val checkHostname: Boolean = false,
    @SerialName("mtu") val mtu: Int = 1500,
    @SerialName("enable_ipv6") val enableIpv6: Boolean = false,
    @SerialName("log_level") val logLevel: String = "info",
)

internal object SnirectCoreBridge {
    @Volatile
    private var initializedDataDir: String? = null

    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    fun initialize(dataDir: String) {
        if (initializedDataDir == dataDir) {
            return
        }

        synchronized(this) {
            if (initializedDataDir != dataDir) {
                Core.setDataDir(dataDir)
                initializedDataDir = dataDir
            }
        }
    }

    fun getCaCertificate(): ByteArray? {
        requireInitialized()
        return Core.getCACertificate()
    }

    fun isCaCertificateInstalled(): Boolean {
        val certBytes = getCaCertificate() ?: return false
        if (certBytes.isEmpty()) return false

        val certFactory = CertificateFactory.getInstance("X.509")
        val caCertificate = certFactory.generateCertificate(ByteArrayInputStream(certBytes)) as X509Certificate

        val keyStore = KeyStore.getInstance("AndroidCAStore")
        keyStore.load(null)
        val aliases = keyStore.aliases()
        while (aliases.hasMoreElements()) {
            val alias = aliases.nextElement()
            val installed = keyStore.getCertificate(alias) as? X509Certificate ?: continue
            if (
                installed.issuerX500Principal == caCertificate.subjectX500Principal &&
                installed.publicKey == caCertificate.publicKey
            ) {
                return true
            }
        }

        return false
    }

    fun startEngine(
        tunFd: Long,
        config: SnirectEngineConfig,
        callbacks: SnirectEngineCallbacks,
        onEngineEvent: (EngineEvent) -> Unit,
    ) {
        requireInitialized()
        Core.startEngine(
            tunFd,
            json.encodeToString(config.toCoreConfig()),
            EngineCallbacksAdapter(callbacks, onEngineEvent)
        )
    }

    fun stopEngine() {
        requireInitialized()
        Core.stopEngine()
    }

    fun checkUpdate(current: String): AppUpdate {
        val info = Core.checkUpdate(current)
            ?: throw IllegalStateException("core returned null update info")
        return AppUpdate(
            current = info.current.orEmpty(),
            latest = info.latest.orEmpty(),
            newer = info.newer,
            url = info.url.orEmpty(),
            notes = info.notes.orEmpty(),
        )
    }

    private fun requireInitialized() {
        check(initializedDataDir != null) {
            "SnirectClient has not been initialized. Create it via SnirectClient.from(context) or SnirectClient.fromDataDir(dataDir)."
        }
    }

    /**
     * Adapts the app-facing [SnirectEngineCallbacks] to the generated gomobile
     * [EngineCallbacks] interface. Log lines and socket protection go straight
     * to the delegate; the engine lifecycle callbacks additionally feed the
     * client's state machine via [onEngineEvent].
     */
    private class EngineCallbacksAdapter(
        private val delegate: SnirectEngineCallbacks,
        private val onEngineEvent: (EngineEvent) -> Unit,
    ) : EngineCallbacks {
        override fun onStatusChanged(status: String?) {
            delegate.onStatusChanged(status)
        }

        override fun protect(fd: Long): Boolean = delegate.protect(fd)

        override fun onEngineStarted() {
            onEngineEvent(EngineEvent.Started)
            delegate.onEngineStarted()
        }

        override fun onEngineError(reason: String) {
            onEngineEvent(EngineEvent.Error(reason))
            delegate.onEngineError(reason)
        }

        override fun onEngineStopped(reason: String) {
            onEngineEvent(EngineEvent.Stopped(reason))
            delegate.onEngineStopped(reason)
        }
    }

    private fun SnirectEngineConfig.toCoreConfig(): CoreConfig = CoreConfig(
        nameservers = nameservers,
        bootstrapDns = bootstrapDns,
        checkHostname = checkHostname,
        mtu = mtu,
        enableIpv6 = enableIpv6,
        logLevel = logLevel
    )
}
