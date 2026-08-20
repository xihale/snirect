package com.xihale.snirect.data.model

import java.util.concurrent.atomic.AtomicLong

enum class LogLevel {
    DEBUG, INFO, WARN, ERROR;
}

data class LogEntry(
    val level: LogLevel,
    val message: String,
    val timestamp: Long = System.currentTimeMillis(),
    // Monotonic id: stable LazyColumn key even when timestamps (ms) collide.
    val id: Long = nextId.getAndIncrement()
) {
    private companion object {
        private val nextId = AtomicLong(0L)
    }
}
