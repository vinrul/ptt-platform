package id.nuwiarul.pttfleet

import android.Manifest
import android.content.res.ColorStateList
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import android.text.method.HideReturnsTransformationMethod
import android.text.method.PasswordTransformationMethod
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
import android.widget.ArrayAdapter
import android.widget.RadioButton
import androidx.appcompat.app.AlertDialog
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AccountCircle
import androidx.compose.material.icons.rounded.Home
import androidx.compose.material.icons.rounded.Map
import androidx.compose.material.icons.rounded.Person
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.ViewCompositionStrategy
import androidx.compose.ui.res.stringResource
import androidx.core.content.ContextCompat
import androidx.core.view.ViewCompat
import androidx.core.view.updatePadding
import androidx.lifecycle.lifecycleScope
import id.nuwiarul.pttfleet.auth.AuthRepository
import id.nuwiarul.pttfleet.auth.AuthSession
import id.nuwiarul.pttfleet.auth.SessionManager
import id.nuwiarul.pttfleet.auth.ServerSettingsStore
import id.nuwiarul.pttfleet.audio.PttAudioEngine
import id.nuwiarul.pttfleet.audio.AudioRouting
import id.nuwiarul.pttfleet.databinding.ActivityMainBinding
import id.nuwiarul.pttfleet.databinding.DialogChangePasswordBinding
import id.nuwiarul.pttfleet.groups.GroupRepository
import id.nuwiarul.pttfleet.groups.DefaultGroupPreferenceStore
import id.nuwiarul.pttfleet.groups.GroupMember
import id.nuwiarul.pttfleet.groups.GroupSummary
import id.nuwiarul.pttfleet.groups.GroupLocation
import id.nuwiarul.pttfleet.fcm.PttWakeNavigation
import id.nuwiarul.pttfleet.fcm.PttWakeNavigationStore
import id.nuwiarul.pttfleet.fcm.WakeupOverlayPreferenceStore
import id.nuwiarul.pttfleet.location.GpsSample
import id.nuwiarul.pttfleet.location.LocationTracker
import id.nuwiarul.pttfleet.map.GroupMapController
import id.nuwiarul.pttfleet.map.ReverseGeocodeRepository
import id.nuwiarul.pttfleet.websocket.ConnectionStatus
import id.nuwiarul.pttfleet.websocket.RealtimeClient
import id.nuwiarul.pttfleet.websocket.RealtimeListener
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import org.maplibre.android.MapLibre
import java.util.concurrent.TimeUnit

class MainActivity : AppCompatActivity(), RealtimeListener {
    companion object {
        var isVisible = false
            private set
    }

