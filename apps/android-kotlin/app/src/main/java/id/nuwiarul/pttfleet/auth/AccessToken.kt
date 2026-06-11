package id.nuwiarul.pttfleet.auth

import org.json.JSONObject
import java.util.Base64

object AccessToken {
    private const val EXPIRY_SKEW_SECONDS = 30L

    fun needsRefresh(
        token: String,
        nowEpochSeconds: Long = System.currentTimeMillis() / 1_000,
    ): Boolean {
        val expiresAt = expiresAt(token) ?: return true
        return expiresAt <= nowEpochSeconds + EXPIRY_SKEW_SECONDS
    }

    internal fun expiresAt(token: String): Long? = runCatching {
        val payload = token.split('.').getOrNull(1) ?: return null
        val decoded = Base64.getUrlDecoder().decode(payload)
        JSONObject(String(decoded, Charsets.UTF_8)).getLong("exp")
    }.getOrNull()
}
