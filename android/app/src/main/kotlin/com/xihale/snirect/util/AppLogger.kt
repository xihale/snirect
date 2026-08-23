package com.xihale.snirect.util

import android.content.Context
import android.util.Log
import com.xihale.snirect.MainActivity
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.Executors

object AppLogger {
    private const val TAG = "Snirect"
    private const val LOG_FILE_NAME = "snirect.log"
    private const val LOG_FILE_MAX_BYTES = 256 * 1024L

    // Privacy boundary: destinations stay visible wherever the log remains on
    // the device (in-app Logs screen, filesDir/snirect.log — the traffic itself
    // is already on the device in richer form) and are scrubbed only where the
    // text can leave: logcat (ROM diagnostics/bug reports sweep it) and the
    // share button. IP literals are scrubbed too — addr= and error= used to
    // leak the resolved destination while host= was covered, i.e. the answer
    // with the question hidden.
    private val hostPattern = Regex("(?i)(host|target|target_sni|sni|domain|remote_addr|addr|ip)=([^\\s,]+)")
    private val fdPattern = Regex("(?i)(fd)=\\d+")
    private val ipv4Pattern = Regex("\\b\\d{1,3}(?:\\.\\d{1,3}){3}\\b")
    // Bracketed IPv6 with at least two colon groups, so hex-word brackets like
    // [DEBUG]/[ERROR] (non-hex letters) and single-colon spans never match.
    private val ipv6Pattern = Regex("\\[(?:[0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}\\]")

    private const val LEVEL_DEBUG = 0
    private const val LEVEL_INFO = 1
    private const val LEVEL_WARN = 2
    private const val LEVEL_ERROR = 3

    /** Mirrors the Settings log level; debug logs are dropped by default. */
    @Volatile
    private var minLevel: Int = LEVEL_INFO

    /** Applies the user-configured level ("debug"/"info"/"warn"/"error"). */
    fun setMinLevel(level: String) {
        minLevel = when (level.lowercase()) {
            "debug" -> LEVEL_DEBUG
            "warn" -> LEVEL_WARN
            "error" -> LEVEL_ERROR
            else -> LEVEL_INFO
        }
    }

    @Volatile
    private var appContext: Context? = null

    /** Single background writer so persistence never runs on the calling thread. */
    private val logWriter = Executors.newSingleThreadExecutor { r ->
        Thread(r, "snirect-log-writer").apply { isDaemon = true }
    }

    // Only touched from the single logWriter thread.
    private val fileDateFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US)

    /**
     * Enables best-effort persistence to `<filesDir>/snirect.log` (ring file,
     * capped at ~256KB per generation with one rotated `.1` backup). Idempotent;
     * call from any process entry point (Activity/Service/Receiver/Tile).
     */
    fun init(context: Context) {
        if (appContext == null) {
            synchronized(this) {
                if (appContext == null) {
                    appContext = context.applicationContext
                }
            }
        }
    }

    /** Scrubs destinations for the channels where logs can leave the device
     *  (logcat, share). Public so the Logs screen share button can pass its
     *  in-memory buffer through the same funnel. */
    fun sanitize(message: String): String = message
        .replace(hostPattern) { "${it.groupValues[1]}=<redacted>" }
        .replace(fdPattern) { "${it.groupValues[1]}=<redacted>" }
        .replace(ipv4Pattern, "<ip>")
        .replace(ipv6Pattern, "<ip>")

    fun d(message: String) {
        if (minLevel > LEVEL_DEBUG) return
        Log.d(TAG, sanitize(message))
        MainActivity.log("[DEBUG] $message")
        persist("DEBUG", message)
    }

    fun i(message: String) {
        if (minLevel > LEVEL_INFO) return
        Log.i(TAG, sanitize(message))
        MainActivity.log("[INFO] $message")
        persist("INFO", message)
    }

    fun w(message: String) {
        if (minLevel > LEVEL_WARN) return
        Log.w(TAG, sanitize(message))
        MainActivity.log("[WARN] $message")
        persist("WARN", message)
    }

    fun e(message: String, throwable: Throwable? = null) {
        if (minLevel > LEVEL_ERROR) return
        Log.e(TAG, sanitize(message), throwable)
        MainActivity.log("[ERROR] $message")
        persist("ERROR", message, throwable)
    }

    private fun persist(level: String, message: String, throwable: Throwable? = null) {
        val context = appContext ?: return
        logWriter.execute {
            try {
                // Line building (and fileDateFormat) stays on this single
                // writer thread — SimpleDateFormat is not thread-safe.
                val line = buildString {
                    append(fileDateFormat.format(Date()))
                    append(" [").append(level).append("] ").append(message)
                    if (throwable != null) {
                        append('\n').append(Log.getStackTraceString(throwable).trimEnd())
                    }
                    append('\n')
                }
                val file = File(context.filesDir, LOG_FILE_NAME)
                if (file.length() > LOG_FILE_MAX_BYTES) {
                    // Rotate by suffix: keep one previous generation (~512KB total).
                    val previous = File(context.filesDir, "$LOG_FILE_NAME.1")
                    if (previous.exists()) previous.delete()
                    if (!file.renameTo(previous)) file.delete()
                }
                file.appendText(line)
            } catch (_: Exception) {
                // Logging must never take the app down.
            }
        }
    }
}
