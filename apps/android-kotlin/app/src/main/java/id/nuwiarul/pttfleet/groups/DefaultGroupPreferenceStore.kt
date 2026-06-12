package id.nuwiarul.pttfleet.groups

import android.content.Context

class DefaultGroupPreferenceStore(context: Context) {
    private val preferences = context.applicationContext.getSharedPreferences(
        PREFERENCES_NAME,
        Context.MODE_PRIVATE,
    )

    fun getDefaultGroupId(): String? =
        preferences.getString(KEY_DEFAULT_GROUP_ID, null)?.takeIf { it.isNotBlank() }

    fun setDefaultGroupId(groupId: String) {
        preferences.edit().putString(KEY_DEFAULT_GROUP_ID, groupId).apply()
    }

    fun clear() {
        preferences.edit().remove(KEY_DEFAULT_GROUP_ID).apply()
    }

    companion object {
        private const val PREFERENCES_NAME = "default_group_preferences"
        private const val KEY_DEFAULT_GROUP_ID = "default_group_id"
    }
}
