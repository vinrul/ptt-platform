package id.nuwiarul.pttfleet.auth

import android.content.Context
import androidx.core.content.edit

class ServerSettingsStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun load(defaultUrl: String): String =
        preferences.getString(KEY_SERVER_URL, null)?.takeIf { it.isNotBlank() } ?: defaultUrl

    fun save(serverUrl: String) {
        preferences.edit {
            putString(KEY_SERVER_URL, EndpointNormalizer.serverUrl(serverUrl))
        }
    }

    companion object {
        private const val PREFERENCES_NAME = "server_settings"
        private const val KEY_SERVER_URL = "server_url"
    }
}
