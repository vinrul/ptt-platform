package id.nuwiarul.pttfleet.auth

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.core.content.edit
import org.json.JSONObject
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

class SecureTokenStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun save(session: AuthSession) {
        val payload = JSONObject()
            .put("serverUrl", session.serverUrl)
            .put("accessToken", session.accessToken)
            .put("refreshToken", session.refreshToken)
            .put(
                "user",
                JSONObject()
                    .put("id", session.user.id)
                    .put("username", session.user.username)
                    .put("fullName", session.user.fullName)
                    .put("role", session.user.role)
                    .put("status", session.user.status),
            )
            .toString()
            .toByteArray(Charsets.UTF_8)

        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, secretKey())

        preferences.edit {
            putString(KEY_PAYLOAD, Base64.encodeToString(cipher.doFinal(payload), Base64.NO_WRAP))
            putString(KEY_IV, Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
        }
    }

    fun load(): AuthSession? {
        val encryptedPayload = preferences.getString(KEY_PAYLOAD, null) ?: return null
        val encodedIv = preferences.getString(KEY_IV, null) ?: return null

        return runCatching {
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(
                Cipher.DECRYPT_MODE,
                secretKey(),
                GCMParameterSpec(128, Base64.decode(encodedIv, Base64.NO_WRAP)),
            )
            val json = JSONObject(
                String(
                    cipher.doFinal(Base64.decode(encryptedPayload, Base64.NO_WRAP)),
                    Charsets.UTF_8,
                ),
            )
            val user = json.getJSONObject("user")
            AuthSession(
                serverUrl = json.getString("serverUrl"),
                accessToken = json.getString("accessToken"),
                refreshToken = json.getString("refreshToken"),
                user = AuthUser(
                    id = user.getString("id"),
                    username = user.getString("username"),
                    fullName = user.getString("fullName"),
                    role = user.getString("role"),
                    status = user.getString("status"),
                ),
            )
        }.getOrElse {
            clear()
            null
        }
    }

    fun clear() {
        preferences.edit { clear() }
    }

    private fun secretKey(): SecretKey {
        val keyStore = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        (keyStore.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }

        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setRandomizedEncryptionRequired(true)
                .build(),
        )
        return generator.generateKey()
    }

    companion object {
        private const val ANDROID_KEYSTORE = "AndroidKeyStore"
        private const val KEY_ALIAS = "ptt_fleet_session_key"
        private const val TRANSFORMATION = "AES/GCM/NoPadding"
        private const val PREFERENCES_NAME = "secure_session"
        private const val KEY_PAYLOAD = "payload"
        private const val KEY_IV = "iv"
    }
}
