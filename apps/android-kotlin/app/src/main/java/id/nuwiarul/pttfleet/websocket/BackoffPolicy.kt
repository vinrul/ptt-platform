package id.nuwiarul.pttfleet.websocket

object BackoffPolicy {
    private const val MAX_DELAY_MILLIS = 30_000L

    fun delayMillis(attempt: Int): Long {
        if (attempt <= 0) return 1_000L
        val multiplier = 1L shl attempt.coerceAtMost(5)
        return (1_000L * multiplier).coerceAtMost(MAX_DELAY_MILLIS)
    }
}
