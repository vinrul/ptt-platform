package id.nuwiarul.pttfleet.auth

data class AuthUser(
    val id: String,
    val username: String,
    val fullName: String,
    val role: String,
    val status: String,
)

data class AuthSession(
    val serverUrl: String,
    val accessToken: String,
    val refreshToken: String,
    val user: AuthUser,
)
