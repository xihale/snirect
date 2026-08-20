package com.xihale.snirect.ktlib

/**
 * Host callbacks for the Snirect engine.
 *
 * Implementations only need to override what they use: the three engine
 * lifecycle callbacks have no-op defaults, and their information is also
 * reflected in [SnirectClient.engineState] — most hosts should watch the
 * state flow instead of overriding them.
 *
 * All methods may be invoked from arbitrary Go runtime threads.
 */
interface SnirectEngineCallbacks {
    /** A log line from the engine. */
    fun onStatusChanged(status: String?)

    /**
     * Asks the host to route an outbound socket outside the VPN
     * (Android: `VpnService.protect(fd)`).
     */
    fun protect(fd: Long): Boolean

    /** Fired once when the whole engine is ready. Default: no-op. */
    fun onEngineStarted() {}

    /** Fired once on startup failure or terminal runtime engine death. Default: no-op. */
    fun onEngineError(reason: String) {}

    /** Fired once after an explicit stop of a running engine. Default: no-op. */
    fun onEngineStopped(reason: String) {}
}
