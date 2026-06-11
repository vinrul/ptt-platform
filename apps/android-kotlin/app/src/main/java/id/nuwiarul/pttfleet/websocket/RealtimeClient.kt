package id.nuwiarul.pttfleet.websocket

import android.os.Handler
import android.os.Looper
import id.nuwiarul.pttfleet.auth.AuthSession
import id.nuwiarul.pttfleet.auth.EndpointNormalizer
import id.nuwiarul.pttfleet.location.GpsSample
import id.nuwiarul.pttfleet.audio.AudioEnvelope
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import org.json.JSONObject
import java.time.Instant
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

enum class ConnectionStatus {
    IDLE,
    CONNECTING,
    CONNECTED,
    RECONNECTING,
}

interface RealtimeListener {
    fun onStatusChanged(status: ConnectionStatus)
    fun onReady(connectionId: String)
    fun onError(message: String)
    fun onGroupJoined(groupId: String, onlineUserIds: Set<String>)
    fun onPresenceUpdated(userId: String, status: String)
    fun onPttGranted(sessionId: String, groupId: String)
    fun onPttBusy(groupId: String, speakerUserId: String, queuePosition: Int?)
    fun onPttStarted(sessionId: String, groupId: String, speakerUserId: String)
    fun onPttStopped(sessionId: String, groupId: String, reason: String)
    fun onAudioFrame(sessionId: String, sequence: Long, payload: ByteArray)
}

class RealtimeClient private constructor() {
    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .build()

    private val mainHandler = Handler(Looper.getMainLooper())
    private var scheduler: ScheduledExecutorService = Executors.newSingleThreadScheduledExecutor()
    private var socket: WebSocket? = null
    private var reconnectFuture: ScheduledFuture<*>? = null
    private var heartbeatFuture: ScheduledFuture<*>? = null
    private var readyTimeoutFuture: ScheduledFuture<*>? = null
    private var session: AuthSession? = null
    private var reconnectAttempt = 0
    private var stopped = true

    private val listeners = CopyOnWriteArrayList<RealtimeListener>()

    var status: ConnectionStatus = ConnectionStatus.IDLE
        private set

    companion object {
        private const val READY_TIMEOUT_SECONDS = 15L

        @Volatile
        private var instance: RealtimeClient? = null

        fun getInstance(): RealtimeClient {
            return instance ?: synchronized(this) {
                instance ?: RealtimeClient().also { instance = it }
            }
        }
    }

    fun addListener(listener: RealtimeListener) {
        if (!listeners.contains(listener)) {
            listeners.add(listener)
        }
        listener.onStatusChanged(status)
    }

    fun removeListener(listener: RealtimeListener) {
        listeners.remove(listener)
    }

    @Synchronized
    fun connect(session: AuthSession, force: Boolean = false) {
        this.session = session
        stopped = false
        if (scheduler.isShutdown) {
            scheduler = Executors.newSingleThreadScheduledExecutor()
        }

        if (force || socket == null) {
            reconnectFuture?.cancel(false)
            reconnectFuture = null

            if (force) {
                socket?.close(1000, "Forced reconnect")
                socket = null
                reconnectAttempt = 0
            }

            openSocket()
        }
    }

    @Synchronized
    fun disconnect() {
        stopped = true
        reconnectFuture?.cancel(true)
        heartbeatFuture?.cancel(true)
        readyTimeoutFuture?.cancel(true)
        reconnectFuture = null
        heartbeatFuture = null
        readyTimeoutFuture = null
        socket?.close(1000, "android logout")
        socket = null
        session = null
        postStatus(ConnectionStatus.IDLE)
        scheduler.shutdownNow()
    }

    @Synchronized
    fun sendGps(sample: GpsSample): Boolean {
        val payload = JSONObject()
            .put("lat", sample.lat)
            .put("lng", sample.lng)
        sample.speed?.let { payload.put("speed", it) }
        sample.heading?.let { payload.put("heading", it) }
        sample.accuracy?.let { payload.put("accuracy", it) }

        val event = JSONObject()
            .put("type", "gps.update")
            .put("timestamp", Instant.now().toString())
            .put("payload", payload)
        return socket?.send(event.toString()) == true
    }

