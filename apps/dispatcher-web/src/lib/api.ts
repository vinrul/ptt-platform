import type { GroupSummary, UserSummary } from "@ptt-fleet/shared-types";

export type AuthUser = UserSummary;

export interface AuthSession {
  accessToken: string;
  refreshToken: string;
  user: AuthUser;
}

export interface UserListResponse {
  items: UserSummary[];
  page: number;
  pageSize: number;
  total: number;
}

export interface GroupListResponse {
  items: GroupSummary[];
}

export interface GroupMember {
  userId: string;
  username: string;
  fullName: string;
  roleInGroup: "member" | "dispatcher" | "supervisor";
  joinedAt: string;
}

export interface GroupDetail extends GroupSummary {
  members: GroupMember[];
}

export interface DeviceSummary {
  id: string;
  userId: string;
  username: string;
  fullName: string;
  deviceName: string;
  deviceImei?: string;
  platform: "android" | "web";
  status: "active" | "disabled";
  lastSeenAt?: string;
  createdAt: string;
}

export interface AuditLog {
  id: number;
  actorUserId?: string;
  actorUsername?: string;
  action: string;
  entityType: string;
  entityId?: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface AuditListResponse {
  items: AuditLog[];
  page: number;
  pageSize: number;
  total: number;
}

interface ApiErrorBody {
  error?: {
    code?: string;
    message?: string;
  };
}

const configuredBaseUrl = import.meta.env.VITE_API_URL?.replace(/\/$/, "") ?? "";

export async function login(
  username: string,
  password: string,
  signal?: AbortSignal,
): Promise<AuthSession> {
  return request<AuthSession>(
    "/api/auth/login",
    {
      method: "POST",
      body: JSON.stringify({
        username,
        password,
        deviceName: browserDeviceName(),
        clientType: "web",
      }),
      signal,
    },
    undefined,
  );
}

export async function fetchUsers(accessToken: string): Promise<UserListResponse> {
  return request<UserListResponse>("/api/users?pageSize=100", {}, accessToken);
}

export async function fetchGroups(accessToken: string): Promise<GroupListResponse> {
  return request<GroupListResponse>("/api/groups", {}, accessToken);
}

export async function createUser(
  accessToken: string,
  input: { username: string; password: string; fullName: string; role: string },
): Promise<UserSummary> {
  return request<UserSummary>(
    "/api/users",
    { method: "POST", body: JSON.stringify(input) },
    accessToken,
  );
}

export async function updateUser(
  accessToken: string,
  userId: string,
  input: { fullName?: string; role?: string; status?: string },
): Promise<UserSummary> {
  return request<UserSummary>(
    `/api/users/${userId}`,
    { method: "PATCH", body: JSON.stringify(input) },
    accessToken,
  );
}

export async function createGroup(
  accessToken: string,
  input: { name: string; description: string },
): Promise<GroupSummary> {
  return request<GroupSummary>(
    "/api/groups",
    { method: "POST", body: JSON.stringify(input) },
    accessToken,
  );
}

export async function fetchGroup(accessToken: string, groupId: string): Promise<GroupDetail> {
  return request<GroupDetail>(`/api/groups/${groupId}`, {}, accessToken);
}

export async function addGroupMember(
  accessToken: string,
  groupId: string,
  userId: string,
): Promise<GroupMember> {
  return request<GroupMember>(
    `/api/groups/${groupId}/members`,
    { method: "POST", body: JSON.stringify({ userId, roleInGroup: "member" }) },
    accessToken,
  );
}

export async function removeGroupMember(
  accessToken: string,
  groupId: string,
  userId: string,
): Promise<void> {
  await request<void>(
    `/api/groups/${groupId}/members/${userId}`,
    { method: "DELETE" },
    accessToken,
  );
}

export async function fetchDevices(accessToken: string): Promise<{ items: DeviceSummary[] }> {
  return request<{ items: DeviceSummary[] }>("/api/devices", {}, accessToken);
}

export async function fetchAuditLogs(accessToken: string): Promise<AuditListResponse> {
  return request<AuditListResponse>("/api/audit-logs?pageSize=100", {}, accessToken);
}

export async function request<T>(
  path: string,
  options: RequestInit,
  accessToken?: string,
): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  const response = await fetch(`${configuredBaseUrl}${path}`, {
    ...options,
    headers,
  });
  if (!response.ok) {
    let message = "Request failed";
    try {
      const body = (await response.json()) as ApiErrorBody;
      message = body.error?.message ?? message;
    } catch {
      message = response.statusText || message;
    }
    throw new Error(message);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

function browserDeviceName(): string {
  const platform = navigator.platform || "Browser";
  return `Dispatcher Web - ${platform}`;
}
