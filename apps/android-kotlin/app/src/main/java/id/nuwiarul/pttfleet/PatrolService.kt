package id.nuwiarul.pttfleet

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.app.AlarmManager
import android.util.Log
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.os.IBinder
import android.os.PowerManager
import androidx.core.app.NotificationCompat
import androidx.core.content.ContextCompat
import id.nuwiarul.pttfleet.audio.PttAudioEngine
import id.nuwiarul.pttfleet.audio.AudioRouting
import id.nuwiarul.pttfleet.auth.SecureTokenStore
import id.nuwiarul.pttfleet.location.GpsSample
import id.nuwiarul.pttfleet.location.LocationTracker
import id.nuwiarul.pttfleet.location.TrackingPreferenceStore
import android.net.wifi.WifiManager
import id.nuwiarul.pttfleet.websocket.ConnectionStatus
import id.nuwiarul.pttfleet.websocket.RealtimeClient
import id.nuwiarul.pttfleet.websocket.RealtimeListener


class PatrolService : Service(), RealtimeListener {
    private lateinit var audioEngine: PttAudioEngine
    private val realtimeClient = RealtimeClient.getInstance()
    private var selectedGroupId: String? = null
    private var ownUserId: String? = null
    private val localSpeakerSessions = mutableSetOf<String>()
    private var wakeLock: PowerManager.WakeLock? = null
    private var wifiLock: WifiManager.WifiLock? = null
    private var locationTracker: LocationTracker? = null
    private var pendingWakeLocation = false

