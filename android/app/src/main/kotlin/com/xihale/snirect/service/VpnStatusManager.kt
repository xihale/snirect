package com.xihale.snirect.service

import com.xihale.snirect.ktlib.EngineState
import com.xihale.snirect.ktlib.SnirectClient
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.flow.onEach

/**
 * Bridges ktlib's [EngineState] (the single source of truth) into the app-wide
 * status used by the existing collectors (MainActivity/ViewModel, tile).
 *
 * [updateStatus] only contributes transient app-level intents around
 * service start/stop (STARTING/STOPPING) — engine-derived states always win.
 *
 * A `FAILED:<reason>` status is terminal: teardown-path DISCONNECTED updates
 * can never overwrite it; only the next user-initiated start (STARTING)
 * clears it.
 */
object VpnStatusManager {
    private val _isRunning = MutableStateFlow(false)
    val isRunning = _isRunning.asStateFlow()

    private val _statusText = MutableStateFlow("DISCONNECTED")
    val statusText = _statusText.asStateFlow()

    @Volatile
    private var failure: String? = null

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var attached = false

    /**
     * Starts mirroring [client]'s engine state. Idempotent; call from any
     * long-lived entry point (ViewModel init, VPN service onCreate, tile).
     */
    fun attach(client: SnirectClient) {
        synchronized(this) {
            if (attached) return
            attached = true
        }
        client.engineState
            .onEach(::onEngineState)
            .launchIn(scope)
    }

    /** Maps a ktlib engine state onto the app-wide status. */
    fun onEngineState(state: EngineState) {
        when (state) {
            EngineState.Idle -> Unit // nothing running yet; keep current app-level status
            EngineState.Starting -> updateStatus(false, "STARTING")
            EngineState.Running -> updateStatus(true, "ACTIVE")
            is EngineState.Failed -> updateStatus(false, "FAILED:${state.reason}")
            is EngineState.Stopped -> {
                // A user-initiated stop publishes STOPPING first. Do not jump
                // to DISCONNECTED here: OnEngineStopped can fire while the
                // service is still closing the TUN / stopSelf-ing, and the
                // UI would re-enable Start on a half-torn-down session.
                // runTeardown publishes DISCONNECTED when a new start is safe.
                if (_statusText.value == "STOPPING") {
                    _isRunning.value = false
                    return
                }
                updateStatus(false, "DISCONNECTED")
            }
        }
    }

    fun updateStatus(running: Boolean, text: String) {
        when {
            text.startsWith("FAILED:") -> failure = text.removePrefix("FAILED:")
            text == "STARTING" -> failure = null
        }
        // A terminal failure is never overwritten (e.g. by the DISCONNECTED
        // emitted during teardown) — only a new start (STARTING) clears it.
        if (failure != null) return
        _isRunning.value = running
        _statusText.value = text
    }
}
