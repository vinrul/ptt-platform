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

async function request<T>(
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

  return (await response.json()) as T;
}

function browserDeviceName(): string {
  const platform = navigator.platform || "Browser";
  return `Dispatcher Web - ${platform}`;
}
