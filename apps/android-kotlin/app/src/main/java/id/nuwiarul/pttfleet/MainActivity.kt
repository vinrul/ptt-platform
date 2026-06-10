package id.nuwiarul.pttfleet

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
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
import id.nuwiarul.pttfleet.audio.AudioRouting
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
    companion object {
        var isVisible = false
            private set
    }

    private lateinit var binding: ActivityMainBinding
    private lateinit var tokenStore: SecureTokenStore
    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .build()
    private val authRepository = AuthRepository(httpClient)
    private val groupRepository = GroupRepository(httpClient)
    private val realtimeClient = RealtimeClient.getInstance()
    private lateinit var audioEngine: PttAudioEngine
    private var latestLocation: GpsSample? = null
    private var groups: List<GroupSummary> = emptyList()
    private var joinedGroupId: String? = null
    private var activePttSessionId: String? = null
    private var audioSequence = 0L
    private var receivedAudioFrames = 0L
    private var pttHeld = false
    private var updatingTrackingSwitch = false
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
    private val backgroundLocationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) {}
    private val startupPermissionsLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { permissions ->
        val recordAudioGranted = permissions[Manifest.permission.RECORD_AUDIO] == true
        val locationGranted = permissions[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
            permissions[Manifest.permission.ACCESS_COARSE_LOCATION] == true

        if (locationGranted) {
            startTracking()
        } else {
            binding.locationStatusText.setText(R.string.location_permission_denied)
        }

        if (recordAudioGranted) {
            binding.pttStatusText.setText(R.string.ptt_ready)
        } else {
            binding.pttStatusText.setText(R.string.audio_permission_denied)
        }

        requestBatteryOptimizationExemption()
        requestBackgroundLocationPermission()
        requestSystemAlertWindowPermission()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (intent?.getBooleanExtra("is_wakeup", false) == true) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O_MR1) {
                setShowWhenLocked(true)
                setTurnScreenOn(true)
            } else {
                @Suppress("DEPRECATION")
                window.addFlags(
                    WindowManager.LayoutParams.FLAG_SHOW_WHEN_LOCKED or
                        WindowManager.LayoutParams.FLAG_TURN_SCREEN_ON,
                )
            }
        }
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        tokenStore = SecureTokenStore(applicationContext)
        audioEngine = PttAudioEngine(
            onEncodedFrame = { payload ->
                activePttSessionId?.let { sessionId ->
                    realtimeClient.sendAudio(sessionId, audioSequence++, payload)
                }
            },
            onPlayedFrame = { samples ->
                runOnUiThread {
                    binding.pttStatusText.text = getString(R.string.ptt_playing, samples)
                }
            },
            onError = { message -> runOnUiThread { binding.pttStatusText.text = message } },
        )
        binding.loginButton.setOnClickListener { login() }
        binding.logoutButton.setOnClickListener { logout() }
        binding.locationTrackingSwitch.setOnCheckedChangeListener { _, checked ->
            if (updatingTrackingSwitch) return@setOnCheckedChangeListener
            if (checked) {
                enableTrackingFromSwitch()
            } else {
                stopTracking()
            }
        }
        binding.sosButton.setOnClickListener { confirmSos() }
        binding.groupSpinner.onItemSelectedListener = GroupSelectionListener { position ->
            groups.getOrNull(position)?.let { group ->
                joinedGroupId = null
                binding.pttButton.isEnabled = false
                realtimeClient.joinGroup(group.id)
                PatrolService.joinGroup(applicationContext, group.id)
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

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
    }

    override fun onStart() {
        super.onStart()
        isVisible = true
        realtimeClient.addListener(this)
    }

    override fun onResume() {
        super.onResume()
        if (PatrolService.isTrackingActive) {
            setTrackingSwitchChecked(true)
            val latest = PatrolService.latestLocation
            if (latest != null) {
                handleLocation(latest)
            } else {
                binding.locationStatusText.setText(R.string.location_waiting)
            }
        } else {
            setTrackingSwitchChecked(false)
            binding.locationStatusText.setText(R.string.location_stopped)
        }

        PatrolService.onLocationChangedListener = { sample ->
            runOnUiThread {
                handleLocation(sample)
            }
        }
    }

    override fun onPause() {
        PatrolService.onLocationChangedListener = null
        super.onPause()
    }

    override fun onStop() {
        realtimeClient.removeListener(this)
        isVisible = false
        super.onStop()
    }

    override fun onDestroy() {
        AudioRouting.setSpeakerphoneOn(applicationContext, false)
        audioEngine.release()
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

        realtimeClient.connect(session)

        val isWakeup = intent?.getBooleanExtra("is_wakeup", false) == true
        if (!isWakeup) {
            requestStartupPermissions()
        }

        PatrolService.start(applicationContext)
        loadGroups(session)

        // Fetch and upload Firebase Cloud Messaging token for background PTT wakeups
        com.google.firebase.messaging.FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            if (task.isSuccessful) {
                val token = task.result
                if (!token.isNullOrBlank()) {
                    id.nuwiarul.pttfleet.fcm.MyFirebaseMessagingService.uploadToken(applicationContext, token)
                }
            }
        }
    }

    private fun showLogin() {
        binding.loginPanel.visibility = View.VISIBLE
        binding.sessionPanel.visibility = View.GONE
        binding.loginError.visibility = View.GONE
        groups = emptyList()
        joinedGroupId = null
        binding.pttButton.isEnabled = false
    }

    private fun logout() {
        stopPtt()
        PatrolService.stopTracking(applicationContext)
        realtimeClient.disconnect()
        PatrolService.stop(applicationContext)
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
        realtimeClient.startPtt(groupId)
    }

    private fun stopPtt() {
        audioEngine.stopCapture()
        activePttSessionId?.let { realtimeClient.stopPtt(it) }
        activePttSessionId = null
        audioSequence = 0
        AudioRouting.setSpeakerphoneOn(applicationContext, false)
    }

    private fun enableTrackingFromSwitch() {
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
            setTrackingSwitchChecked(false)
            locationPermissionLauncher.launch(
                arrayOf(
                    Manifest.permission.ACCESS_FINE_LOCATION,
                    Manifest.permission.ACCESS_COARSE_LOCATION,
                ),
            )
        }
    }

    private fun startTracking() {
        PatrolService.startTracking(applicationContext)
        setTrackingSwitchChecked(true)
        binding.locationStatusText.setText(R.string.location_waiting)
    }

    private fun stopTracking() {
        PatrolService.stopTracking(applicationContext)
        setTrackingSwitchChecked(false)
        binding.locationStatusText.setText(R.string.location_stopped)
    }

    private fun setTrackingSwitchChecked(checked: Boolean) {
        updatingTrackingSwitch = true
        binding.locationTrackingSwitch.isChecked = checked
        updatingTrackingSwitch = false
    }

    private fun handleLocation(sample: GpsSample) {
        latestLocation = sample
        binding.locationStatusText.text = getString(
            R.string.location_sent,
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
        val sent = realtimeClient.sendSos(latestLocation) == true
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
        groups.firstOrNull()?.let { realtimeClient.joinGroup(it.id) }
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
            realtimeClient.stopPtt(sessionId)
            return
        }
        activePttSessionId = sessionId
        audioSequence = 0
        binding.pttStatusText.setText(R.string.ptt_transmitting)
        AudioRouting.setSpeakerphoneOn(applicationContext, true)
        audioEngine.startCapture()
    }

    override fun onPttBusy(groupId: String, speakerUserId: String) {
        activePttSessionId = null
        binding.pttStatusText.setText(R.string.ptt_busy)
    }

    override fun onPttStarted(sessionId: String, groupId: String, speakerUserId: String) {
        if (sessionId != activePttSessionId) {
            receivedAudioFrames = 0
            binding.pttStatusText.setText(R.string.ptt_receiving)
            AudioRouting.setSpeakerphoneOn(applicationContext, true)
        }
    }

    override fun onPttStopped(sessionId: String, groupId: String, reason: String) {
        if (sessionId == activePttSessionId) {
            audioEngine.stopCapture()
            activePttSessionId = null
        }
        binding.pttStatusText.setText(R.string.ptt_ready)
        AudioRouting.setSpeakerphoneOn(applicationContext, false)
    }

    override fun onAudioFrame(sessionId: String, sequence: Long, payload: ByteArray) {
        if (sessionId != activePttSessionId) {
            receivedAudioFrames += 1
            binding.pttStatusText.text = getString(
                R.string.ptt_receiving_frame,
                receivedAudioFrames,
                payload.size,
            )
            AudioRouting.setSpeakerphoneOn(applicationContext, true)
            audioEngine.play(payload)
        }
    }

    private fun requestStartupPermissions() {
        val permissions = mutableListOf(
            Manifest.permission.RECORD_AUDIO,
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.ACCESS_COARSE_LOCATION
        )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissions.add(Manifest.permission.POST_NOTIFICATIONS)
        }
        startupPermissionsLauncher.launch(permissions.toTypedArray())
    }

    private fun requestBatteryOptimizationExemption() {
        val powerManager = getSystemService(PowerManager::class.java)
        if (powerManager.isIgnoringBatteryOptimizations(packageName)) return
        runCatching {
            startActivity(
                Intent(
                    Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                    Uri.parse("package:$packageName"),
                ),
            )
        }
    }

    private fun requestBackgroundLocationPermission() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return
        if (
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            ) == PackageManager.PERMISSION_GRANTED
        ) {
            return
        }

        if (Build.VERSION.SDK_INT == Build.VERSION_CODES.Q) {
            backgroundLocationPermissionLauncher.launch(
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            )
            return
        }

        AlertDialog.Builder(this)
            .setTitle(R.string.background_location_permission_title)
            .setMessage(R.string.background_location_permission_message)
            .setPositiveButton(R.string.enable_permission) { _, _ ->
                runCatching {
                    startActivity(
                        Intent(
                            Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                            Uri.parse("package:$packageName"),
                        ),
                    )
                }
            }
            .setNegativeButton(R.string.try_later, null)
            .show()
    }

    private fun requestSystemAlertWindowPermission() {
        if (!Settings.canDrawOverlays(this)) {
            AlertDialog.Builder(this)
                .setTitle(R.string.permission_draw_overlays_title)
                .setMessage(R.string.permission_draw_overlays_message)
                .setPositiveButton(R.string.allow) { _, _ ->
                    runCatching {
                        startActivity(
                            Intent(
                                Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                                Uri.parse("package:$packageName")
                            )
                        )
                    }
                }
                .setNegativeButton(R.string.deny, null)
                .show()
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
