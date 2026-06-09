package id.nuwiarul.pttfleet

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import android.view.View
import androidx.appcompat.app.AlertDialog
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import id.nuwiarul.pttfleet.auth.AuthRepository
import id.nuwiarul.pttfleet.auth.AuthSession
import id.nuwiarul.pttfleet.auth.SecureTokenStore
import id.nuwiarul.pttfleet.databinding.ActivityMainBinding
import id.nuwiarul.pttfleet.location.GpsSample
import id.nuwiarul.pttfleet.location.LocationTracker
import id.nuwiarul.pttfleet.websocket.ConnectionStatus
import id.nuwiarul.pttfleet.websocket.RealtimeClient
import id.nuwiarul.pttfleet.websocket.RealtimeListener
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import java.util.concurrent.TimeUnit

class MainActivity : AppCompatActivity(), RealtimeListener {
    private lateinit var binding: ActivityMainBinding
    private lateinit var tokenStore: SecureTokenStore
    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .build()
    private val authRepository = AuthRepository(httpClient)
    private var realtimeClient: RealtimeClient? = null
    private lateinit var locationTracker: LocationTracker
    private var latestLocation: GpsSample? = null
    private val locationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { permissions ->
        val granted = permissions[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
            permissions[Manifest.permission.ACCESS_COARSE_LOCATION] == true
        if (granted) {
            startTracking()
        } else {
            binding.locationStatusText.setText(R.string.location_permission_denied)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        tokenStore = SecureTokenStore(applicationContext)
        locationTracker = LocationTracker(
            applicationContext,
            ::handleLocation,
        ) { message -> binding.locationStatusText.text = message }
        binding.loginButton.setOnClickListener { login() }
        binding.logoutButton.setOnClickListener { logout() }
        binding.locationToggleButton.setOnClickListener { toggleTracking() }
        binding.sosButton.setOnClickListener { confirmSos() }

        tokenStore.load()?.let { showSession(it) } ?: showLogin()
    }

    override fun onDestroy() {
        locationTracker.stop()
        realtimeClient?.disconnect()
        realtimeClient = null
        super.onDestroy()
    }

    private fun login() {
        val serverUrl = binding.serverUrlInput.text.toString()
        val username = binding.usernameInput.text.toString().trim()
        val password = binding.passwordInput.text.toString()
        if (serverUrl.isBlank() || username.isBlank() || password.isBlank()) {
            showLoginError(getString(R.string.login_fields_required))
            return
        }

        setLoginLoading(true)
        binding.loginError.visibility = View.GONE
        lifecycleScope.launch {
            runCatching {
                authRepository.login(serverUrl, username, password)
            }.onSuccess { session ->
                tokenStore.save(session)
                binding.passwordInput.text?.clear()
                showSession(session)
            }.onFailure { error ->
                showLoginError(error.message ?: getString(R.string.login_failed))
            }
            setLoginLoading(false)
        }
    }

    private fun showSession(session: AuthSession) {
        binding.serverUrlInput.setText(session.serverUrl)
        binding.loginPanel.visibility = View.GONE
        binding.sessionPanel.visibility = View.VISIBLE
        binding.userNameText.text = session.user.fullName
        binding.userRoleText.text = getString(
            R.string.user_identity,
            session.user.username,
            session.user.role.replace('_', ' '),
        )
        binding.connectionDetailText.setText(R.string.realtime_opening)

        realtimeClient?.disconnect()
        realtimeClient = RealtimeClient(httpClient, this).also { it.connect(session) }
    }

    private fun showLogin() {
        stopTracking()
        binding.loginPanel.visibility = View.VISIBLE
        binding.sessionPanel.visibility = View.GONE
        binding.loginError.visibility = View.GONE
    }

    private fun logout() {
        stopTracking()
        realtimeClient?.disconnect()
        realtimeClient = null
        tokenStore.clear()
        binding.passwordInput.text?.clear()
        showLogin()
    }

    private fun toggleTracking() {
        if (locationTracker.isTracking()) {
            stopTracking()
            return
        }

        val hasFine = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.ACCESS_FINE_LOCATION,
        ) == PackageManager.PERMISSION_GRANTED
        val hasCoarse = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.ACCESS_COARSE_LOCATION,
        ) == PackageManager.PERMISSION_GRANTED
        if (hasFine || hasCoarse) {
            startTracking()
        } else {
            locationPermissionLauncher.launch(
                arrayOf(
                    Manifest.permission.ACCESS_FINE_LOCATION,
                    Manifest.permission.ACCESS_COARSE_LOCATION,
                ),
            )
        }
    }

    private fun startTracking() {
        locationTracker.start()
        binding.locationToggleButton.setText(R.string.stop_tracking)
        binding.locationStatusText.setText(R.string.location_waiting)
    }

    private fun stopTracking() {
        if (::locationTracker.isInitialized) {
            locationTracker.stop()
        }
        if (::binding.isInitialized) {
            binding.locationToggleButton.setText(R.string.start_tracking)
            binding.locationStatusText.setText(R.string.location_stopped)
        }
    }

    private fun handleLocation(sample: GpsSample) {
        latestLocation = sample
        val sent = realtimeClient?.sendGps(sample) == true
        binding.locationStatusText.text = getString(
            if (sent) R.string.location_sent else R.string.location_not_sent,
            sample.lat,
            sample.lng,
        )
    }

    private fun confirmSos() {
        AlertDialog.Builder(this)
            .setTitle(R.string.sos_confirm_title)
            .setMessage(R.string.sos_confirm_message)
            .setNegativeButton(R.string.cancel, null)
            .setPositiveButton(R.string.send_sos) { _, _ -> sendSos() }
            .show()
    }

    private fun sendSos() {
        val sent = realtimeClient?.sendSos(latestLocation) == true
        binding.sosStatusText.setText(if (sent) R.string.sos_sent else R.string.sos_not_sent)
    }

    private fun setLoginLoading(loading: Boolean) {
        binding.loginButton.isEnabled = !loading
        binding.loginButton.setText(if (loading) R.string.connecting else R.string.login)
    }

    private fun showLoginError(message: String) {
        binding.loginError.text = message
        binding.loginError.visibility = View.VISIBLE
    }

    override fun onStatusChanged(status: ConnectionStatus) {
        binding.connectionStatusText.setText(
            when (status) {
                ConnectionStatus.IDLE -> R.string.connection_idle
                ConnectionStatus.CONNECTING -> R.string.connection_connecting
                ConnectionStatus.CONNECTED -> R.string.connection_connected
                ConnectionStatus.RECONNECTING -> R.string.connection_reconnecting
            },
        )
    }

    override fun onReady(connectionId: String) {
        binding.connectionDetailText.text = getString(R.string.realtime_ready, connectionId.take(8))
    }

    override fun onError(message: String) {
        binding.connectionDetailText.text = message
    }
}
