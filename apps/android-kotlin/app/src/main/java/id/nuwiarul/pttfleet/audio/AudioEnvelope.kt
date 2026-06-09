package id.nuwiarul.pttfleet.audio

import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.util.UUID

object AudioEnvelope {
    const val HEADER_SIZE = 25
    const val UPLINK: Byte = 0x01
    const val DOWNLINK: Byte = 0x02

    fun encodeUplink(sessionId: String, sequence: Long, opusPayload: ByteArray): ByteArray {
        val uuid = UUID.fromString(sessionId)
        return ByteBuffer.allocate(HEADER_SIZE + opusPayload.size)
            .order(ByteOrder.BIG_ENDIAN)
            .put(UPLINK)
            .putLong(uuid.mostSignificantBits)
            .putLong(uuid.leastSignificantBits)
            .putLong(sequence)
            .put(opusPayload)
            .array()
    }

    fun decodeDownlink(data: ByteArray): DownlinkFrame? {
        if (data.size <= HEADER_SIZE || data[0] != DOWNLINK) return null
        val buffer = ByteBuffer.wrap(data).order(ByteOrder.BIG_ENDIAN)
        buffer.get()
        val sessionId = UUID(buffer.long, buffer.long).toString()
        val sequence = buffer.long
        val payload = ByteArray(buffer.remaining())
        buffer.get(payload)
        return DownlinkFrame(sessionId, sequence, payload)
    }
}

data class DownlinkFrame(
    val sessionId: String,
    val sequence: Long,
    val payload: ByteArray,
)
