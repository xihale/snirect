package com.xihale.snirect.ktlib

/**
 * Kotlin-side engine state — the single source of truth for the UI.
 *
 * The gomobile engine reports its lifecycle through the three callbacks
 * (`onEngineStarted` / `onEngineError` / `onEngineStopped`); [SnirectClient]
 * maps them onto this state and exposes it as a [kotlinx.coroutines.flow.StateFlow].
 *
 * Transitions:
 * - `Idle -> Starting -> Running` on a successful start;
 * - `Starting -> Failed` on startup failure, `Running -> Failed` on runtime death;
 * - `Running -> Stopped` on an explicit [SnirectClient.stopEngine].
 *
 * [Failed] is terminal: only the next user-initiated [SnirectClient.startEngine]
 * (which moves the state back to [Starting]) resets it.
 */
sealed interface EngineState {
    /** No engine has been started in this process yet. */
    data object Idle : EngineState

    /** `startEngine` has been issued; the core is still initializing. */
    data object Starting : EngineState

    /** The whole engine is ready (proxy listening, TUN bridged). */
    data object Running : EngineState

    /** Startup failed, or the engine died at runtime. Terminal until the next start. */
    data class Failed(val reason: String) : EngineState

    /** A running engine was stopped explicitly and shut down cleanly. */
    data class Stopped(val reason: String) : EngineState
}

data class SnirectEngineConfig(
    val nameservers: List<String> = emptyList(),
    val bootstrapDns: List<String> = emptyList(),
    val checkHostname: Boolean = false,
    val mtu: Int = 1500,
    val enableIpv6: Boolean = false,
    val logLevel: String = "info",
) {
    class Builder {
        private var nameservers: List<String> = emptyList()
        private var bootstrapDns: List<String> = emptyList()
        private var checkHostname: Boolean = false
        private var mtu: Int = 1500
        private var enableIpv6: Boolean = false
        private var logLevel: String = "info"

        fun nameservers(value: List<String>) = apply { nameservers = value }

        fun bootstrapDns(value: List<String>) = apply { bootstrapDns = value }

        fun checkHostname(value: Boolean) = apply { checkHostname = value }

        fun mtu(value: Int) = apply { mtu = value }

        fun enableIpv6(value: Boolean) = apply { enableIpv6 = value }

        fun logLevel(value: String) = apply { logLevel = value }

        fun build(): SnirectEngineConfig = SnirectEngineConfig(
            nameservers = nameservers,
            bootstrapDns = bootstrapDns,
            checkHostname = checkHostname,
            mtu = mtu,
            enableIpv6 = enableIpv6,
            logLevel = logLevel,
        )
    }
}
