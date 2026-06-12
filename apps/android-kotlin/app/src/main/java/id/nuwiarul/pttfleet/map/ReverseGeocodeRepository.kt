package id.nuwiarul.pttfleet.map

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import id.nuwiarul.pttfleet.auth.AuthSession
import id.nuwiarul.pttfleet.auth.EndpointNormalizer
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject
import java.util.Locale

class ReverseGeocodeRepository(
    private val httpClient: OkHttpClient,
) {
    private val cache = mutableMapOf<String, String>()

    suspend fun reverse(session: AuthSession, lat: Double, lng: Double): String = withContext(Dispatchers.IO) {
        val key = cacheKey(lat, lng)
        cache[key]?.let { return@withContext it }

        val url = "${EndpointNormalizer.serverUrl(session.serverUrl)}/api/geocode/reverse"
            .toHttpUrl()
            .newBuilder()
            .addQueryParameter("lat", lat.toString())
            .addQueryParameter("lng", lng.toString())
            .build()
        val request = Request.Builder()
            .url(url)
            .header("Authorization", "Bearer ${session.accessToken}")
            .header("Accept", "application/json")
            .build()

        httpClient.newCall(request).execute().use { response ->
            val body = response.body?.string().orEmpty()
            if (!response.isSuccessful) {
                throw IllegalStateException("Unable to reverse geocode (HTTP ${response.code})")
            }
            val address = JSONObject(body)
                .optString("displayName")
                .takeIf { it.isNotBlank() }
                ?: throw IllegalStateException("Address not found")
            cache[key] = address
            address
        }
    }

    private fun cacheKey(lat: Double, lng: Double): String =
        String.format(Locale.US, "%.5f,%.5f", lat, lng)
}
