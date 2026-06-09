package id.nuwiarul.pttfleet.websocket

import org.junit.Assert.assertEquals
import org.junit.Test

class BackoffPolicyTest {
    @Test
    fun growsExponentiallyAndCapsAtThirtySeconds() {
        assertEquals(1_000L, BackoffPolicy.delayMillis(0))
        assertEquals(2_000L, BackoffPolicy.delayMillis(1))
        assertEquals(8_000L, BackoffPolicy.delayMillis(3))
        assertEquals(30_000L, BackoffPolicy.delayMillis(10))
    }
}
