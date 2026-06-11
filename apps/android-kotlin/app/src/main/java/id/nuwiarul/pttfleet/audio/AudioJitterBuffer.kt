package id.nuwiarul.pttfleet.audio

import java.util.TreeMap

data class BufferedAudioFrame(
    val sessionId: String,
    val sequence: Long,
    val payload: ByteArray,
)

class AudioJitterBuffer(
    private val prebufferFrames: Int = 3,
    private val gapToleranceFrames: Int = 3,
    private val maxBufferedFrames: Int = 8,
) {
    private val frames = TreeMap<Long, BufferedAudioFrame>()
    private var sessionId: String? = null
    private var expectedSequence: Long? = null
    private var started = false

    @Synchronized
    fun add(frame: BufferedAudioFrame): List<BufferedAudioFrame> {
        if (sessionId != frame.sessionId) {
            clearLocked()
            sessionId = frame.sessionId
        }

        val expected = expectedSequence
        if (expected != null && frame.sequence < expected) return emptyList()
        frames.putIfAbsent(frame.sequence, frame)

        if (!started) {
            if (frames.size < prebufferFrames) return emptyList()
            expectedSequence = frames.firstKey()
            started = true
        }

        if (expectedSequence !in frames && frames.size >= gapToleranceFrames) {
            expectedSequence = frames.firstKey()
        }

        while (frames.size > maxBufferedFrames) {
            frames.pollFirstEntry()
            expectedSequence = frames.firstKey()
        }

        return buildList {
            var nextSequence = expectedSequence ?: return@buildList
            while (true) {
                val next = frames.remove(nextSequence) ?: break
                add(next)
                nextSequence++
            }
            expectedSequence = nextSequence
        }
    }

    @Synchronized
    fun clear() {
        clearLocked()
    }

    private fun clearLocked() {
        frames.clear()
        sessionId = null
        expectedSequence = null
        started = false
    }
}
