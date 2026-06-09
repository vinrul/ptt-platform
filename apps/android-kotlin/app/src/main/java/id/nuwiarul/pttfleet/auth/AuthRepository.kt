package id.nuwiarul.pttfleet.auth

import android.os.Build
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

class AuthRepository(
    private val httpClient: OkHttpClient,
) {
    suspend fun login(
        serverUrl: String,
        username: String,
        password: String,
    ): AuthSession = withContext(Dispatchers.IO) {
        val normalizedServerUrl = EndpointNormalizer.serverUrl(serverUrl)
        val requestBody = JSONObject()
            .put("username", username.trim())
            .put("password", password)
            .put("deviceName", "Android ${Build.MANUFACTURER} ${Build.MODEL}".trim())
            .put("clientType", "android")
            .toString()
            .toRequestBody(JSON_MEDIA_TYPE)

        val request = Request.Builder()
            .url(EndpointNormalizer.loginUrl(normalizedServerUrl))
            .post(requestBody)
            .build()

        httpClient.newCall(request).execute().use { response ->
            val body = response.body?.string().orEmpty()
            if (!response.isSuccessful) {
                throw AuthException(errorMessage(body, response.code))
            }

            parseSession(normalizedServerUrl, body)
        }
    }

    private fun parseSession(serverUrl: String, body: String): AuthSession {
        val root = JSONObject(body)
        val user = root.getJSONObject("user")
        return AuthSession(
            serverUrl = serverUrl,
            accessToken = root.getString("accessToken"),
            refreshToken = root.getString("refreshToken"),
            user = AuthUser(
                id = user.getString("id"),
                username = user.getString("username"),
                fullName = user.getString("fullName"),
                role = user.getString("role"),
                status = user.optString("status", "active"),
            ),
        )
    }

    private fun errorMessage(body: String, statusCode: Int): String {
        return runCatching {
            JSONObject(body).getJSONObject("error").getString("message")
        }.getOrElse {
            "Login failed (HTTP $statusCode)"
        }
    }

    companion object {
        private val JSON_MEDIA_TYPE = "application/json; charset=utf-8".toMediaType()
    }
}

class AuthException(message: String) : Exception(message)
