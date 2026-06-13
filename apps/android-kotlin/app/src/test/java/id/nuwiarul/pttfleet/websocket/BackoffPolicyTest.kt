package id.nuwiarul.pttfleet.websocket

import org.junit.Assert.assertEquals
import org.junit.Test

class BackoffPolicyTest {
    @Test
    fun growsExponentiallyDuringFastReconnectWindow() {
        assertEquals(1_000L, BackoffPolicy.delayMillis(0))
        assertEquals(2_000L, BackoffPolicy.delayMillis(1))
        assertEquals(8_000L, BackoffPolicy.delayMillis(3))
        assertEquals(30_000L, BackoffPolicy.delayMillis(5))
    }

    @Test
    fun slowsDownAfterRepeatedFailures() {
        assertEquals(60_000L, BackoffPolicy.delayMillis(6))
        assertEquals(60_000L, BackoffPolicy.delayMillis(10))
        assertEquals(120_000L, BackoffPolicy.delayMillis(11))
        assertEquals(120_000L, BackoffPolicy.delayMillis(20))
        assertEquals(300_000L, BackoffPolicy.delayMillis(21))
    }
}
