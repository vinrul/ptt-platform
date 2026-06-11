package id.nuwiarul.pttfleet.fcm

import android.content.Context
import androidx.core.content.edit

data class PttWakeNavigation(
    val groupId: String,
    val mode: String,
    val speakerUserId: String?,
    val speakerUsername: String?,
    val receivedAtMillis: Long = System.currentTimeMillis(),
) {
    val isDirect: Boolean
        get() = mode == MODE_DIRECT

    companion object {
        const val MODE_BROADCAST = "broadcast"
        const val MODE_DIRECT = "direct"

        fun fromData(data: Map<String, String>): PttWakeNavigation? {
            val groupId = data["groupId"]?.takeIf { it.isNotBlank() } ?: return null
            val mode = data["mode"].takeIf { it == MODE_DIRECT } ?: MODE_BROADCAST
            return PttWakeNavigation(
                groupId = groupId,
                mode = mode,
                speakerUserId = data["speakerUserId"]?.takeIf { it.isNotBlank() },
                speakerUsername = data["speakerUsername"]?.takeIf { it.isNotBlank() },
            )
        }
    }
}

class PttWakeNavigationStore(context: Context) {
    private val preferences =
        context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun save(navigation: PttWakeNavigation) {
        preferences.edit {
            putString(KEY_GROUP_ID, navigation.groupId)
            putString(KEY_MODE, navigation.mode)
            putString(KEY_SPEAKER_USER_ID, navigation.speakerUserId)
            putString(KEY_SPEAKER_USERNAME, navigation.speakerUsername)
            putLong(KEY_RECEIVED_AT, navigation.receivedAtMillis)
        }
    }

    fun consume(nowMillis: Long = System.currentTimeMillis()): PttWakeNavigation? {
        val groupId = preferences.getString(KEY_GROUP_ID, null)
        val mode = preferences.getString(KEY_MODE, null)
        val receivedAt = preferences.getLong(KEY_RECEIVED_AT, 0)
        val navigation = if (
            !groupId.isNullOrBlank() &&
            !mode.isNullOrBlank() &&
            nowMillis - receivedAt in 0..MAX_AGE_MILLIS
        ) {
            PttWakeNavigation(
                groupId = groupId,
                mode = mode,
                speakerUserId = preferences.getString(KEY_SPEAKER_USER_ID, null),
                speakerUsername = preferences.getString(KEY_SPEAKER_USERNAME, null),
                receivedAtMillis = receivedAt,
            )
        } else {
            null
        }
        preferences.edit { clear() }
        return navigation
    }

    private companion object {
        const val PREFERENCES_NAME = "ptt_wake_navigation"
        const val KEY_GROUP_ID = "group_id"
        const val KEY_MODE = "mode"
        const val KEY_SPEAKER_USER_ID = "speaker_user_id"
        const val KEY_SPEAKER_USERNAME = "speaker_username"
        const val KEY_RECEIVED_AT = "received_at"
        const val MAX_AGE_MILLIS = 5 * 60 * 1_000L
    }
}
