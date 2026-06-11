package id.nuwiarul.pttfleet.location

import org.junit.Assert.assertEquals
import org.junit.Test

class AdaptiveLocationPolicyTest {
    @Test
    fun `starts in moving mode`() {
        val policy = AdaptiveLocationPolicy()

        assertEquals(LocationTrackingMode.MOVING, policy.mode)
    }

    @Test
    fun `switches to stationary after consecutive slow samples`() {
        val policy = AdaptiveLocationPolicy(stationarySamplesRequired = 3)

        assertEquals(LocationTrackingMode.MOVING, policy.update(0.2))
        assertEquals(LocationTrackingMode.MOVING, policy.update(0.1))
        assertEquals(LocationTrackingMode.STATIONARY, policy.update(0.0))
    }

    @Test
    fun `medium speed cancels pending stationary transition`() {
        val policy = AdaptiveLocationPolicy(stationarySamplesRequired = 2)

        policy.update(0.1)
        policy.update(0.7)

        assertEquals(LocationTrackingMode.MOVING, policy.update(0.1))
    }

    @Test
    fun `movement immediately returns to moving mode`() {
        val policy = AdaptiveLocationPolicy(stationarySamplesRequired = 1)
        policy.update(0.0)

        assertEquals(LocationTrackingMode.MOVING, policy.update(1.2))
    }

    @Test
    fun `unknown speed does not force a mode change`() {
        val policy = AdaptiveLocationPolicy(stationarySamplesRequired = 1)

        assertEquals(LocationTrackingMode.MOVING, policy.update(null))
        policy.update(0.0)
        assertEquals(LocationTrackingMode.STATIONARY, policy.update(null))
    }

    @Test
    fun `unknown speed cancels an unfinished stationary transition`() {
        val policy = AdaptiveLocationPolicy(stationarySamplesRequired = 2)

        policy.update(0.0)
        policy.update(null)

        assertEquals(LocationTrackingMode.MOVING, policy.update(0.0))
    }

    @Test
    fun `reset restores moving mode`() {
        val policy = AdaptiveLocationPolicy(stationarySamplesRequired = 1)
        policy.update(0.0)

        policy.reset()

        assertEquals(LocationTrackingMode.MOVING, policy.mode)
    }
}
