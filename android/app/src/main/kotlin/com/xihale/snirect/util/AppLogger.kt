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

    private val hostPattern = Regex("(?i)(host|target|target_sni|sni|domain|remote_addr|ip)=([^\\s,]+)")
    private val fdPattern = Regex("(?i)(fd)=\\d+")

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

    private fun sanitize(message: String): String = message
        .replace(hostPattern) { "${it.groupValues[1]}=<redacted>" }
        .replace(fdPattern) { "${it.groupValues[1]}=<redacted>" }

    fun d(message: String) {
        if (minLevel > LEVEL_DEBUG) return
        val clean = sanitize(message)
        Log.d(TAG, clean)
        MainActivity.log("[DEBUG] $clean")
        persist("DEBUG", clean)
    }

    fun i(message: String) {
        if (minLevel > LEVEL_INFO) return
        val clean = sanitize(message)
        Log.i(TAG, clean)
        MainActivity.log("[INFO] $clean")
        persist("INFO", clean)
    }

    fun w(message: String) {
        if (minLevel > LEVEL_WARN) return
        val clean = sanitize(message)
        Log.w(TAG, clean)
        MainActivity.log("[WARN] $clean")
        persist("WARN", clean)
    }

    fun e(message: String, throwable: Throwable? = null) {
        if (minLevel > LEVEL_ERROR) return
        val clean = sanitize(message)
        Log.e(TAG, clean, throwable)
        MainActivity.log("[ERROR] $clean")
        persist("ERROR", clean, throwable)
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