    private lateinit var binding: ActivityMainBinding
    private lateinit var sessionManager: SessionManager
    private lateinit var serverSettingsStore: ServerSettingsStore
    private lateinit var wakeNavigationStore: PttWakeNavigationStore
    private lateinit var wakeupOverlayPreferenceStore: WakeupOverlayPreferenceStore
    private lateinit var defaultGroupPreferenceStore: DefaultGroupPreferenceStore
    private lateinit var firstRunWizardStore: FirstRunWizardStore
    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS)
        .callTimeout(30, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()
    private val authRepository = AuthRepository(httpClient)
    private val groupRepository = GroupRepository(httpClient)
    private val reverseGeocodeRepository = ReverseGeocodeRepository(httpClient)
    private val realtimeClient = RealtimeClient.getInstance()
    private lateinit var audioEngine: PttAudioEngine
    private lateinit var groupMapController: GroupMapController
    private var latestLocation: GpsSample? = null
    private var groups: List<GroupSummary> = emptyList()
    private var groupMembers: List<GroupMember> = emptyList()
    private val onlineUserIds = mutableSetOf<String>()
    private var joinedGroupId: String? = null
    private var targetUserId: String? = null
    private var activePttSessionId: String? = null
    private var pendingPttGroupId: String? = null
    private var audioSequence = 0L
    private var receivedAudioFrames = 0L
    private var pttHeld = false
    private var updatingTrackingSwitch = false
    private var updatingGroupSelection = false
    private var updatingDefaultGroupSelection = false
    private var updatingWakeupOverlaySwitch = false
    private var pendingWakeNavigation: PttWakeNavigation? = null
    private var firstRunWizardStep = FirstRunWizardStep.BASIC_PERMISSIONS
    private var firstRunSettingsIndex = 0
    private var firstRunBasicPermissionRequested = false
    private var firstRunBasicPermissionBlocked = false
    private var returningFromFirstRunBasicSettings = false
    private var firstRunPendingSetting: FirstRunSetting? = null
    private val selectedTabState: MutableState<SessionTab> = mutableStateOf(SessionTab.HOME)
    private val loadingGroupMemberIds = mutableSetOf<String>()
    private val mapLocations = mutableMapOf<String, GroupLocation>()
    private var selectedMapLocation: GroupLocation? = null
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
    ) {
        syncProfileAccessControls()
    }
    private val startupPermissionsLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { permissions ->
        val recordAudioGranted = permissions[Manifest.permission.RECORD_AUDIO] == true
        val locationGranted = permissions[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
            permissions[Manifest.permission.ACCESS_COARSE_LOCATION] == true

        if (!locationGranted) {
            binding.locationStatusText.setText(R.string.location_permission_denied)
        } else if (PatrolService.isTrackingEnabled(applicationContext)) {
            startTracking()
        } else {
            setTrackingSwitchChecked(false)
            binding.locationStatusText.setText(R.string.location_stopped)
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
    private val firstRunPermissionsLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { permissions ->
        handleFirstRunBasicPermissionResult(permissions)
    }
    private val firstRunSettingsLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) {
        if (returningFromFirstRunBasicSettings) {
            returningFromFirstRunBasicSettings = false
            if (hasFirstRunBasicPermissions()) {
                showFirstRunWizardStep(FirstRunWizardStep.BACKGROUND_ACCESS)
            } else {
                firstRunBasicPermissionBlocked = isAnyFirstRunBasicPermissionPermanentlyDenied()
                binding.firstRunWizardStatus.setText(R.string.first_run_required_permissions_missing)
                binding.firstRunWizardStatus.visibility = View.VISIBLE
                binding.firstRunWizardNextButton.setText(
                    if (firstRunBasicPermissionBlocked) {
                        R.string.first_run_open_settings
                    } else {
                        R.string.first_run_next
                    },
                )
            }
            return@registerForActivityResult
        }
        handleFirstRunSettingsReturn()
    }
    private val firstRunBackgroundLocationLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) {
        finishFirstRunWizard()
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
        MapLibre.getInstance(this)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        applySystemBarInsets()
        configureBottomNavigation()

        sessionManager = SessionManager.getInstance(applicationContext)
        serverSettingsStore = ServerSettingsStore(applicationContext)
        wakeNavigationStore = PttWakeNavigationStore(applicationContext)
        wakeupOverlayPreferenceStore = WakeupOverlayPreferenceStore(applicationContext)
        defaultGroupPreferenceStore = DefaultGroupPreferenceStore(applicationContext)
        firstRunWizardStore = FirstRunWizardStore(applicationContext)
        pendingWakeNavigation = wakeNavigationStore.consume()
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
        groupMapController = GroupMapController(binding.groupMapView, ::showMapUserDetail)
        groupMapController.onCreate(savedInstanceState)
        binding.loginButton.setOnClickListener { login() }
        binding.firstRunWizardNextButton.setOnClickListener { handleFirstRunWizardNext() }
        binding.showPasswordCheck.setOnCheckedChangeListener { _, checked ->
            setPasswordVisible(binding.passwordInput, checked)
        }
        binding.settingsButton.setOnClickListener {
            startActivity(Intent(this, SettingsActivity::class.java))
        }
        binding.changePasswordButton.setOnClickListener { showChangePasswordDialog() }
        binding.logoutButton.setOnClickListener { logout() }
        binding.wakeupOverlaySwitch.setOnCheckedChangeListener { _, checked ->
            if (updatingWakeupOverlaySwitch) return@setOnCheckedChangeListener
            wakeupOverlayPreferenceStore.setEnabled(checked)
            if (checked) {
                requestSystemAlertWindowPermission()
            }
            syncProfileAccessControls()
        }
        binding.backgroundServiceButton.setOnClickListener { enableBackgroundServiceFromProfile() }
        binding.overlayPermissionButton.setOnClickListener { openOverlaySettingsFromProfile() }
        binding.backgroundLocationButton.setOnClickListener { openBackgroundLocationSettingsFromProfile() }
        binding.saveDefaultGroupButton.setOnClickListener { saveDefaultGroupSelection() }
        binding.mapTalkButton.setOnClickListener { talkToSelectedMapUser() }
        binding.mapDetailCloseButton.setOnClickListener { hideMapUserDetail() }
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
            selectGroup(position, syncMapSpinner = true)
        }
        binding.mapGroupSpinner.onItemSelectedListener = GroupSelectionListener { position ->
            selectGroup(position, syncMainSpinner = true)
        }
        configurePttButton(binding.pttButton) { null }
        configurePttButton(binding.targetPttButton) { targetUserId }

        sessionManager.currentSession()?.let { showSession(it) } ?: run {
            if (firstRunWizardStore.isComplete()) {
                showLogin()
            } else {
                showFirstRunWizardStep(FirstRunWizardStep.BASIC_PERMISSIONS)
            }
        }
    }

    private fun applySystemBarInsets() {
        ViewCompat.setOnApplyWindowInsetsListener(binding.rootLayout) { _, insets ->
            binding.bottomNavigation.updatePadding(left = 0, top = 0, right = 0, bottom = 0)
            insets
        }
        ViewCompat.requestApplyInsets(binding.rootLayout)
    }

    private fun configureBottomNavigation() {
        binding.bottomNavigation.setViewCompositionStrategy(
            ViewCompositionStrategy.DisposeOnViewTreeLifecycleDestroyed,
        )
        binding.bottomNavigation.setContent {
            PttFleetTheme {
                PttBottomNavigation(
                    selectedTab = selectedTabState.value,
                    onTabSelected = ::showTab,
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        pendingWakeNavigation = wakeNavigationStore.consume() ?: pendingWakeNavigation
        applyPendingWakeNavigation()
    }

    override fun onStart() {
        super.onStart()
        groupMapController.onStart()
        isVisible = true
        realtimeClient.addListener(this)
    }

    override fun onResume() {
        super.onResume()
        groupMapController.onResume()
        if (PatrolService.isTrackingEnabled(applicationContext)) {
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
        syncWakeupOverlaySwitch()
        syncProfileAccessControls()

        PatrolService.onLocationChangedListener = { sample ->
            runOnUiThread {
                handleLocation(sample)
            }
        }
    }

    override fun onPause() {
        PatrolService.onLocationChangedListener = null
        groupMapController.onPause()
        super.onPause()
    }

    override fun onStop() {
        realtimeClient.removeListener(this)
        isVisible = false
        groupMapController.onStop()
        super.onStop()
    }

    override fun onDestroy() {
        AudioRouting.setSpeakerphoneOn(applicationContext, false)
        audioEngine.release()
        groupMapController.onDestroy()
        super.onDestroy()
    }

    override fun onLowMemory() {
        super.onLowMemory()
        groupMapController.onLowMemory()
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        groupMapController.onSaveInstanceState(outState)
    }

    private fun login() {
        val serverUrl = serverSettingsStore.load(getString(R.string.default_server_url))
        val username = binding.usernameInput.text.toString().trim()
        val password = binding.passwordInput.text.toString()
        if (username.isBlank() || password.isBlank()) {
            showLoginError(getString(R.string.login_fields_required))
            return
        }

        setLoginLoading(true)
        binding.loginError.visibility = View.GONE
        lifecycleScope.launch {
            runCatching {
                authRepository.login(serverUrl, username, password)
            }.onSuccess { session ->
                sessionManager.save(session)
                binding.passwordInput.text?.clear()
                showSession(session)
            }.onFailure { error ->
                showLoginError(error.message ?: getString(R.string.login_failed))
            }
            setLoginLoading(false)
        }
    }

    private fun showSession(session: AuthSession) {
        binding.firstRunWizardPanel.visibility = View.GONE
        binding.settingsButton.visibility = View.GONE
        binding.loginTitle.visibility = View.GONE
        binding.loginSubtitle.visibility = View.GONE
        binding.loginPanel.visibility = View.GONE
        binding.sessionPanel.visibility = View.VISIBLE
        binding.bottomNavigation.visibility = View.VISIBLE
        binding.userNameText.text = session.user.fullName
        binding.userRoleText.text = getString(
            R.string.user_identity,
            session.user.username,
            session.user.role.replace('_', ' '),
        )
        syncWakeupOverlaySwitch()
        syncProfileAccessControls()
        binding.connectionDetailText.setText(R.string.realtime_opening)
        showTab(
            if (pendingWakeNavigation?.isDirect == true) SessionTab.TARGET else SessionTab.HOME,
        )

        val isWakeup = intent?.getBooleanExtra("is_wakeup", false) == true
        if (!isWakeup && !firstRunWizardStore.isComplete()) {
            requestStartupPermissions()
        }

        PatrolService.start(applicationContext)
        loadGroups()

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
        binding.contentScroll.visibility = View.VISIBLE
        binding.mapPanel.visibility = View.GONE
        binding.firstRunWizardPanel.visibility = View.GONE
        binding.settingsButton.visibility = View.VISIBLE
        binding.loginTitle.visibility = View.VISIBLE
        binding.loginSubtitle.visibility = View.VISIBLE
        binding.loginPanel.visibility = View.VISIBLE
        binding.sessionPanel.visibility = View.GONE
        binding.bottomNavigation.visibility = View.GONE
        binding.loginError.visibility = View.GONE
        groups = emptyList()
        groupMembers = emptyList()
        onlineUserIds.clear()
        joinedGroupId = null
        targetUserId = null
        renderGroupMembers(emptyList())
        binding.pttButton.isEnabled = false
        binding.targetPttButton.isEnabled = false
    }

    private fun showFirstRunWizardStep(step: FirstRunWizardStep) {
        firstRunWizardStep = step
        binding.contentScroll.visibility = View.VISIBLE
        binding.mapPanel.visibility = View.GONE
        binding.bottomNavigation.visibility = View.GONE
        binding.settingsButton.visibility = View.GONE
        binding.loginTitle.visibility = View.GONE
        binding.loginSubtitle.visibility = View.GONE
        binding.loginPanel.visibility = View.GONE
        binding.sessionPanel.visibility = View.GONE
        binding.loginError.visibility = View.GONE
        binding.firstRunWizardPanel.visibility = View.VISIBLE

        when (step) {
            FirstRunWizardStep.BASIC_PERMISSIONS -> {
                binding.firstRunWizardStepText.setText(R.string.first_run_step_1)
                binding.firstRunWizardTitle.setText(R.string.first_run_basic_title)
                binding.firstRunWizardDescription.setText(R.string.first_run_basic_description)
                binding.firstRunWizardList.setText(R.string.first_run_basic_list)
                binding.firstRunWizardNextButton.setText(R.string.first_run_next)
                binding.firstRunWizardStatus.visibility = View.GONE
            }
            FirstRunWizardStep.BACKGROUND_ACCESS -> {
                binding.firstRunWizardStepText.setText(R.string.first_run_step_2)
                binding.firstRunWizardTitle.setText(R.string.first_run_background_title)
                binding.firstRunWizardDescription.setText(R.string.first_run_background_description)
                binding.firstRunWizardList.setText(R.string.first_run_background_list)
                renderFirstRunBackgroundAccessState()
            }
        }
    }

    private fun handleFirstRunWizardNext() {
        when (firstRunWizardStep) {
            FirstRunWizardStep.BASIC_PERMISSIONS -> requestFirstRunBasicPermissions()
            FirstRunWizardStep.BACKGROUND_ACCESS -> handleFirstRunBackgroundAccessNext()
        }
    }

    private fun requestFirstRunBasicPermissions() {
        if (hasFirstRunBasicPermissions()) {
            showFirstRunWizardStep(FirstRunWizardStep.BACKGROUND_ACCESS)
            return
        }
        if (firstRunBasicPermissionBlocked) {
            openAppSettingsForFirstRunPermissions()
            return
        }
        val permissions = mutableListOf(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.ACCESS_COARSE_LOCATION,
            Manifest.permission.RECORD_AUDIO,
        )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissions.add(Manifest.permission.POST_NOTIFICATIONS)
        }
        firstRunBasicPermissionRequested = true
        firstRunPermissionsLauncher.launch(permissions.toTypedArray())
    }

    private fun handleFirstRunBasicPermissionResult(permissions: Map<String, Boolean>) {
        if (hasFirstRunBasicPermissions(permissions)) {
            firstRunBasicPermissionBlocked = false
            binding.firstRunWizardStatus.visibility = View.GONE
            showFirstRunWizardStep(FirstRunWizardStep.BACKGROUND_ACCESS)
            return
        }

        firstRunBasicPermissionBlocked = isAnyFirstRunBasicPermissionPermanentlyDenied()
        binding.firstRunWizardStatus.setText(R.string.first_run_required_permissions_missing)
        binding.firstRunWizardStatus.visibility = View.VISIBLE
        binding.firstRunWizardNextButton.setText(
            if (firstRunBasicPermissionBlocked) {
                R.string.first_run_open_settings
            } else {
                R.string.first_run_next
            },
        )
    }

    private fun hasFirstRunBasicPermissions(
        permissionResult: Map<String, Boolean>? = null,
    ): Boolean {
        val hasLocation = permissionResult?.let {
            it[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
                it[Manifest.permission.ACCESS_COARSE_LOCATION] == true
        } ?: (
            ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_FINE_LOCATION) ==
                PackageManager.PERMISSION_GRANTED ||
                ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_COARSE_LOCATION) ==
                PackageManager.PERMISSION_GRANTED
            )
        val hasAudio = permissionResult?.get(Manifest.permission.RECORD_AUDIO)
            ?: (
                ContextCompat.checkSelfPermission(this, Manifest.permission.RECORD_AUDIO) ==
                    PackageManager.PERMISSION_GRANTED
                )
        val hasNotification = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissionResult?.get(Manifest.permission.POST_NOTIFICATIONS)
                ?: (
                    ContextCompat.checkSelfPermission(
                        this,
                        Manifest.permission.POST_NOTIFICATIONS,
                    ) == PackageManager.PERMISSION_GRANTED
                    )
        } else {
            true
        }
        return hasLocation && hasAudio && hasNotification
    }

    private fun isAnyFirstRunBasicPermissionPermanentlyDenied(): Boolean {
        if (!firstRunBasicPermissionRequested) return false
        val locationDenied =
            ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_FINE_LOCATION) !=
                PackageManager.PERMISSION_GRANTED &&
                ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_COARSE_LOCATION) !=
                PackageManager.PERMISSION_GRANTED &&
                !shouldShowRequestPermissionRationale(Manifest.permission.ACCESS_FINE_LOCATION) &&
                !shouldShowRequestPermissionRationale(Manifest.permission.ACCESS_COARSE_LOCATION)
        val audioDenied =
            ContextCompat.checkSelfPermission(this, Manifest.permission.RECORD_AUDIO) !=
                PackageManager.PERMISSION_GRANTED &&
                !shouldShowRequestPermissionRationale(Manifest.permission.RECORD_AUDIO)
        val notificationDenied = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) !=
            PackageManager.PERMISSION_GRANTED &&
            !shouldShowRequestPermissionRationale(Manifest.permission.POST_NOTIFICATIONS)
        return locationDenied || audioDenied || notificationDenied
    }

    private fun openAppSettingsForFirstRunPermissions() {
        runCatching {
            returningFromFirstRunBasicSettings = true
            firstRunSettingsLauncher.launch(
                Intent(
                    Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                    Uri.parse("package:$packageName"),
                ),
            )
        }.onFailure {
            binding.firstRunWizardStatus.setText(R.string.first_run_required_permissions_missing)
            binding.firstRunWizardStatus.visibility = View.VISIBLE
        }
    }

    private fun handleFirstRunBackgroundAccessNext() {
        when (firstRunSettingsIndex) {
            0 -> {
                PatrolService.start(applicationContext)
                firstRunSettingsIndex = 1
                syncProfileAccessControls()
                renderFirstRunBackgroundAccessState(R.string.first_run_background_service_enabled)
            }
            1 -> {
                wakeupOverlayPreferenceStore.setEnabled(true)
                syncWakeupOverlaySwitch()
                if (launchOverlaySetting()) return
                firstRunSettingsIndex = 2
                renderFirstRunBackgroundAccessState(R.string.first_run_overlay_reviewed)
            }
            2 -> {
                if (launchBackgroundLocationSetting()) return
                firstRunSettingsIndex = 3
                renderFirstRunBackgroundAccessState(R.string.first_run_background_location_reviewed)
            }
            else -> finishFirstRunWizard()
        }
    }

    private fun handleFirstRunSettingsReturn() {
        when (firstRunPendingSetting) {
            FirstRunSetting.OVERLAY -> {
                firstRunPendingSetting = null
                firstRunSettingsIndex = 2
                renderFirstRunBackgroundAccessState(R.string.first_run_overlay_reviewed)
            }
            FirstRunSetting.BACKGROUND_LOCATION -> {
                firstRunPendingSetting = null
                firstRunSettingsIndex = 3
                renderFirstRunBackgroundAccessState(R.string.first_run_background_location_reviewed)
            }
            null -> Unit
        }
    }

    private fun renderFirstRunBackgroundAccessState(statusRes: Int? = null) {
        binding.firstRunWizardNextButton.setText(
            when (firstRunSettingsIndex) {
                0 -> R.string.first_run_enable_background_service
                1 -> R.string.first_run_enable_overlay
                2 -> R.string.first_run_enable_all_time_location
                else -> R.string.first_run_finish
            },
        )
        if (statusRes == null) {
            binding.firstRunWizardStatus.visibility = View.GONE
        } else {
            binding.firstRunWizardStatus.setText(statusRes)
            binding.firstRunWizardStatus.visibility = View.VISIBLE
        }
    }

    private fun launchOverlaySetting(): Boolean {
        if (Settings.canDrawOverlays(this)) return false
        return runCatching {
            firstRunPendingSetting = FirstRunSetting.OVERLAY
            firstRunSettingsLauncher.launch(
                Intent(
                    Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                    Uri.parse("package:$packageName"),
                ),
            )
        }.isSuccess
    }

    private fun launchBackgroundLocationSetting(): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return false
        if (
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            ) == PackageManager.PERMISSION_GRANTED
        ) {
            return false
        }
        if (Build.VERSION.SDK_INT == Build.VERSION_CODES.Q) {
            firstRunBackgroundLocationLauncher.launch(
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            )
            return true
        }
        return runCatching {
            firstRunPendingSetting = FirstRunSetting.BACKGROUND_LOCATION
            firstRunSettingsLauncher.launch(
                Intent(
                    Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                    Uri.parse("package:$packageName"),
                ),
            )
        }.isSuccess
    }

    private fun finishFirstRunWizard() {
        firstRunWizardStore.markComplete()
        showLogin()
    }

    private fun logout() {
        stopPtt()
        PatrolService.stopTracking(applicationContext)
        realtimeClient.disconnect()
        PatrolService.stop(applicationContext)
        sessionManager.clear()
        binding.passwordInput.text?.clear()
        binding.showPasswordCheck.isChecked = false
        showLogin()
    }

    private fun showChangePasswordDialog() {
        if (sessionManager.currentSession() == null) return
        val dialogBinding = DialogChangePasswordBinding.inflate(layoutInflater)
        val dialog = AlertDialog.Builder(this)
            .setTitle(R.string.change_password_title)
            .setView(dialogBinding.root)
            .setNegativeButton(R.string.cancel, null)
            .setPositiveButton(R.string.save_password, null)
            .create()

        dialogBinding.showPasswordsCheck.setOnCheckedChangeListener { _, checked ->
            setPasswordVisible(dialogBinding.currentPasswordInput, checked)
            setPasswordVisible(dialogBinding.newPasswordInput, checked)
            setPasswordVisible(dialogBinding.confirmPasswordInput, checked)
        }
        dialog.setOnShowListener {
            dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener {
                val currentPassword = dialogBinding.currentPasswordInput.text.toString()
                val newPassword = dialogBinding.newPasswordInput.text.toString()
                val confirmation = dialogBinding.confirmPasswordInput.text.toString()
                val validationMessage = when {
                    currentPassword.isBlank() -> getString(R.string.current_password_required)
                    newPassword.length < 8 -> getString(R.string.password_min_length)
                    newPassword != confirmation -> getString(R.string.password_confirmation_mismatch)
                    else -> null
                }
                if (validationMessage != null) {
                    dialogBinding.changePasswordError.text = validationMessage
                    dialogBinding.changePasswordError.visibility = View.VISIBLE
                    return@setOnClickListener
                }

                dialog.getButton(AlertDialog.BUTTON_POSITIVE).isEnabled = false
                dialogBinding.changePasswordError.visibility = View.GONE
                lifecycleScope.launch {
                    runCatching {
                        val session = sessionManager.validSession()
                            ?: throw IllegalStateException(getString(R.string.login_failed))
                        authRepository.changePassword(session, currentPassword, newPassword)
                    }.onSuccess {
                        dialog.dismiss()
                        AlertDialog.Builder(this@MainActivity)
                            .setMessage(R.string.password_changed)
                            .setPositiveButton(android.R.string.ok) { _, _ -> logout() }
                            .setCancelable(false)
                            .show()
                    }.onFailure { error ->
                        dialogBinding.changePasswordError.text =
                            error.message ?: getString(R.string.login_failed)
                        dialogBinding.changePasswordError.visibility = View.VISIBLE
                        dialog.getButton(AlertDialog.BUTTON_POSITIVE).isEnabled = true
                    }
                }
            }
        }
        dialog.show()
    }

    private fun setPasswordVisible(input: android.widget.EditText, visible: Boolean) {
        val cursor = input.selectionStart
        input.transformationMethod = if (visible) {
            HideReturnsTransformationMethod.getInstance()
        } else {
            PasswordTransformationMethod.getInstance()
        }
        input.setSelection(cursor.coerceAtLeast(0))
    }

    private fun loadGroups() {
        lifecycleScope.launch {
            runCatching {
                val session = sessionManager.validSession()
                    ?: throw IllegalStateException(getString(R.string.login_failed))
                groupRepository.list(session)
                }
                .onSuccess { items ->
                    groups = sortedGroupsByDefault(items)
                    val groupAdapter = ArrayAdapter(
                        this@MainActivity,
                        android.R.layout.simple_spinner_dropdown_item,
                        groups.map { it.name },
                    )
                    updatingGroupSelection = true
                    binding.groupSpinner.adapter = groupAdapter
                    binding.mapGroupSpinner.adapter = groupAdapter
                    binding.defaultGroupSpinner.adapter = groupAdapter
                    updatingGroupSelection = false
                    syncDefaultGroupSpinner()
                    if (groups.isNotEmpty()) {
                        selectGroup(0, syncMainSpinner = true, syncMapSpinner = true)
                    }
                    applyPendingWakeNavigation()
                    binding.pttStatusText.setText(
                        if (groups.isEmpty()) R.string.no_groups else R.string.channel_joining,
                    )
                    updateDefaultGroupStatus()
                }
                .onFailure { binding.pttStatusText.text = it.message }
        }
    }

    private fun sortedGroupsByDefault(items: List<GroupSummary>): List<GroupSummary> {
        val defaultGroupId = defaultGroupPreferenceStore.getDefaultGroupId() ?: return items
        if (items.none { it.id == defaultGroupId }) {
            defaultGroupPreferenceStore.clear()
            return items
        }
        return items.sortedWith(compareByDescending<GroupSummary> { it.id == defaultGroupId }
            .thenBy { it.name.lowercase() })
    }

    private fun syncDefaultGroupSpinner() {
        val defaultGroupId = defaultGroupPreferenceStore.getDefaultGroupId()
        val selectedIndex = defaultGroupId
            ?.let { groupId -> groups.indexOfFirst { it.id == groupId } }
            ?.takeIf { it >= 0 }
            ?: 0
        if (groups.isNotEmpty() && binding.defaultGroupSpinner.selectedItemPosition != selectedIndex) {
            updatingDefaultGroupSelection = true
            binding.defaultGroupSpinner.setSelection(selectedIndex)
            updatingDefaultGroupSelection = false
        }
    }

    private fun updateDefaultGroupStatus(savedMessage: String? = null) {
        if (savedMessage != null) {
            binding.defaultGroupStatusText.text = savedMessage
            return
        }
        val defaultGroup = defaultGroupPreferenceStore.getDefaultGroupId()
            ?.let { groupId -> groups.firstOrNull { it.id == groupId } }
        binding.defaultGroupStatusText.text = defaultGroup?.let {
            getString(R.string.profile_default_group_current, it.name)
        } ?: getString(R.string.profile_default_group_empty)
    }

    private fun saveDefaultGroupSelection() {
        if (updatingDefaultGroupSelection) return
        val group = groups.getOrNull(binding.defaultGroupSpinner.selectedItemPosition) ?: return
        val currentGroupId = selectedGroupId()
        defaultGroupPreferenceStore.setDefaultGroupId(group.id)
        groups = sortedGroupsByDefault(groups)
        val adapter = ArrayAdapter(
            this,
            android.R.layout.simple_spinner_dropdown_item,
            groups.map { it.name },
        )
        updatingGroupSelection = true
        binding.groupSpinner.adapter = adapter
        binding.mapGroupSpinner.adapter = adapter
        binding.defaultGroupSpinner.adapter = adapter
        currentGroupId
            ?.let { groupId -> groups.indexOfFirst { it.id == groupId } }
            ?.takeIf { it >= 0 }
            ?.let { position ->
                binding.groupSpinner.setSelection(position)
                binding.mapGroupSpinner.setSelection(position)
            }
        updatingGroupSelection = false
        syncDefaultGroupSpinner()
        updateDefaultGroupStatus(getString(R.string.profile_default_group_saved, group.name))
    }

    private fun selectGroup(
        position: Int,
        syncMainSpinner: Boolean = false,
        syncMapSpinner: Boolean = false,
    ) {
        if (updatingGroupSelection) return
        val group = groups.getOrNull(position) ?: return

        updatingGroupSelection = true
        if (syncMainSpinner && binding.groupSpinner.selectedItemPosition != position) {
            binding.groupSpinner.setSelection(position)
        }
        if (syncMapSpinner && binding.mapGroupSpinner.selectedItemPosition != position) {
            binding.mapGroupSpinner.setSelection(position)
        }
        updatingGroupSelection = false

        joinedGroupId = null
        targetUserId = null
        onlineUserIds.clear()
        binding.pttButton.isEnabled = false
        binding.targetPttButton.isEnabled = false
        renderGroupMembers(emptyList())
        clearMapLocations()
        realtimeClient.joinGroup(group.id)
        PatrolService.joinGroup(applicationContext, group.id)
        loadGroupMembers(group.id)
        loadGroupLocations(group.id)
    }

    private fun selectedGroupId(): String? =
        groups.getOrNull(binding.groupSpinner.selectedItemPosition)?.id

    private fun loadGroupMembers(groupId: String) {
        if (!loadingGroupMemberIds.add(groupId)) return
        binding.pttStatusText.setText(R.string.members_loading)
        lifecycleScope.launch {
            try {
                var activeSession: AuthSession? = null
                runCatching {
                    sessionManager.validSession()
                        ?.also { activeSession = it }
                        ?.let { groupRepository.members(it, groupId) }
                        ?: throw IllegalStateException(getString(R.string.login_failed))
                }.onSuccess { members ->
                    if (selectedGroupId() != groupId) {
                        return@onSuccess
                    }
                    val wakeNavigation = pendingWakeNavigation
                        ?.takeIf { it.isDirect && it.groupId == groupId }
                    val wakeTarget = wakeNavigation?.let { wake ->
                        members.firstOrNull { it.userId == wake.speakerUserId } ?:
                            members.firstOrNull { it.username == wake.speakerUsername }
                    }
                    groupMembers = members.filter {
                        it.userId != activeSession?.user?.id &&
                            (
                                (it.role != "super_admin" && it.role != "dispatcher") ||
                                    it.userId == wakeTarget?.userId
                                )
                    }
                    if (wakeTarget != null) {
                        targetUserId = wakeTarget.userId
                        showTab(SessionTab.TARGET)
                        pendingWakeNavigation = null
                    }
                    renderGroupMembers(groupMembers)
                    if (joinedGroupId == groupId) {
                        updateReadyStatus()
                    }
                }.onFailure {
                    groupMembers = emptyList()
                    renderGroupMembers(emptyList())
                    binding.pttStatusText.setText(R.string.members_load_failed)
                }
            } finally {
                loadingGroupMemberIds -= groupId
            }
        }
    }

    private fun loadGroupLocations(groupId: String) {
        binding.mapStatusText.setText(R.string.map_waiting)
        lifecycleScope.launch {
            runCatching {
                val session = sessionManager.validSession()
                    ?: throw IllegalStateException(getString(R.string.login_failed))
                groupRepository.latestLocations(session, groupId)
            }.onSuccess { items ->
                if (selectedGroupId() != groupId) {
                    return@onSuccess
                }
                val visibleItems = items.filter {
                    it.role != "super_admin" && it.role != "dispatcher"
                }
                mapLocations.clear()
                visibleItems.forEach { mapLocations[it.userId] = it }
                groupMapController.replaceLocations(visibleItems)
                binding.mapStatusText.text = if (visibleItems.isEmpty()) {
                    getString(R.string.map_empty)
                } else {
                    getString(R.string.map_locations_shown, visibleItems.size)
                }
            }.onFailure {
                binding.mapStatusText.setText(R.string.map_load_failed)
            }
        }
    }

    private fun clearMapLocations() {
        mapLocations.clear()
        selectedMapLocation = null
        hideMapUserDetail()
        binding.mapStatusText.setText(R.string.map_waiting)
        groupMapController.clear()
    }

    private fun showMapUserDetail(location: GroupLocation) {
        selectedMapLocation = location
        binding.mapUserDetailPanel.visibility = View.VISIBLE
        binding.mapUserNameText.text = getString(
            R.string.map_user_detail,
            location.fullName,
            location.username,
        )
        val accuracy = location.accuracy?.let {
            getString(R.string.map_accuracy_meters, it)
        } ?: getString(R.string.map_accuracy_unknown)
        binding.mapUserLocationText.text = getString(
            R.string.map_location_detail,
            location.recordedAt,
            accuracy,
        )
        binding.mapUserAddressText.setText(R.string.map_address_loading)
        binding.mapTalkButton.isEnabled = groupMembers.any { it.userId == location.userId }
        loadMapUserAddress(location)
    }

    private fun hideMapUserDetail() {
        selectedMapLocation = null
        binding.mapUserDetailPanel.visibility = View.GONE
    }

    private fun loadMapUserAddress(location: GroupLocation) {
        lifecycleScope.launch {
            runCatching {
                val session = sessionManager.validSession() ?: throw IllegalStateException("Session unavailable")
                reverseGeocodeRepository.reverse(session, location.lat, location.lng)
            }.onSuccess { address ->
                if (selectedMapLocation?.userId == location.userId) {
                    binding.mapUserAddressText.text = address
                }
            }.onFailure {
                if (selectedMapLocation?.userId == location.userId) {
                    binding.mapUserAddressText.setText(R.string.map_address_failed)
                }
            }
        }
    }

    private fun talkToSelectedMapUser() {
        val location = selectedMapLocation ?: return
        if (groupMembers.none { it.userId == location.userId }) return
        targetUserId = location.userId
        renderGroupMembers(groupMembers)
        showTab(SessionTab.TARGET)
    }

    private fun applyPendingWakeNavigation() {
        val navigation = pendingWakeNavigation ?: return
        showTab(if (navigation.isDirect) SessionTab.TARGET else SessionTab.HOME)
        val groupIndex = groups.indexOfFirst { it.id == navigation.groupId }
        if (groupIndex < 0) return

        val currentGroupId = groups.getOrNull(binding.groupSpinner.selectedItemPosition)?.id
        if (currentGroupId != navigation.groupId) {
            binding.groupSpinner.setSelection(groupIndex)
            if (!navigation.isDirect) {
                pendingWakeNavigation = null
            }
            return
        }
        if (navigation.isDirect) {
            loadGroupMembers(navigation.groupId)
        } else {
            pendingWakeNavigation = null
        }
    }

    private fun renderGroupMembers(members: List<GroupMember>) {
        binding.groupMembersList.setOnCheckedChangeListener(null)
        binding.groupMembersList.removeAllViews()

        members.forEach { member ->
            val isOnline = member.userId in onlineUserIds
            binding.groupMembersList.addView(
                RadioButton(this).apply {
                    id = View.generateViewId()
                    text = getString(
                        R.string.talk_to_user,
                        member.fullName,
                        member.username,
                        getString(
                            if (isOnline) R.string.presence_online else R.string.presence_offline,
                        ),
                    )
                    setTextColor(
                        ContextCompat.getColor(
                            this@MainActivity,
                            if (isOnline) R.color.fleet_primary else R.color.fleet_muted,
                        ),
                    )
                    tag = member.userId
                    isChecked = targetUserId == member.userId
                    buttonTintList = controlButtonTint()
                },
            )
        }
        binding.groupMembersList.setOnCheckedChangeListener { group, checkedId ->
            targetUserId = group.findViewById<RadioButton>(checkedId)?.tag as? String
            updateTargetButton()
            updateReadyStatus()
        }
        updateTargetButton()
    }

    private fun controlButtonTint(): ColorStateList = ColorStateList(
        arrayOf(
            intArrayOf(android.R.attr.state_checked),
            intArrayOf(-android.R.attr.state_enabled),
            intArrayOf(),
        ),
        intArrayOf(
            ContextCompat.getColor(this, R.color.fleet_primary),
            ContextCompat.getColor(this, R.color.fleet_control_disabled),
            ContextCompat.getColor(this, R.color.fleet_control_off),
        ),
    )

    private fun selectedTargetName(): String? =
        targetUserId?.let { selectedId ->
            groupMembers.firstOrNull { it.userId == selectedId }?.fullName
        }

    private fun updateReadyStatus() {
        val targetName = selectedTargetName()
        binding.pttStatusText.text = when {
            binding.targetPanel.visibility != View.VISIBLE -> getString(R.string.ptt_ready)
            targetName == null -> getString(R.string.select_target_first)
            else -> getString(R.string.ptt_ready_private, targetName)
        }
    }

    private fun requestPttStart(requestedTargetUserId: String?) {
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
        if (binding.targetPanel.visibility == View.VISIBLE && requestedTargetUserId == null) {
            binding.pttStatusText.setText(R.string.select_target_first)
            return
        }
        val targetName = groupMembers
            .firstOrNull { it.userId == requestedTargetUserId }
            ?.fullName
        binding.pttStatusText.text = if (targetName == null) {
            getString(R.string.ptt_requesting)
        } else {
            getString(R.string.ptt_requesting_private, targetName)
        }
        pendingPttGroupId = groupId
        if (!realtimeClient.startPtt(groupId, requestedTargetUserId)) {
            pendingPttGroupId = null
            binding.pttStatusText.setText(R.string.channel_not_ready)
        }
    }

    private fun configurePttButton(
        button: View,
        targetUserId: () -> String?,
    ) {
        button.setOnTouchListener { _, motionEvent ->
            when (motionEvent.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    button.isPressed = true
                    pttHeld = true
                    requestPttStart(targetUserId())
                    true
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    button.isPressed = false
                    pttHeld = false
                    stopPtt()
                    true
                }
                else -> false
            }
        }
    }

    private fun updateTargetButton() {
        val targetName = selectedTargetName()
        binding.targetPttButton.isEnabled = joinedGroupId != null && targetName != null
        binding.targetPttButton.text = if (targetName == null) {
            getString(R.string.select_target_first)
        } else {
            getString(R.string.hold_to_target, targetName)
        }
    }

    private fun showTab(tab: SessionTab) {
        selectedTabState.value = tab
        val showMap = tab == SessionTab.MAP
        binding.contentScroll.visibility = if (showMap) View.GONE else View.VISIBLE
        binding.mapPanel.visibility = if (showMap) View.VISIBLE else View.GONE
        binding.homePanel.visibility = if (tab == SessionTab.HOME) View.VISIBLE else View.GONE
        binding.targetPanel.visibility = if (tab == SessionTab.TARGET) View.VISIBLE else View.GONE
        binding.profilePanel.visibility = if (tab == SessionTab.PROFILE) View.VISIBLE else View.GONE
        binding.pttStatusText.visibility =
            if (tab == SessionTab.PROFILE) View.GONE else View.VISIBLE

        if (tab != SessionTab.PROFILE) {
            updateReadyStatus()
        }
    }

    private fun stopPtt() {
        binding.pttButton.isPressed = false
        binding.targetPttButton.isPressed = false
        audioEngine.stopCapture()
        activePttSessionId?.let { realtimeClient.stopPtt(it) }
        if (activePttSessionId == null) {
            pendingPttGroupId?.let { realtimeClient.cancelPtt(it) }
        }
        activePttSessionId = null
        pendingPttGroupId = null
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

    private fun syncWakeupOverlaySwitch() {
        updatingWakeupOverlaySwitch = true
        binding.wakeupOverlaySwitch.isChecked = wakeupOverlayPreferenceStore.isEnabled()
        updatingWakeupOverlaySwitch = false
    }

    private fun syncProfileAccessControls() {
        val backgroundServiceEnabled = PatrolService.isEnabled(applicationContext)
        binding.backgroundServiceStatusText.setText(
            if (backgroundServiceEnabled) {
                R.string.profile_background_service_enabled
            } else {
                R.string.profile_background_service_disabled
            },
        )
        binding.backgroundServiceButton.isEnabled = !backgroundServiceEnabled

        val overlayEnabled = Settings.canDrawOverlays(this)
        binding.overlayPermissionStatusText.setText(
            if (overlayEnabled) R.string.profile_overlay_enabled else R.string.profile_overlay_disabled,
        )
        binding.overlayPermissionButton.isEnabled = !overlayEnabled

        val backgroundLocationEnabled = hasBackgroundLocationPermission()
        binding.backgroundLocationStatusText.setText(
            if (backgroundLocationEnabled) {
                R.string.profile_background_location_enabled
            } else {
                R.string.profile_background_location_disabled
            },
        )
        binding.backgroundLocationButton.isEnabled = !backgroundLocationEnabled
    }

    private fun enableBackgroundServiceFromProfile() {
        PatrolService.start(applicationContext)
        syncProfileAccessControls()
    }

    private fun openOverlaySettingsFromProfile() {
        if (Settings.canDrawOverlays(this)) {
            syncProfileAccessControls()
            return
        }
        runCatching {
            startActivity(
                Intent(
                    Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                    Uri.parse("package:$packageName"),
                ),
            )
        }
    }

    private fun openBackgroundLocationSettingsFromProfile() {
        if (hasBackgroundLocationPermission()) {
            syncProfileAccessControls()
            return
        }
        if (Build.VERSION.SDK_INT == Build.VERSION_CODES.Q) {
            backgroundLocationPermissionLauncher.launch(
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            )
            return
        }
        runCatching {
            startActivity(
                Intent(
                    Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                    Uri.parse("package:$packageName"),
                ),
            )
        }
    }

    private fun hasBackgroundLocationPermission(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.Q ||
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            ) == PackageManager.PERMISSION_GRANTED

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
            binding.targetPttButton.isEnabled = false
        }
    }

    override fun onReady(connectionId: String) {
        binding.connectionDetailText.text = getString(R.string.realtime_ready, connectionId.take(8))
        groups.firstOrNull()?.let { realtimeClient.joinGroup(it.id) }
    }

    override fun onError(message: String) {
        binding.connectionDetailText.text = message
    }

    override fun onGroupJoined(groupId: String, onlineUserIds: Set<String>) {
        joinedGroupId = groupId
        this.onlineUserIds.clear()
        this.onlineUserIds.addAll(onlineUserIds)
        renderGroupMembers(groupMembers)
        binding.pttButton.isEnabled = true
        updateTargetButton()
        updateReadyStatus()
    }

    override fun onPresenceUpdated(userId: String, status: String) {
        if (status == "online") {
            onlineUserIds += userId
        } else {
            onlineUserIds -= userId
        }
        if (groupMembers.any { it.userId == userId }) {
            renderGroupMembers(groupMembers)
        }
    }

    override fun onGpsUpdated(
        groupId: String,
        userId: String,
        lat: Double,
        lng: Double,
        speed: Double?,
        heading: Double?,
        accuracy: Double?,
        recordedAt: String,
    ) {
        if (groupId != joinedGroupId) return
        val existing = mapLocations[userId]
        val member = groupMembers.firstOrNull { it.userId == userId }
        if (existing == null && member == null) return
        val item = GroupLocation(
            userId = userId,
            username = member?.username ?: existing!!.username,
            fullName = member?.fullName ?: existing!!.fullName,
            role = member?.role ?: existing!!.role,
            lat = lat,
            lng = lng,
            speed = speed,
            heading = heading,
            accuracy = accuracy,
            recordedAt = recordedAt,
        )
        mapLocations[userId] = item
        groupMapController.updateLocation(item)
        if (selectedMapLocation?.userId == userId) {
            showMapUserDetail(item)
        }
    }

    override fun onPttGranted(sessionId: String, groupId: String) {
        pendingPttGroupId = null
        if (!pttHeld || joinedGroupId != groupId) {
            realtimeClient.stopPtt(sessionId)
            return
        }
        activePttSessionId = sessionId
        audioSequence = 0
        binding.pttStatusText.setText(R.string.ptt_transmitting)
        PatrolService.sendPttLocationSnapshot(applicationContext)
        AudioRouting.setSpeakerphoneOn(applicationContext, true)
        audioEngine.startCapture()
    }

    override fun onPttBusy(groupId: String, speakerUserId: String, queuePosition: Int?) {
        activePttSessionId = null
        if (queuePosition == null) {
            pendingPttGroupId = null
            binding.pttStatusText.setText(R.string.ptt_busy)
        } else {
            pendingPttGroupId = groupId
            binding.pttStatusText.text = getString(R.string.ptt_queued, queuePosition)
        }
    }

    override fun onPttStarted(sessionId: String, groupId: String, speakerUserId: String) {
        if (sessionId != activePttSessionId) {
            audioEngine.stopPlayback()
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
        audioEngine.stopPlayback()
        updateReadyStatus()
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
            audioEngine.play(sessionId, sequence, payload)
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
        if (!wakeupOverlayPreferenceStore.isEnabled()) return
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

private enum class SessionTab {
    HOME,
    TARGET,
    MAP,
    PROFILE,
}

private enum class FirstRunWizardStep {
    BASIC_PERMISSIONS,
    BACKGROUND_ACCESS,
}

private enum class FirstRunSetting {
    OVERLAY,
    BACKGROUND_LOCATION,
}

private data class BottomNavItem(
    val tab: SessionTab,
    val labelRes: Int,
    val icon: ImageVector,
)

@Composable
private fun PttFleetTheme(content: @Composable () -> Unit) {
    val darkColors = darkColorScheme(
        primary = Color(0xFF34D399),
        onPrimary = Color(0xFF06251A),
        secondary = Color(0xFFF59E0B),
        background = Color(0xFF08110F),
        surface = Color(0xFF101815),
        surfaceContainer = Color(0xFF14211D),
        onSurface = Color(0xFFEAFBF4),
        onSurfaceVariant = Color(0xFF8EA99C),
    )
    val lightColors = lightColorScheme(
        primary = Color(0xFF047857),
        onPrimary = Color(0xFFFFFFFF),
        secondary = Color(0xFF9A5B00),
        background = Color(0xFFF5F8F5),
        surface = Color(0xFFFFFFFF),
        surfaceContainer = Color(0xFFEDF3EF),
        onSurface = Color(0xFF10201A),
        onSurfaceVariant = Color(0xFF5F6F68),
    )
    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) darkColors else lightColors,
        content = content,
    )
}

@Composable
private fun PttBottomNavigation(
    selectedTab: SessionTab,
    onTabSelected: (SessionTab) -> Unit,
) {
    val items = listOf(
        BottomNavItem(SessionTab.HOME, R.string.tab_home, Icons.Rounded.Home),
        BottomNavItem(SessionTab.TARGET, R.string.tab_talk_target, Icons.Rounded.Person),
        BottomNavItem(SessionTab.MAP, R.string.tab_map, Icons.Rounded.Map),
        BottomNavItem(SessionTab.PROFILE, R.string.tab_profile, Icons.Rounded.AccountCircle),
    )
    NavigationBar(containerColor = MaterialTheme.colorScheme.surface) {
        items.forEach { item ->
            NavigationBarItem(
                selected = selectedTab == item.tab,
                onClick = { onTabSelected(item.tab) },
                icon = {
                    Icon(
                        imageVector = item.icon,
                        contentDescription = stringResource(item.labelRes),
                    )
                },
                label = { Text(stringResource(item.labelRes)) },
            )
        }
    }
}
