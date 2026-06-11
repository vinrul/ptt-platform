package id.nuwiarul.pttfleet.location

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.os.Looper
import com.google.android.gms.location.LocationCallback
import com.google.android.gms.location.LocationRequest
import com.google.android.gms.location.LocationResult
import com.google.android.gms.location.LocationServices
import com.google.android.gms.location.Priority
import com.google.android.gms.tasks.CancellationTokenSource

class LocationTracker(
    context: Context,
    private val onLocation: (GpsSample) -> Unit,
    private val onError: (String) -> Unit,
) {
    private val client = LocationServices.getFusedLocationProviderClient(context)
    private val adaptivePolicy = AdaptiveLocationPolicy()
    private val callback = object : LocationCallback() {
        override fun onLocationResult(result: LocationResult) {
            result.lastLocation?.let { location ->
                onLocation(location.toGpsSample())
                val nextMode = adaptivePolicy.update(location.effectiveSpeedMetersPerSecond())
                previousLocation = Location(location)
                if (tracking && nextMode != activeMode) {
                    activeMode = nextMode
                    restartLocationUpdates()
                }
            }
        }
    }
    private var tracking = false
    private var activeMode = LocationTrackingMode.MOVING
    private var previousLocation: Location? = null
    private var requestGeneration = 0

    @SuppressLint("MissingPermission")
    fun start() {
        if (tracking) return
        tracking = true
        adaptivePolicy.reset()
        activeMode = adaptivePolicy.mode
        previousLocation = null
        registerLocationUpdates(activeMode, ++requestGeneration)
    }

    @SuppressLint("MissingPermission")
    private fun registerLocationUpdates(mode: LocationTrackingMode, generation: Int) {
        client.requestLocationUpdates(buildRequest(mode), callback, Looper.getMainLooper())
            .addOnFailureListener {
                if (generation != requestGeneration) return@addOnFailureListener
                tracking = false
                onError(it.message ?: "Unable to start location tracking")
            }
    }

    private fun restartLocationUpdates() {
        val generation = ++requestGeneration
        client.removeLocationUpdates(callback).addOnCompleteListener {
            if (tracking && generation == requestGeneration) {
                registerLocationUpdates(activeMode, generation)
            }
        }
    }

    fun stop() {
        if (!tracking) return
        tracking = false
        requestGeneration++
        previousLocation = null
        client.removeLocationUpdates(callback)
    }

    fun isTracking(): Boolean = tracking

    @SuppressLint("MissingPermission")
    fun requestCurrentLocation(
        onResult: (GpsSample) -> Unit,
        onUnavailable: (String) -> Unit = onError,
    ) {
        val cancellation = CancellationTokenSource()
        client.getCurrentLocation(Priority.PRIORITY_HIGH_ACCURACY, cancellation.token)
            .addOnSuccessListener { location ->
                if (location != null) {
                    onResult(location.toGpsSample())
                } else {
                    client.lastLocation
                        .addOnSuccessListener { cached ->
                            if (cached != null) onResult(cached.toGpsSample())
                            else onUnavailable("Unable to obtain current location")
                        }
                        .addOnFailureListener {
                            onUnavailable(it.message ?: "Unable to obtain last location")
                        }
                }
            }
            .addOnFailureListener {
                onUnavailable(it.message ?: "Unable to obtain current location")
            }
    }

    private fun Location.toGpsSample() = GpsSample(
        lat = latitude,
        lng = longitude,
        speed = speed.takeIf { hasSpeed() }?.toDouble(),
        heading = bearing.takeIf { hasBearing() }?.toDouble(),
        accuracy = accuracy.takeIf { hasAccuracy() }?.toDouble(),
    )

    private fun Location.effectiveSpeedMetersPerSecond(): Double? {
        if (hasSpeed()) return speed.toDouble()

        val previous = previousLocation ?: return null
        val elapsedSeconds = (elapsedRealtimeNanos - previous.elapsedRealtimeNanos) / NANOS_PER_SECOND
        if (elapsedSeconds <= 0.0) return null

        val distanceMeters = previous.distanceTo(this).toDouble()
        val noiseFloorMeters = maxOf(
            MIN_DERIVED_DISTANCE_METERS,
            accuracy.takeIf { hasAccuracy() }?.toDouble() ?: 0.0,
        )
        if (distanceMeters <= noiseFloorMeters) return 0.0

        return distanceMeters / elapsedSeconds
    }

    private fun buildRequest(mode: LocationTrackingMode): LocationRequest {
        val settings = when (mode) {
            LocationTrackingMode.MOVING -> RequestSettings(
                priority = Priority.PRIORITY_HIGH_ACCURACY,
                intervalMillis = MOVING_INTERVAL_MILLIS,
                minIntervalMillis = MOVING_MIN_INTERVAL_MILLIS,
                minDistanceMeters = MOVING_MIN_DISTANCE_METERS,
            )
            LocationTrackingMode.STATIONARY -> RequestSettings(
                priority = Priority.PRIORITY_BALANCED_POWER_ACCURACY,
                intervalMillis = STATIONARY_INTERVAL_MILLIS,
                minIntervalMillis = STATIONARY_MIN_INTERVAL_MILLIS,
                minDistanceMeters = STATIONARY_MIN_DISTANCE_METERS,
            )
        }

        return LocationRequest.Builder(settings.priority, settings.intervalMillis)
            .setMinUpdateIntervalMillis(settings.minIntervalMillis)
            .setMinUpdateDistanceMeters(settings.minDistanceMeters)
            .build()
    }

    private data class RequestSettings(
        val priority: Int,
        val intervalMillis: Long,
        val minIntervalMillis: Long,
        val minDistanceMeters: Float,
    )

    private companion object {
        const val MOVING_INTERVAL_MILLIS = 10_000L
        const val MOVING_MIN_INTERVAL_MILLIS = 5_000L
        const val MOVING_MIN_DISTANCE_METERS = 5f
        const val STATIONARY_INTERVAL_MILLIS = 60_000L
        const val STATIONARY_MIN_INTERVAL_MILLIS = 30_000L
        const val STATIONARY_MIN_DISTANCE_METERS = 15f
        const val MIN_DERIVED_DISTANCE_METERS = 10.0
        const val NANOS_PER_SECOND = 1_000_000_000.0
    }
}
