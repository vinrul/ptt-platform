package id.nuwiarul.pttfleet.fcm

import android.content.Context
import android.provider.Settings
import androidx.core.content.edit

class WakeupOverlayPreferenceStore(context: Context) {
    private val preferencesContext = context.applicationContext
    private val preferences =
        preferencesContext.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun isEnabled(): Boolean =
        preferences.getBoolean(KEY_ENABLED, true)

    fun setEnabled(enabled: Boolean) {
        preferences.edit { putBoolean(KEY_ENABLED, enabled) }
    }

    fun canOpenAppOnWakeup(): Boolean =
        isEnabled() && Settings.canDrawOverlays(preferencesContext)

    private companion object {
        const val PREFERENCES_NAME = "wakeup_overlay_preferences"
        const val KEY_ENABLED = "enabled"
    }
}
