package id.nuwiarul.pttfleet.groups

import id.nuwiarul.pttfleet.auth.AuthSession
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject

class GroupRepository(
    private val httpClient: OkHttpClient,
) {
    suspend fun list(session: AuthSession): List<GroupSummary> = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url("${session.serverUrl}/api/groups")
            .header("Authorization", "Bearer ${session.accessToken}")
            .build()

        httpClient.newCall(request).execute().use { response ->
            val body = response.body?.string().orEmpty()
            if (!response.isSuccessful) {
                throw IllegalStateException("Unable to load groups (HTTP ${response.code})")
            }
            val items = JSONObject(body).getJSONArray("items")
            buildList {
                for (index in 0 until items.length()) {
                    val item = items.getJSONObject(index)
                    add(
                        GroupSummary(
                            id = item.getString("id"),
                            name = item.getString("name"),
                            description = item.optString("description"),
                        ),
                    )
                }
            }
        }
    }
}
