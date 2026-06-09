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

class LocationTracker(
    context: Context,
    private val onLocation: (GpsSample) -> Unit,
    private val onError: (String) -> Unit,
) {
    private val client = LocationServices.getFusedLocationProviderClient(context)
    private val request = LocationRequest.Builder(Priority.PRIORITY_HIGH_ACCURACY, UPDATE_INTERVAL_MILLIS)
        .setMinUpdateIntervalMillis(MIN_UPDATE_INTERVAL_MILLIS)
        .setMinUpdateDistanceMeters(MIN_UPDATE_DISTANCE_METERS)
        .build()
    private val callback = object : LocationCallback() {
        override fun onLocationResult(result: LocationResult) {
            result.lastLocation?.let { onLocation(it.toGpsSample()) }
        }
    }
    private var tracking = false

    @SuppressLint("MissingPermission")
    fun start() {
        if (tracking) return
        tracking = true
        client.requestLocationUpdates(request, callback, Looper.getMainLooper())
            .addOnFailureListener {
                tracking = false
                onError(it.message ?: "Unable to start location tracking")
            }
    }

    fun stop() {
        if (!tracking) return
        tracking = false
        client.removeLocationUpdates(callback)
    }

    fun isTracking(): Boolean = tracking

    private fun Location.toGpsSample() = GpsSample(
        lat = latitude,
        lng = longitude,
        speed = speed.takeIf { hasSpeed() }?.toDouble(),
        heading = bearing.takeIf { hasBearing() }?.toDouble(),
        accuracy = accuracy.takeIf { hasAccuracy() }?.toDouble(),
    )

    private companion object {
        const val UPDATE_INTERVAL_MILLIS = 15_000L
        const val MIN_UPDATE_INTERVAL_MILLIS = 5_000L
        const val MIN_UPDATE_DISTANCE_METERS = 5f
    }
}
