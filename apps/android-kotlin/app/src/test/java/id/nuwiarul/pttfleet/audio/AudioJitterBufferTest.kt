package id.nuwiarul.pttfleet.audio

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class AudioJitterBufferTest {
    private val buffer = AudioJitterBuffer(
        prebufferFrames = 3,
        gapToleranceFrames = 3,
        maxBufferedFrames = 8,
    )

    @Test
    fun `prebuffers and sorts initial frames`() {
        assertTrue(buffer.add(frame(2)).isEmpty())
        assertTrue(buffer.add(frame(0)).isEmpty())

        assertEquals(listOf(0L, 1L, 2L), buffer.add(frame(1)).map { it.sequence })
    }

    @Test
    fun `drops duplicate and stale frames`() {
        buffer.add(frame(0))
        buffer.add(frame(1))
        buffer.add(frame(2))

        assertTrue(buffer.add(frame(2)).isEmpty())
        assertEquals(listOf(3L), buffer.add(frame(3)).map { it.sequence })
    }

    @Test
    fun `skips a missing frame after tolerance is reached`() {
        buffer.add(frame(0))
        buffer.add(frame(1))
        buffer.add(frame(2))

        assertTrue(buffer.add(frame(4)).isEmpty())
        assertTrue(buffer.add(frame(5)).isEmpty())
        assertEquals(listOf(4L, 5L, 6L), buffer.add(frame(6)).map { it.sequence })
    }

    @Test
    fun `new session clears frames from previous session`() {
        buffer.add(frame(0))
        buffer.add(frame(1))

        assertTrue(buffer.add(frame(0, sessionId = "new-session")).isEmpty())
        assertTrue(buffer.add(frame(1, sessionId = "new-session")).isEmpty())
        assertEquals(
            listOf(0L, 1L, 2L),
            buffer.add(frame(2, sessionId = "new-session")).map { it.sequence },
        )
    }

    private fun frame(sequence: Long, sessionId: String = "session") = BufferedAudioFrame(
        sessionId = sessionId,
        sequence = sequence,
        payload = byteArrayOf(sequence.toByte()),
    )
}