    @Synchronized
    fun sendSos(sample: GpsSample?, message: String = "Emergency"): Boolean {
        val payload = JSONObject().put("message", message)
        sample?.let {
            payload.put("lat", it.lat)
            payload.put("lng", it.lng)
        }

        val event = JSONObject()
            .put("type", "sos.create")
            .put("requestId", "android-${System.currentTimeMillis()}")
            .put("timestamp", Instant.now().toString())
            .put("payload", payload)
        return socket?.send(event.toString()) == true
    }

    @Synchronized
    fun joinGroup(groupId: String): Boolean = sendJson(
        "group.join",
        JSONObject().put("groupId", groupId),
    )

    @Synchronized
    fun startPtt(groupId: String, targetUserId: String? = null): Boolean {
        val payload = JSONObject()
            .put("groupId", groupId)
            .put("queue", true)
        targetUserId?.let { payload.put("targetUserId", it) }
        return sendJson("ptt.start", payload)
    }

    @Synchronized
    fun cancelPtt(groupId: String): Boolean = sendJson(
        "ptt.cancel",
        JSONObject().put("groupId", groupId),
    )

    @Synchronized
    fun stopPtt(sessionId: String): Boolean = sendJson(
        "ptt.stop",
        JSONObject().put("sessionId", sessionId),
    )

    @Synchronized
    fun sendAudio(sessionId: String, sequence: Long, opusPayload: ByteArray): Boolean {
        return socket?.send(ByteString.of(*AudioEnvelope.encodeUplink(sessionId, sequence, opusPayload))) == true
    }

    private fun sendJson(type: String, payload: JSONObject): Boolean {
        val event = JSONObject()
            .put("type", type)
            .put("requestId", "android-${System.currentTimeMillis()}")
            .put("timestamp", Instant.now().toString())
            .put("payload", payload)
        return socket?.send(event.toString()) == true
    }

    @Synchronized
    private fun openSocket() {
        val activeSession = session ?: return
        if (stopped || socket != null) return

        postStatus(if (reconnectAttempt == 0) ConnectionStatus.CONNECTING else ConnectionStatus.RECONNECTING)
        val request = Request.Builder()
            .url(EndpointNormalizer.websocketUrl(activeSession.serverUrl, activeSession.accessToken))
            .build()
        socket = httpClient.newWebSocket(request, SocketListener())
    }

    @Synchronized
    private fun scheduleReconnect() {
        if (stopped || scheduler.isShutdown) return
        reconnectAttempt += 1
        postStatus(ConnectionStatus.RECONNECTING)
        reconnectFuture?.cancel(false)
        reconnectFuture = scheduler.schedule(
            { synchronized(this) { openSocket() } },
            BackoffPolicy.delayMillis(reconnectAttempt),
            TimeUnit.MILLISECONDS,
        )
    }

    @Synchronized
    private fun startHeartbeat() {
        heartbeatFuture?.cancel(false)
        heartbeatFuture = scheduler.scheduleWithFixedDelay(
            {
                val event = JSONObject()
                    .put("type", "heartbeat")
                    .put("timestamp", Instant.now().toString())
                    .put("payload", JSONObject())
                socket?.send(event.toString())
            },
            25,
            25,
            TimeUnit.SECONDS,
        )
    }

