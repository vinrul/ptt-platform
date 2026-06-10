package id.nuwiarul.pttfleet.fcm

import android.content.Context
import android.content.Intent
import android.util.Log
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import id.nuwiarul.pttfleet.PatrolService
import id.nuwiarul.pttfleet.auth.EndpointNormalizer
import id.nuwiarul.pttfleet.auth.SecureTokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

class MyFirebaseMessagingService : FirebaseMessagingService() {

    override fun onNewToken(token: String) {
        super.onNewToken(token)
        Log.d(TAG, "New FCM Token: $token")

        // Save the token to local SharedPreferences
        val prefs = getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        prefs.edit().putString(KEY_FCM_TOKEN, token).apply()

        // Upload token to server if user is logged in
        uploadToken(applicationContext, token)
    }

    override fun onMessageReceived(remoteMessage: RemoteMessage) {
        super.onMessageReceived(remoteMessage)
        Log.d(TAG, "Received FCM Message: ${remoteMessage.data}")

        val type = remoteMessage.data["type"]
        if (type == "ptt_wakeup") {
            val groupId = remoteMessage.data["groupId"]
            Log.d(TAG, "PTT Wakeup notification received for group: $groupId")

            // Wake the patrol connection, join the target group, and send one fresh GPS update.
            PatrolService.wakeup(applicationContext, groupId)

            // Launch MainActivity to bring the app to the foreground immediately
            runCatching {
                val startAppIntent = Intent(applicationContext, id.nuwiarul.pttfleet.MainActivity::class.java).apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
                    putExtra("is_wakeup", true)
                }
                applicationContext.startActivity(startAppIntent)
            }
        }
    }

    companion object {
        private const val TAG = "FCM_Service"
        const val PREFS_NAME = "fcm_prefs"
        const val KEY_FCM_TOKEN = "fcm_token"

        fun uploadToken(context: Context, token: String) {
            val session = SecureTokenStore(context).load() ?: return
            if (session.deviceId.isBlank()) {
                Log.w(TAG, "Cannot upload FCM token: deviceId is empty.")
                return
            }

            val client = OkHttpClient()
            val requestBody = JSONObject()
                .put("pushToken", token)
                .toString()
                .toRequestBody("application/json; charset=utf-8".toMediaType())

            val url = "${EndpointNormalizer.serverUrl(session.serverUrl)}/api/devices/${session.deviceId}/push-token"
            val request = Request.Builder()
                .url(url)
                .put(requestBody)
                .header("Authorization", "Bearer ${session.accessToken}")
                .build()

            CoroutineScope(Dispatchers.IO).launch {
                runCatching {
                    client.newCall(request).execute().use { response ->
                        if (!response.isSuccessful) {
                            Log.e(TAG, "Failed to upload FCM token: HTTP ${response.code} - ${response.body?.string()}")
                        } else {
                            Log.d(TAG, "FCM token uploaded successfully to server.")
                        }
                    }
                }.onFailure {
                    Log.e(TAG, "Error uploading FCM token: ${it.message}", it)
                }
            }
        }
    }
}
