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