    private fun handleMessage(webSocket: WebSocket, text: String) {
        runCatching {
            val event = JSONObject(text)
            when (event.getString("type")) {
                "connection.ready" -> {
                    if (!markReady(webSocket)) return
                    val connectionId = event.getJSONObject("payload").getString("connectionId")
                    mainHandler.post {
                        listeners.forEach { it.onReady(connectionId) }
                    }
                }
                "group.joined" -> {
                    val payload = event.getJSONObject("payload")
                    val groupId = payload.getString("groupId")
                    val onlineItems = payload.optJSONArray("onlineUserIds")
                    val onlineUserIds = buildSet {
                        if (onlineItems != null) {
                            for (index in 0 until onlineItems.length()) {
                                add(onlineItems.getString(index))
                            }
                        }
                    }
                    mainHandler.post {
                        listeners.forEach { it.onGroupJoined(groupId, onlineUserIds) }
                    }
                }
                "presence.updated" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listeners.forEach {
                            it.onPresenceUpdated(
                                payload.getString("userId"),
                                payload.getString("status"),
                            )
                        }
                    }
                }
                "ptt.granted" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listeners.forEach {
                            it.onPttGranted(payload.getString("sessionId"), payload.getString("groupId"))
                        }
                    }
                }
                "ptt.busy" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listeners.forEach {
                            it.onPttBusy(
                                payload.getString("groupId"),
                                payload.getString("speakerUserId"),
                                payload.optInt("queuePosition").takeIf { payload.has("queuePosition") && !payload.isNull("queuePosition") },
                            )
                        }
                    }
                }
                "ptt.started" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listeners.forEach {
                            it.onPttStarted(
                                payload.getString("sessionId"),
                                payload.getString("groupId"),
                                payload.getString("speakerUserId"),
                            )
                        }
                    }
                }
                "ptt.stopped" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listeners.forEach {
                            it.onPttStopped(
                                payload.getString("sessionId"),
                                payload.getString("groupId"),
                                payload.getString("reason"),
                            )
                        }
                    }
                }
                "error" -> {
                    val message = event.getJSONObject("payload").optString("message", "Realtime error")
                    mainHandler.post {
                        listeners.forEach { it.onError(message) }
                    }
                }
            }
        }.onFailure {
            mainHandler.post {
                listeners.forEach { it.onError("Received malformed realtime event") }
            }
        }
    }

    private fun postStatus(newStatus: ConnectionStatus) {
        this.status = newStatus
        mainHandler.post {
            listeners.forEach { it.onStatusChanged(newStatus) }
        }
    }

    @Synchronized
    private fun markReady(webSocket: WebSocket): Boolean {
        if (socket !== webSocket || stopped) return false
        readyTimeoutFuture?.cancel(false)
        readyTimeoutFuture = null
        reconnectAttempt = 0
        startHeartbeat()
        postStatus(ConnectionStatus.CONNECTED)
        return true
    }

    @Synchronized
    private fun scheduleReadyTimeout(webSocket: WebSocket) {
        readyTimeoutFuture?.cancel(false)
        readyTimeoutFuture = scheduler.schedule(
            {
                var timedOut = false
                synchronized(this@RealtimeClient) {
                    if (socket === webSocket && status != ConnectionStatus.CONNECTED) {
                        socket = null
                        timedOut = true
                        webSocket.cancel()
                        scheduleReconnect()
                    }
                }
                if (timedOut) {
                    mainHandler.post {
                        listeners.forEach {
                            it.onError("Realtime handshake timed out; retrying")
                        }
                    }
                }
            },
            READY_TIMEOUT_SECONDS,
            TimeUnit.SECONDS,
        )
    }

    private inner class SocketListener : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            synchronized(this@RealtimeClient) {
                if (socket !== webSocket || stopped) {
                    webSocket.cancel()
                    return
                }
                scheduleReadyTimeout(webSocket)
            }
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            handleMessage(webSocket, text)
        }

        override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
            val frame = AudioEnvelope.decodeDownlink(bytes.toByteArray()) ?: return
            mainHandler.post {
                listeners.forEach {
                    it.onAudioFrame(frame.sessionId, frame.sequence, frame.payload)
                }
            }
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            webSocket.close(code, reason)
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            synchronized(this@RealtimeClient) {
                if (socket !== webSocket) return
                socket = null
                heartbeatFuture?.cancel(false)
                heartbeatFuture = null
                readyTimeoutFuture?.cancel(false)
                readyTimeoutFuture = null
                scheduleReconnect()
            }
        }

        override fun onFailure(webSocket: WebSocket, error: Throwable, response: Response?) {
            synchronized(this@RealtimeClient) {
                if (socket !== webSocket) return
                socket = null
                heartbeatFuture?.cancel(false)
                heartbeatFuture = null
                readyTimeoutFuture?.cancel(false)
                readyTimeoutFuture = null
                scheduleReconnect()
            }
            mainHandler.post {
                listeners.forEach { it.onError(error.message ?: "Realtime connection failed") }
            }
        }
    }
}
