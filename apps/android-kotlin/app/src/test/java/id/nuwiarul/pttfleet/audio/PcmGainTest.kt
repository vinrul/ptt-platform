package id.nuwiarul.pttfleet.audio

import org.junit.Assert.assertArrayEquals
import org.junit.Test

class PcmGainTest {
    @Test
    fun amplifiesAndClipsPcmSamples() {
        val samples = shortArrayOf(1_000, -1_000, 20_000, -20_000)

        PcmGain.apply(samples, samples.size, 2f)

        assertArrayEquals(
            shortArrayOf(2_000, -2_000, Short.MAX_VALUE, Short.MIN_VALUE),
            samples,
        )
    }

    @Test
    fun onlyChangesDecodedSampleRange() {
        val samples = shortArrayOf(1_000, 2_000, 3_000)

        PcmGain.apply(samples, 2, 1.5f)

        assertArrayEquals(shortArrayOf(1_500, 3_000, 3_000), samples)
    }
}

