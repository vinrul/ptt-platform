package id.nuwiarul.pttfleet.auth

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Base64

class AccessTokenTest {
    @Test
    fun `refreshes token inside expiry skew`() {
        val token = tokenWithExpiry(1_025)

        assertTrue(AccessToken.needsRefresh(token, nowEpochSeconds = 1_000))
    }

    @Test
    fun `keeps token with sufficient validity`() {
        val token = tokenWithExpiry(1_031)

        assertFalse(AccessToken.needsRefresh(token, nowEpochSeconds = 1_000))
    }

    @Test
    fun `refreshes malformed token`() {
        assertTrue(AccessToken.needsRefresh("not-a-jwt", nowEpochSeconds = 1_000))
    }

    private fun tokenWithExpiry(expiresAt: Long): String {
        val encoder = Base64.getUrlEncoder().withoutPadding()
        val header = encoder.encodeToString("""{"alg":"none"}""".toByteArray())
        val payload = encoder.encodeToString("""{"exp":$expiresAt}""".toByteArray())
        return "$header.$payload.signature"
    }
}
