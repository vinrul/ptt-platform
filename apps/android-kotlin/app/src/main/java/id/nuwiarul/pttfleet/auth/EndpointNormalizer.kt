package id.nuwiarul.pttfleet.auth

import java.net.URI

object EndpointNormalizer {
    fun serverUrl(value: String): String {
        val trimmed = value.trim().trimEnd('/')
        require(trimmed.isNotEmpty()) { "Server URL is required" }

        val withScheme = if ("://" in trimmed) trimmed else "http://$trimmed"
        val uri = URI(withScheme)
        require(uri.scheme == "http" || uri.scheme == "https") {
            "Server URL must use http or https"
        }
        require(!uri.host.isNullOrBlank()) { "Server URL host is invalid" }
        require(uri.rawQuery == null && uri.rawFragment == null) {
            "Server URL must not contain query or fragment"
        }
        return withScheme.trimEnd('/')
    }

    fun loginUrl(serverUrl: String): String = "${serverUrl(serverUrl)}/api/auth/login"

    fun websocketUrl(serverUrl: String, accessToken: String): String {
        val normalized = serverUrl(serverUrl)
        val websocketBase = when {
            normalized.startsWith("https://") -> "wss://${normalized.removePrefix("https://")}"
            else -> "ws://${normalized.removePrefix("http://")}"
        }
        return "$websocketBase/ws?token=${java.net.URLEncoder.encode(accessToken, Charsets.UTF_8.name())}"
    }
}
