package id.nuwiarul.pttfleet.location

enum class LocationTrackingMode {
    MOVING,
    STATIONARY,
}

class AdaptiveLocationPolicy(
    private val movingSpeedThresholdMetersPerSecond: Double = 1.0,
    private val stationarySpeedThresholdMetersPerSecond: Double = 0.5,
    private val stationarySamplesRequired: Int = 6,
) {
    var mode: LocationTrackingMode = LocationTrackingMode.MOVING
        private set

    private var stationarySamples = 0

    fun update(speedMetersPerSecond: Double?): LocationTrackingMode {
        when {
            speedMetersPerSecond == null -> {
                if (mode == LocationTrackingMode.MOVING) {
                    stationarySamples = 0
                }
            }
            speedMetersPerSecond >= movingSpeedThresholdMetersPerSecond -> {
                stationarySamples = 0
                mode = LocationTrackingMode.MOVING
            }
            speedMetersPerSecond <= stationarySpeedThresholdMetersPerSecond -> {
                stationarySamples++
                if (stationarySamples >= stationarySamplesRequired) {
                    mode = LocationTrackingMode.STATIONARY
                }
            }
            else -> stationarySamples = 0
        }

        return mode
    }

    fun reset() {
        mode = LocationTrackingMode.MOVING
        stationarySamples = 0
    }
}
