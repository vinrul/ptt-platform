import type { GroupSummary, UserSummary } from "@ptt-fleet/shared-types";
import { useAuthStore } from "../features/auth/authStore";

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
  role: "super_admin" | "dispatcher" | "supervisor" | "field_user";
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
let refreshPromise: Promise<AuthSession> | null = null;

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

export async function ensureAccessToken(forceRefresh = false): Promise<string> {
  const session = useAuthStore.getState().session;
  if (!session) {
    throw new Error("Session is no longer available. Please log in again.");
  }
  if (!forceRefresh && !isTokenExpiring(session.accessToken)) {
    return session.accessToken;
  }
  return (await refreshSession()).accessToken;
}

export async function request<T>(
  path: string,
  options: RequestInit,
  accessToken?: string,
): Promise<T> {
  const token = accessToken
    ? await currentAccessToken(accessToken)
    : undefined;
  const response = await performRequest(path, options, token);
  if (response.ok) {
    return parseResponse<T>(response);
  }

  const error = await parseApiError(response);
  if (token && response.status === 401 && error.code === "unauthorized") {
    const storedToken = useAuthStore.getState().session?.accessToken;
    const nextToken = storedToken && storedToken !== token
      ? storedToken
      : await ensureAccessToken(true);
    const retryResponse = await performRequest(path, options, nextToken);
    if (retryResponse.ok) {
      return parseResponse<T>(retryResponse);
    }
    throw await parseApiError(retryResponse);
  }
  throw error;
}

async function currentAccessToken(fallbackToken: string): Promise<string> {
  const session = useAuthStore.getState().session;
  if (!session) return fallbackToken;
  return isTokenExpiring(session.accessToken)
    ? ensureAccessToken(true)
    : session.accessToken;
}

async function refreshSession(): Promise<AuthSession> {
  if (refreshPromise) return refreshPromise;

  const session = useAuthStore.getState().session;
  if (!session) {
    throw new Error("Session is no longer available. Please log in again.");
  }

  refreshPromise = (async () => {
    const response = await performRequest(
      "/api/auth/refresh",
      {
        method: "POST",
        body: JSON.stringify({ refreshToken: session.refreshToken }),
      },
      undefined,
    );
    if (!response.ok) {
      const error = await parseApiError(response);
      if (response.status === 401 || response.status === 403) {
        useAuthStore.getState().clearSession();
      }
      throw error;
    }

    const refreshed = await parseResponse<AuthSession>(response);
    useAuthStore.getState().setSession(refreshed);
    return refreshed;
  })().finally(() => {
    refreshPromise = null;
  });

  return refreshPromise;
}

async function performRequest(
  path: string,
  options: RequestInit,
  accessToken?: string,
): Promise<Response> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  return fetch(`${configuredBaseUrl}${path}`, {
    ...options,
    headers,
  });
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

async function parseApiError(response: Response): Promise<ApiError> {
  let code = "";
  let message = "Request failed";
  try {
    const body = (await response.json()) as ApiErrorBody;
    code = body.error?.code ?? "";
    message = body.error?.message ?? message;
  } catch {
    message = response.statusText || message;
  }
  return new ApiError(response.status, code, message);
}

function isTokenExpiring(accessToken: string): boolean {
  try {
    const [, payload] = accessToken.split(".");
    if (!payload) return false;
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
    const normalized = base64.padEnd(Math.ceil(base64.length / 4) * 4, "=");
    const claims = JSON.parse(window.atob(normalized)) as { exp?: number };
    return typeof claims.exp === "number" && claims.exp * 1_000 <= Date.now() + 30_000;
  } catch {
    return false;
  }
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function browserDeviceName(): string {
  const platform = navigator.platform || "Browser";
  return `Dispatcher Web - ${platform}`;
}
