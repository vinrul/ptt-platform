package id.nuwiarul.pttfleet

import android.os.Bundle
import android.view.View
import androidx.appcompat.app.AppCompatActivity
import id.nuwiarul.pttfleet.auth.ServerSettingsStore
import id.nuwiarul.pttfleet.databinding.ActivitySettingsBinding

class SettingsActivity : AppCompatActivity() {
    private lateinit var binding: ActivitySettingsBinding
    private lateinit var settingsStore: ServerSettingsStore

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivitySettingsBinding.inflate(layoutInflater)
        setContentView(binding.root)

        settingsStore = ServerSettingsStore(applicationContext)
        binding.serverUrlInput.setText(
            settingsStore.load(getString(R.string.default_server_url)),
        )
        binding.saveSettingsButton.setOnClickListener { saveSettings() }
    }

    private fun saveSettings() {
        binding.settingsError.visibility = View.GONE
        runCatching {
            settingsStore.save(binding.serverUrlInput.text.toString())
        }.onSuccess {
            setResult(RESULT_OK)
            finish()
        }.onFailure { error ->
            binding.settingsError.text = error.message ?: getString(R.string.settings_save_failed)
            binding.settingsError.visibility = View.VISIBLE
        }
    }
}
