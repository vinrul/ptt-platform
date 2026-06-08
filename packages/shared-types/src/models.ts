export type UserRole = "super_admin" | "dispatcher" | "supervisor" | "field_user";

export type UserStatus = "active" | "disabled";

export type PresenceStatus = "online" | "offline";

export interface UserSummary {
  id: string;
  username: string;
  fullName: string;
  role: UserRole;
  status: UserStatus;
}

export interface GroupSummary {
  id: string;
  name: string;
  description: string;
}
