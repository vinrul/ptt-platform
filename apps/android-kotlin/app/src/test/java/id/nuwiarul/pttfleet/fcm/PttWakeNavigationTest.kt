package id.nuwiarul.pttfleet.fcm

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PttWakeNavigationTest {
    @Test
    fun `direct wakeup keeps speaker identity`() {
        val navigation = PttWakeNavigation.fromData(
            mapOf(
                "groupId" to "group-1",
                "mode" to "direct",
                "speakerUserId" to "user-1",
                "speakerUsername" to "field1",
            ),
        )!!

        assertTrue(navigation.isDirect)
        assertEquals("user-1", navigation.speakerUserId)
        assertEquals("field1", navigation.speakerUsername)
    }

    @Test
    fun `legacy or broadcast wakeup opens home`() {
        val navigation = PttWakeNavigation.fromData(mapOf("groupId" to "group-1"))!!

        assertFalse(navigation.isDirect)
        assertEquals(PttWakeNavigation.MODE_BROADCAST, navigation.mode)
    }
}
