package id.nuwiarul.pttfleet

import android.content.Context
import androidx.core.content.edit

class FirstRunWizardStore(context: Context) {
    private val preferences =
        context.applicationContext.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun isComplete(): Boolean =
        preferences.getBoolean(KEY_COMPLETE, false)

    fun markComplete() {
        preferences.edit { putBoolean(KEY_COMPLETE, true) }
    }

    private companion object {
        const val PREFERENCES_NAME = "first_run_wizard"
        const val KEY_COMPLETE = "complete"
    }
}
