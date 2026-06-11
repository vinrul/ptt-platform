package id.nuwiarul.pttfleet.groups

data class GroupSummary(
    val id: String,
    val name: String,
    val description: String,
)

data class GroupMember(
    val userId: String,
    val username: String,
    val fullName: String,
    val role: String,
    val roleInGroup: String,
)

data class GroupLocation(
    val userId: String,
    val username: String,
    val fullName: String,
    val role: String,
    val lat: Double,
    val lng: Double,
    val speed: Double?,
    val heading: Double?,
    val accuracy: Double?,
    val recordedAt: String,
)
