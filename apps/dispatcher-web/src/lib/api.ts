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

export interface GpsHistoryPoint {
  userId: string;
  lat: number;
  lng: number;
  speed?: number;
  heading?: number;
  accuracy?: number;
  recordedAt: string;
}

export interface GpsHistoryResponse {
  user: AuthUser;
  items: GpsHistoryPoint[];
}

export interface GroupLocation extends GpsHistoryPoint {
  username: string;
  fullName: string;
  role: string;
}

export interface GroupLocationsResponse {
  items: GroupLocation[];
}

export interface ReverseGeocodeResponse {
  displayName: string;
}

export interface RouteLineResponse {
  coordinates: Array<[number, number]>;
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

export async function changePassword(
  accessToken: string,
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  await request<void>(
    "/api/auth/change-password",
    {
      method: "POST",
      body: JSON.stringify({ currentPassword, newPassword }),
    },
    accessToken,
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

export async function resetUserPassword(
  accessToken: string,
  userId: string,
  newPassword: string,
): Promise<void> {
  await request<void>(
    `/api/users/${userId}/reset-password`,
    {
      method: "POST",
      body: JSON.stringify({ newPassword }),
    },
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

export async function fetchGroupLocations(
  accessToken: string,
  groupId: string,
  hours?: number,
): Promise<GroupLocationsResponse> {
  const query = hours ? `?${new URLSearchParams({ hours: String(hours) }).toString()}` : "";
  return request<GroupLocationsResponse>(`/api/groups/${groupId}/locations${query}`, {}, accessToken);
}

export async function fetchGpsHistory(
  accessToken: string,
  userId: string,
  hours = 24,
): Promise<GpsHistoryResponse> {
  const to = new Date();
  const from = new Date(to.getTime() - hours * 60 * 60 * 1_000);
  const query = new URLSearchParams({
    from: from.toISOString(),
    to: to.toISOString(),
    limit: "500",
  });
  return request<GpsHistoryResponse>(
    `/api/users/${userId}/gps-history?${query.toString()}`,
    {},
    accessToken,
  );
}

export async function fetchGpsHistoryForDate(
  accessToken: string,
  userId: string,
  date: string,
): Promise<GpsHistoryResponse> {
  const from = new Date(`${date}T00:00:00`);
  const to = new Date(from);
  to.setDate(to.getDate() + 1);
  const query = new URLSearchParams({
    from: from.toISOString(),
    to: to.toISOString(),
    limit: "1000",
  });
  return request<GpsHistoryResponse>(
    `/api/users/${userId}/gps-history?${query.toString()}`,
    {},
    accessToken,
  );
}

export async function fetchReverseGeocode(
  accessToken: string,
  lat: number,
  lng: number,
): Promise<ReverseGeocodeResponse> {
  const query = new URLSearchParams({ lat: String(lat), lng: String(lng) });
  return request<ReverseGeocodeResponse>(`/api/geocode/reverse?${query.toString()}`, {}, accessToken);
}

export async function fetchRouteLine(
  accessToken: string,
  points: GpsHistoryPoint[],
): Promise<RouteLineResponse> {
  return request<RouteLineResponse>(
    "/api/routes/line",
    {
      method: "POST",
      body: JSON.stringify({
        points: sampledRoutePoints(points).map((point) => ({
          lat: point.lat,
          lng: point.lng,
        })),
      }),
    },
    accessToken,
  );
}

function sampledRoutePoints(points: GpsHistoryPoint[]): GpsHistoryPoint[] {
  const chronological = [...points].sort((left, right) =>
    left.recordedAt.localeCompare(right.recordedAt),
  );
  const deduped = chronological.filter((point, index, source) => {
    const previous = source[index - 1];
    return !previous || previous.lat !== point.lat || previous.lng !== point.lng;
  });
  if (deduped.length <= 80) return deduped;

  const interval = (deduped.length - 1) / 79;
  return Array.from({ length: 80 }, (_, index) => deduped[Math.round(index * interval)]);
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
