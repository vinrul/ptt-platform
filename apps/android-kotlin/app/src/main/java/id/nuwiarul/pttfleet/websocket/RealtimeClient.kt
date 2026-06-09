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
import java.util.concurrent.Executors
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
    fun onGroupJoined(groupId: String)
    fun onPttGranted(sessionId: String, groupId: String)
    fun onPttBusy(groupId: String, speakerUserId: String)
    fun onPttStarted(sessionId: String, groupId: String, speakerUserId: String)
    fun onPttStopped(sessionId: String, groupId: String, reason: String)
    fun onAudioFrame(sessionId: String, sequence: Long, payload: ByteArray)
}

class RealtimeClient(
    private val httpClient: OkHttpClient,
    private val listener: RealtimeListener,
) {
    private val mainHandler = Handler(Looper.getMainLooper())
    private val scheduler = Executors.newSingleThreadScheduledExecutor()
    private var socket: WebSocket? = null
    private var reconnectFuture: ScheduledFuture<*>? = null
    private var heartbeatFuture: ScheduledFuture<*>? = null
    private var session: AuthSession? = null
    private var reconnectAttempt = 0
    private var stopped = true

    @Synchronized
    fun connect(session: AuthSession) {
        this.session = session
        stopped = false
        openSocket()
    }

    @Synchronized
    fun disconnect() {
        stopped = true
        reconnectFuture?.cancel(true)
        heartbeatFuture?.cancel(true)
        reconnectFuture = null
        heartbeatFuture = null
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
    fun startPtt(groupId: String): Boolean = sendJson(
        "ptt.start",
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

    private fun handleMessage(text: String) {
        runCatching {
            val event = JSONObject(text)
            when (event.getString("type")) {
                "connection.ready" -> {
                    val connectionId = event.getJSONObject("payload").getString("connectionId")
                    mainHandler.post { listener.onReady(connectionId) }
                }
                "group.joined" -> {
                    val groupId = event.getJSONObject("payload").getString("groupId")
                    mainHandler.post { listener.onGroupJoined(groupId) }
                }
                "ptt.granted" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listener.onPttGranted(payload.getString("sessionId"), payload.getString("groupId"))
                    }
                }
                "ptt.busy" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listener.onPttBusy(payload.getString("groupId"), payload.getString("speakerUserId"))
                    }
                }
                "ptt.started" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listener.onPttStarted(
                            payload.getString("sessionId"),
                            payload.getString("groupId"),
                            payload.getString("speakerUserId"),
                        )
                    }
                }
                "ptt.stopped" -> {
                    val payload = event.getJSONObject("payload")
                    mainHandler.post {
                        listener.onPttStopped(
                            payload.getString("sessionId"),
                            payload.getString("groupId"),
                            payload.getString("reason"),
                        )
                    }
                }
                "error" -> {
                    val message = event.getJSONObject("payload").optString("message", "Realtime error")
                    mainHandler.post { listener.onError(message) }
                }
            }
        }.onFailure {
            mainHandler.post { listener.onError("Received malformed realtime event") }
        }
    }

    private fun postStatus(status: ConnectionStatus) {
        mainHandler.post { listener.onStatusChanged(status) }
    }

    private inner class SocketListener : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            synchronized(this@RealtimeClient) {
                socket = webSocket
                reconnectAttempt = 0
                startHeartbeat()
            }
            postStatus(ConnectionStatus.CONNECTED)
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            handleMessage(text)
        }

        override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
            val frame = AudioEnvelope.decodeDownlink(bytes.toByteArray()) ?: return
            mainHandler.post {
                listener.onAudioFrame(frame.sessionId, frame.sequence, frame.payload)
            }
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            webSocket.close(code, reason)
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            synchronized(this@RealtimeClient) {
                if (socket === webSocket) socket = null
                heartbeatFuture?.cancel(false)
                heartbeatFuture = null
                scheduleReconnect()
            }
        }

        override fun onFailure(webSocket: WebSocket, error: Throwable, response: Response?) {
            synchronized(this@RealtimeClient) {
                if (socket === webSocket) socket = null
                heartbeatFuture?.cancel(false)
                heartbeatFuture = null
                scheduleReconnect()
            }
            mainHandler.post { listener.onError(error.message ?: "Realtime connection failed") }
        }
    }
}
