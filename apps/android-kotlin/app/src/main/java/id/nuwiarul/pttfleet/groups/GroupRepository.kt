package id.nuwiarul.pttfleet.groups

import id.nuwiarul.pttfleet.auth.AuthSession
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONArray
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

    suspend fun members(session: AuthSession, groupId: String): List<GroupMember> =
        withContext(Dispatchers.IO) {
            val request = Request.Builder()
                .url("${session.serverUrl}/api/groups/$groupId")
                .header("Authorization", "Bearer ${session.accessToken}")
                .build()

            httpClient.newCall(request).execute().use { response ->
                val body = response.body?.string().orEmpty()
                if (!response.isSuccessful) {
                    throw IllegalStateException("Unable to load group members (HTTP ${response.code})")
                }
                val items = JSONObject(body).getJSONArray("members")
                buildList {
                    for (index in 0 until items.length()) {
                        val item = items.getJSONObject(index)
                        add(
                            GroupMember(
                                userId = item.getString("userId"),
                                username = item.getString("username"),
                                fullName = item.getString("fullName"),
                                role = item.getString("role"),
                                roleInGroup = item.getString("roleInGroup"),
                            ),
                        )
                    }
                }
            }
        }

    suspend fun latestLocations(session: AuthSession, groupId: String): List<GroupLocation> =
        withContext(Dispatchers.IO) {
            val request = Request.Builder()
                .url("${session.serverUrl}/api/groups/$groupId/locations")
                .header("Authorization", "Bearer ${session.accessToken}")
                .build()

            httpClient.newCall(request).execute().use { response ->
                val body = response.body?.string().orEmpty()
                if (!response.isSuccessful) {
                    throw IllegalStateException(
                        "Unable to load group locations (HTTP ${response.code})",
                    )
                }
                parseLocations(JSONObject(body).getJSONArray("items"))
            }
        }

    private fun parseLocations(items: JSONArray): List<GroupLocation> = buildList {
        for (index in 0 until items.length()) {
            val item = items.getJSONObject(index)
            add(
                GroupLocation(
                    userId = item.getString("userId"),
                    username = item.getString("username"),
                    fullName = item.getString("fullName"),
                    role = item.getString("role"),
                    lat = item.getDouble("lat"),
                    lng = item.getDouble("lng"),
                    speed = item.nullableDouble("speed"),
                    heading = item.nullableDouble("heading"),
                    accuracy = item.nullableDouble("accuracy"),
                    recordedAt = item.getString("recordedAt"),
                ),
            )
        }
    }

    private fun JSONObject.nullableDouble(name: String): Double? =
        if (has(name) && !isNull(name)) getDouble(name) else null
}
