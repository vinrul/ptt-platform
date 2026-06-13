package id.nuwiarul.pttfleet.websocket

object BackoffPolicy {
    private const val FAST_RECONNECT_MAX_DELAY_MILLIS = 30_000L
    private const val WARM_RECONNECT_DELAY_MILLIS = 60_000L
    private const val SLOW_RECONNECT_DELAY_MILLIS = 120_000L
    private const val MAX_DELAY_MILLIS = 300_000L

    fun delayMillis(attempt: Int): Long {
        if (attempt <= 0) return 1_000L
        if (attempt > 20) return MAX_DELAY_MILLIS
        if (attempt > 10) return SLOW_RECONNECT_DELAY_MILLIS
        if (attempt > 5) return WARM_RECONNECT_DELAY_MILLIS

        val multiplier = 1L shl attempt.coerceAtMost(5)
        return (1_000L * multiplier).coerceAtMost(FAST_RECONNECT_MAX_DELAY_MILLIS)
    }
}