    override fun onCreate() {
        super.onCreate()
        isTrackingActive = TrackingPreferenceStore(applicationContext).isEnabled()
        createNotificationChannel()
        audioEngine = PttAudioEngine(
            onEncodedFrame = {},
            onPlayedFrame = { samples ->
                updateNotification(getString(R.string.background_playing, samples))
            },
            onError = { message -> updateNotification(message) },
        )
        locationTracker = LocationTracker(
            applicationContext,
            onLocation = { sample ->
                latestLocation = sample
                realtimeClient.sendGps(sample)
                onLocationChangedListener?.invoke(sample)
            },
            onError = { message -> updateNotification(message) }
        )
        wakeLock = (getSystemService(POWER_SERVICE) as PowerManager)
            .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "$packageName:patrol")
            .apply {
                setReferenceCounted(false)
                acquire()
            }
        val wifiManager = applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        wifiLock = wifiManager.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "PttFleet:wifi_lock").apply {
            setReferenceCounted(false)
            acquire()
        }
        realtimeClient.addListener(this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            setEnabled(this, false)
            stopSelf()
            return START_NOT_STICKY
        }
        if (intent?.action == ACTION_START_TRACKING) {
            isTrackingActive = true
            TrackingPreferenceStore(applicationContext).setEnabled(true)
            checkAndStartLocationTracking()
        } else if (intent?.action == ACTION_STOP_TRACKING) {
            isTrackingActive = false
            TrackingPreferenceStore(applicationContext).setEnabled(false)
            locationTracker?.stop()
        }
        if (intent?.action == ACTION_WAKEUP) {
            pendingWakeLocation = true
            intent.getStringExtra(EXTRA_GROUP_ID)?.takeIf { it.isNotBlank() }?.let { groupId ->
                selectedGroupId = groupId
                getSharedPreferences(PREFERENCES_NAME, MODE_PRIVATE)
                    .edit()
                    .putString(KEY_GROUP_ID, groupId)
                    .apply()
            }
            wakeScreenBriefly()
        }
        if (intent?.action == ACTION_JOIN_GROUP) {
            selectedGroupId = intent.getStringExtra(EXTRA_GROUP_ID)
            selectedGroupId?.let { groupId ->
                getSharedPreferences(PREFERENCES_NAME, MODE_PRIVATE)
                    .edit()
                    .putString(KEY_GROUP_ID, groupId)
                    .apply()
            }
            selectedGroupId?.let { realtimeClient.joinGroup(it) }
        }

        startPatrolForeground(getString(R.string.background_connecting))
        setEnabled(this, true)
        val forceConnect = intent?.action == ACTION_JOIN_GROUP || intent?.action == ACTION_WAKEUP
        ensureConnected(forceConnect)
        checkAndStartLocationTracking()
        return START_STICKY
    }

    private fun checkAndStartLocationTracking() {
        if (!isTrackingActive) return
        val hasFine = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.ACCESS_FINE_LOCATION,
        ) == PackageManager.PERMISSION_GRANTED
        val hasCoarse = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.ACCESS_COARSE_LOCATION,
        ) == PackageManager.PERMISSION_GRANTED
        if (hasFine || hasCoarse) {
            locationTracker?.start()
        }
    }

    private fun startPatrolForeground(status: String) {
        val notification = buildNotification(status)
        if (android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification)
            return
        }

        var serviceType = ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            serviceType = serviceType or ServiceInfo.FOREGROUND_SERVICE_TYPE_REMOTE_MESSAGING
        }
        if (
            (isTrackingActive || pendingWakeLocation) &&
            hasLocationPermission() &&
            (!pendingWakeLocation || hasBackgroundLocationPermission())
        ) {
            serviceType = serviceType or ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION
        }
        startForeground(NOTIFICATION_ID, notification, serviceType)
    }

    private fun hasLocationPermission(): Boolean =
        ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_FINE_LOCATION) ==
            PackageManager.PERMISSION_GRANTED ||
            ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_COARSE_LOCATION) ==
            PackageManager.PERMISSION_GRANTED

    private fun hasBackgroundLocationPermission(): Boolean =
        android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.Q ||
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            ) == PackageManager.PERMISSION_GRANTED

    override fun onDestroy() {
        AudioRouting.setSpeakerphoneOn(applicationContext, false)
        realtimeClient.removeListener(this)
        realtimeClient.disconnect()
        audioEngine.release()
        locationTracker?.stop()
        locationTracker = null
        if (wakeLock?.isHeld == true) wakeLock?.release()
        wakeLock = null
        if (wifiLock?.isHeld == true) wifiLock?.release()
        wifiLock = null
        isTrackingActive = false
        latestLocation = null
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onTaskRemoved(rootIntent: Intent?) {
        val session = SecureTokenStore(applicationContext).load()
        if (session != null && isEnabled(applicationContext)) {
            Log.d("PatrolService", "Task removed, scheduling service restart...")
            val restartIntent = Intent(applicationContext, PatrolService::class.java).apply {
                setAction(ACTION_WAKEUP)
            }
            val pendingIntent = PendingIntent.getService(
                this,
                99,
                restartIntent,
                PendingIntent.FLAG_ONE_SHOT or PendingIntent.FLAG_IMMUTABLE
            )
            val alarmManager = getSystemService(Context.ALARM_SERVICE) as AlarmManager
            runCatching {
                alarmManager.set(
                    AlarmManager.RTC_WAKEUP,
                    System.currentTimeMillis() + 1000,
                    pendingIntent
                )
            }
        }
        super.onTaskRemoved(rootIntent)
    }

    override fun onStatusChanged(status: ConnectionStatus) {
        updateNotification(
            when (status) {
                ConnectionStatus.IDLE -> getString(R.string.background_offline)
                ConnectionStatus.CONNECTING -> getString(R.string.background_connecting)
                ConnectionStatus.CONNECTED -> getString(R.string.background_connected)
                ConnectionStatus.RECONNECTING -> getString(R.string.background_reconnecting)
            },
        )
    }

    override fun onReady(connectionId: String) {
        selectedGroupId?.let { realtimeClient.joinGroup(it) }
        sendWakeLocationIfPending()
    }

    override fun onError(message: String) = updateNotification(message)

    override fun onGroupJoined(groupId: String, onlineUserIds: Set<String>) {
        updateNotification(getString(R.string.background_channel_ready))
    }

    override fun onPresenceUpdated(userId: String, status: String) = Unit

    override fun onPttGranted(sessionId: String, groupId: String) = Unit

    override fun onPttBusy(groupId: String, speakerUserId: String, queuePosition: Int?) = Unit

    override fun onPttStarted(sessionId: String, groupId: String, speakerUserId: String) {
        if (speakerUserId == ownUserId) {
            localSpeakerSessions += sessionId
            return
        }
        audioEngine.stopPlayback()
        updateNotification(getString(R.string.background_receiving))
        AudioRouting.setSpeakerphoneOn(applicationContext, true)
    }

    override fun onPttStopped(sessionId: String, groupId: String, reason: String) {
        localSpeakerSessions -= sessionId
        audioEngine.stopPlayback()
        updateNotification(getString(R.string.background_channel_ready))
        AudioRouting.setSpeakerphoneOn(applicationContext, false)
    }

    override fun onAudioFrame(sessionId: String, sequence: Long, payload: ByteArray) {
        if (sessionId in localSpeakerSessions) return
        if (MainActivity.isVisible) return
        AudioRouting.setSpeakerphoneOn(applicationContext, true)
        audioEngine.play(sessionId, sequence, payload)
    }

    private fun ensureConnected(force: Boolean = false) {
        val session = SecureTokenStore(applicationContext).load()
        if (session == null) {
            stopSelf()
            return
        }
        ownUserId = session.user.id
        selectedGroupId = getSharedPreferences(PREFERENCES_NAME, MODE_PRIVATE)
            .getString(KEY_GROUP_ID, selectedGroupId)
            
        if (force || realtimeClient.status == ConnectionStatus.IDLE) {
            realtimeClient.connect(session, force)
        }
    }

    private fun sendWakeLocationIfPending() {
        if (!pendingWakeLocation) return
        if (!hasLocationPermission() || !hasBackgroundLocationPermission()) {
            pendingWakeLocation = false
            updateNotification(getString(R.string.background_location_permission_required))
            return
        }
        pendingWakeLocation = false
        locationTracker?.requestCurrentLocation { sample ->
            latestLocation = sample
            realtimeClient.sendGps(sample)
            onLocationChangedListener?.invoke(sample)
            updateNotification(getString(R.string.background_location_sent))
        }
    }

    @Suppress("DEPRECATION")
    private fun wakeScreenBriefly() {
        val powerManager = getSystemService(POWER_SERVICE) as PowerManager
        powerManager.newWakeLock(
            PowerManager.SCREEN_BRIGHT_WAKE_LOCK or
                PowerManager.ACQUIRE_CAUSES_WAKEUP or
                PowerManager.ON_AFTER_RELEASE,
            "$packageName:fcm-screen",
        ).apply {
            setReferenceCounted(false)
            acquire(SCREEN_WAKE_DURATION_MS)
        }
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            NOTIFICATION_CHANNEL_ID,
            getString(R.string.background_channel_name),
            NotificationManager.IMPORTANCE_LOW,
        )
        channel.description = getString(R.string.background_channel_description)
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun buildNotification(status: String): Notification {
        val activityIntent = Intent(this, MainActivity::class.java).apply {
            putExtra("is_wakeup", true)
        }
        val openIntent = PendingIntent.getActivity(
            this,
            0,
            activityIntent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val stopIntent = PendingIntent.getService(
            this,
            1,
            Intent(this, PatrolService::class.java).setAction(ACTION_STOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        
        val builder = NotificationCompat.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_launcher)
            .setContentTitle(getString(R.string.background_notification_title))
            .setContentText(status)
            .setContentIntent(openIntent)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .addAction(0, getString(R.string.background_stop), stopIntent)

        // Use full screen intent for PTT voice receiving/playing to bring MainActivity to foreground
        if (status == getString(R.string.background_receiving) || status.contains("playing") || status.contains("Playing")) {
            builder.setPriority(NotificationCompat.PRIORITY_HIGH)
                .setCategory(NotificationCompat.CATEGORY_CALL)
                .setFullScreenIntent(openIntent, true)
        }
        
        return builder.build()
    }

    private fun updateNotification(status: String) {
        getSystemService(NotificationManager::class.java)
            .notify(NOTIFICATION_ID, buildNotification(status))
    }

    companion object {
        private const val NOTIFICATION_CHANNEL_ID = "patrol_connection"
        private const val NOTIFICATION_ID = 1001
        private const val ACTION_WAKEUP = "id.nuwiarul.pttfleet.action.WAKEUP"
        private const val ACTION_JOIN_GROUP = "id.nuwiarul.pttfleet.action.JOIN_GROUP"
        private const val ACTION_STOP = "id.nuwiarul.pttfleet.action.STOP_PATROL"
        private const val ACTION_START_TRACKING = "id.nuwiarul.pttfleet.action.START_TRACKING"
        private const val ACTION_STOP_TRACKING = "id.nuwiarul.pttfleet.action.STOP_TRACKING"
        private const val EXTRA_GROUP_ID = "group_id"
        private const val PREFERENCES_NAME = "patrol_service"
        private const val KEY_GROUP_ID = "selected_group_id"
        private const val KEY_ENABLED = "enabled"
        private const val SCREEN_WAKE_DURATION_MS = 8_000L

        var isTrackingActive = false
            internal set

        var latestLocation: GpsSample? = null
            internal set

        var onLocationChangedListener: ((GpsSample) -> Unit)? = null

        fun start(context: Context) {
            setEnabled(context, true)
            context.startForegroundService(Intent(context, PatrolService::class.java))
        }

        fun wakeup(context: Context, groupId: String? = null) {
            context.startForegroundService(
                Intent(context, PatrolService::class.java)
                    .setAction(ACTION_WAKEUP)
                    .apply {
                        if (!groupId.isNullOrBlank()) putExtra(EXTRA_GROUP_ID, groupId)
                    },
            )
        }

        fun joinGroup(context: Context, groupId: String) {
            context.startForegroundService(
                Intent(context, PatrolService::class.java)
                    .setAction(ACTION_JOIN_GROUP)
                    .putExtra(EXTRA_GROUP_ID, groupId),
            )
        }

        fun startTracking(context: Context) {
            TrackingPreferenceStore(context.applicationContext).setEnabled(true)
            isTrackingActive = true
            context.startForegroundService(
                Intent(context, PatrolService::class.java).setAction(ACTION_START_TRACKING)
            )
        }

        fun stopTracking(context: Context) {
            TrackingPreferenceStore(context.applicationContext).setEnabled(false)
            isTrackingActive = false
            context.startForegroundService(
                Intent(context, PatrolService::class.java).setAction(ACTION_STOP_TRACKING)
            )
        }

        fun isTrackingEnabled(context: Context): Boolean =
            TrackingPreferenceStore(context.applicationContext).isEnabled()

        fun stop(context: Context) {
            context.startForegroundService(
                Intent(context, PatrolService::class.java).setAction(ACTION_STOP),
            )
        }

        fun isEnabled(context: Context): Boolean =
            context.getSharedPreferences(PREFERENCES_NAME, MODE_PRIVATE)
                .getBoolean(KEY_ENABLED, false)

        private fun setEnabled(context: Context, enabled: Boolean) {
            context.getSharedPreferences(PREFERENCES_NAME, MODE_PRIVATE)
                .edit()
                .putBoolean(KEY_ENABLED, enabled)
                .apply()
        }

    }
}
