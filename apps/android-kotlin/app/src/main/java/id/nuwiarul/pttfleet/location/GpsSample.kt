package id.nuwiarul.pttfleet.location

data class GpsSample(
    val lat: Double,
    val lng: Double,
    val speed: Double?,
    val heading: Double?,
    val accuracy: Double?,
)
