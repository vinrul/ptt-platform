package id.nuwiarul.pttfleet

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import android.view.MotionEvent
import android.view.View
import android.widget.ArrayAdapter
import androidx.appcompat.app.AlertDialog
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import id.nuwiarul.pttfleet.auth.AuthRepository
import id.nuwiarul.pttfleet.auth.AuthSession
import id.nuwiarul.pttfleet.auth.SecureTokenStore
import id.nuwiarul.pttfleet.audio.PttAudioEngine
import id.nuwiarul.pttfleet.databinding.ActivityMainBinding
import id.nuwiarul.pttfleet.groups.GroupRepository
import id.nuwiarul.pttfleet.groups.GroupSummary
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
    private val groupRepository = GroupRepository(httpClient)
    private var realtimeClient: RealtimeClient? = null
    private lateinit var audioEngine: PttAudioEngine
    private lateinit var locationTracker: LocationTracker
    private var latestLocation: GpsSample? = null
    private var groups: List<GroupSummary> = emptyList()
    private var joinedGroupId: String? = null
    private var activePttSessionId: String? = null
    private var audioSequence = 0L
    private var pttHeld = false
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
    private val audioPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        binding.pttStatusText.setText(
            if (granted) R.string.ptt_hold_again else R.string.audio_permission_denied,
        )
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
        audioEngine = PttAudioEngine(
            onEncodedFrame = { payload ->
                activePttSessionId?.let { sessionId ->
                    realtimeClient?.sendAudio(sessionId, audioSequence++, payload)
                }
            },
            onError = { message -> runOnUiThread { binding.pttStatusText.text = message } },
        )
        binding.loginButton.setOnClickListener { login() }
        binding.logoutButton.setOnClickListener { logout() }
        binding.locationToggleButton.setOnClickListener { toggleTracking() }
        binding.sosButton.setOnClickListener { confirmSos() }
        binding.groupSpinner.onItemSelectedListener = GroupSelectionListener { position ->
            groups.getOrNull(position)?.let { group ->
                joinedGroupId = null
                binding.pttButton.isEnabled = false
                realtimeClient?.joinGroup(group.id)
            }
        }
        binding.pttButton.setOnTouchListener { _, motionEvent ->
            when (motionEvent.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    pttHeld = true
                    requestPttStart()
                    true
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    pttHeld = false
                    stopPtt()
                    true
                }
                else -> false
            }
        }

        tokenStore.load()?.let { showSession(it) } ?: showLogin()
    }

    override fun onDestroy() {
        audioEngine.release()
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
        loadGroups(session)
    }

    private fun showLogin() {
        stopTracking()
        binding.loginPanel.visibility = View.VISIBLE
        binding.sessionPanel.visibility = View.GONE
        binding.loginError.visibility = View.GONE
        groups = emptyList()
        joinedGroupId = null
        binding.pttButton.isEnabled = false
    }

    private fun logout() {
        stopPtt()
        stopTracking()
        realtimeClient?.disconnect()
        realtimeClient = null
        tokenStore.clear()
        binding.passwordInput.text?.clear()
        showLogin()
    }

    private fun loadGroups(session: AuthSession) {
        lifecycleScope.launch {
            runCatching { groupRepository.list(session) }
                .onSuccess { items ->
                    groups = items
                    binding.groupSpinner.adapter = ArrayAdapter(
                        this@MainActivity,
                        android.R.layout.simple_spinner_dropdown_item,
                        items.map { it.name },
                    )
                    binding.pttStatusText.setText(
                        if (items.isEmpty()) R.string.no_groups else R.string.channel_joining,
                    )
                }
                .onFailure { binding.pttStatusText.text = it.message }
        }
    }

    private fun requestPttStart() {
        val groupId = joinedGroupId
        if (groupId == null) {
            binding.pttStatusText.setText(R.string.channel_not_ready)
            return
        }
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.RECORD_AUDIO) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            pttHeld = false
            audioPermissionLauncher.launch(Manifest.permission.RECORD_AUDIO)
            return
        }
        binding.pttStatusText.setText(R.string.ptt_requesting)
        realtimeClient?.startPtt(groupId)
    }

    private fun stopPtt() {
        audioEngine.stopCapture()
        activePttSessionId?.let { realtimeClient?.stopPtt(it) }
        activePttSessionId = null
        audioSequence = 0
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
        if (status != ConnectionStatus.CONNECTED) {
            stopPtt()
            joinedGroupId = null
            binding.pttButton.isEnabled = false
        }
    }

    override fun onReady(connectionId: String) {
        binding.connectionDetailText.text = getString(R.string.realtime_ready, connectionId.take(8))
        groups.firstOrNull()?.let { realtimeClient?.joinGroup(it.id) }
    }

    override fun onError(message: String) {
        binding.connectionDetailText.text = message
    }

    override fun onGroupJoined(groupId: String) {
        joinedGroupId = groupId
        binding.pttButton.isEnabled = true
        binding.pttStatusText.setText(R.string.ptt_ready)
    }

    override fun onPttGranted(sessionId: String, groupId: String) {
        if (!pttHeld || joinedGroupId != groupId) {
            realtimeClient?.stopPtt(sessionId)
            return
        }
        activePttSessionId = sessionId
        audioSequence = 0
        binding.pttStatusText.setText(R.string.ptt_transmitting)
        audioEngine.startCapture()
    }

    override fun onPttBusy(groupId: String, speakerUserId: String) {
        activePttSessionId = null
        binding.pttStatusText.setText(R.string.ptt_busy)
    }

    override fun onPttStarted(sessionId: String, groupId: String, speakerUserId: String) {
        if (sessionId != activePttSessionId) {
            binding.pttStatusText.setText(R.string.ptt_receiving)
        }
    }

    override fun onPttStopped(sessionId: String, groupId: String, reason: String) {
        if (sessionId == activePttSessionId) {
            audioEngine.stopCapture()
            activePttSessionId = null
        }
        binding.pttStatusText.setText(R.string.ptt_ready)
    }

    override fun onAudioFrame(sessionId: String, sequence: Long, payload: ByteArray) {
        if (sessionId != activePttSessionId) {
            audioEngine.play(payload)
        }
    }
}

private class GroupSelectionListener(
    private val onSelected: (Int) -> Unit,
) : android.widget.AdapterView.OnItemSelectedListener {
    override fun onItemSelected(
        parent: android.widget.AdapterView<*>?,
        view: View?,
        position: Int,
        id: Long,
    ) {
        onSelected(position)
    }

    override fun onNothingSelected(parent: android.widget.AdapterView<*>?) = Unit
}
