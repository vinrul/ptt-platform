package id.nuwiarul.pttfleet.location

import android.content.Context
import androidx.core.content.edit

class TrackingPreferenceStore(context: Context) {
    private val preferences =
        context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun isEnabled(): Boolean = preferences.getBoolean(KEY_ENABLED, false)

    fun setEnabled(enabled: Boolean) {
        preferences.edit {
            putBoolean(KEY_ENABLED, enabled)
        }
    }

    companion object {
        private const val PREFERENCES_NAME = "location_tracking"
        private const val KEY_ENABLED = "enabled"
    }
}
