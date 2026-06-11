package id.nuwiarul.pttfleet.auth

import android.content.Context
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import okhttp3.OkHttpClient
import java.util.concurrent.TimeUnit

class SessionManager private constructor(context: Context) {
    private val tokenStore = SecureTokenStore(context.applicationContext)
    private val authRepository = AuthRepository(
        OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(20, TimeUnit.SECONDS)
            .writeTimeout(15, TimeUnit.SECONDS)
            .callTimeout(30, TimeUnit.SECONDS)
            .retryOnConnectionFailure(true)
            .build(),
    )
    private val refreshMutex = Mutex()

    fun currentSession(): AuthSession? = tokenStore.load()

    fun save(session: AuthSession) = tokenStore.save(session)

    fun clear() = tokenStore.clear()

    suspend fun validSession(forceRefresh: Boolean = false): AuthSession? {
        val current = tokenStore.load() ?: return null
        if (!forceRefresh && !AccessToken.needsRefresh(current.accessToken)) {
            return current
        }

        return refreshMutex.withLock {
            val latest = tokenStore.load() ?: return@withLock null
            if (!forceRefresh && !AccessToken.needsRefresh(latest.accessToken)) {
                return@withLock latest
            }

            try {
                authRepository.refresh(latest).also(tokenStore::save)
            } catch (error: AuthRefreshException) {
                if (error.sessionInvalid) {
                    tokenStore.clear()
                }
                throw error
            }
        }
    }

    companion object {
        @Volatile
        private var instance: SessionManager? = null

        fun getInstance(context: Context): SessionManager =
            instance ?: synchronized(this) {
                instance ?: SessionManager(context.applicationContext).also { instance = it }
            }
    }
}
