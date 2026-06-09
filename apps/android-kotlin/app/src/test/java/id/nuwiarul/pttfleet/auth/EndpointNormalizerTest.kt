package id.nuwiarul.pttfleet.auth

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class EndpointNormalizerTest {
    @Test
    fun addsHttpSchemeAndNormalizesTrailingSlash() {
        assertEquals(
            "http://10.0.2.2:8080",
            EndpointNormalizer.serverUrl("10.0.2.2:8080/"),
        )
    }

    @Test
    fun buildsWebSocketEndpoint() {
        assertEquals(
            "wss://api.example.com/ws?token=access-token",
            EndpointNormalizer.websocketUrl("https://api.example.com", "access-token"),
        )
    }

    @Test
    fun rejectsUnsupportedScheme() {
        assertThrows(IllegalArgumentException::class.java) {
            EndpointNormalizer.serverUrl("ftp://example.com")
        }
    }
}
