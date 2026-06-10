package id.nuwiarul.pttfleet.audio

import kotlin.math.roundToInt

object PcmGain {
    fun apply(samples: ShortArray, length: Int, gain: Float) {
        if (gain == 1f) return

        for (index in 0 until length.coerceAtMost(samples.size)) {
            samples[index] = (samples[index] * gain)
                .roundToInt()
                .coerceIn(Short.MIN_VALUE.toInt(), Short.MAX_VALUE.toInt())
                .toShort()
        }
    }
}
