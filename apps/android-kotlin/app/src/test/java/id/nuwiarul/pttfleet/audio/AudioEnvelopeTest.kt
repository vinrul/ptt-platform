package id.nuwiarul.pttfleet.audio

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test

class AudioEnvelopeTest {
    @Test
    fun roundTripsBinaryEnvelope() {
        val sessionId = "11111111-1111-4111-8111-111111111111"
        val uplink = AudioEnvelope.encodeUplink(sessionId, 42, byteArrayOf(1, 2, 3))
        uplink[0] = AudioEnvelope.DOWNLINK

        val decoded = requireNotNull(AudioEnvelope.decodeDownlink(uplink))
        assertEquals(sessionId, decoded.sessionId)
        assertEquals(42, decoded.sequence)
        assertArrayEquals(byteArrayOf(1, 2, 3), decoded.payload)
    }
}
