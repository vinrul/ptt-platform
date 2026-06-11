import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "../features/auth/authStore";
import { fetchGpsHistory, fetchUsers } from "./api";

const user = {
  id: "dispatcher-1",
  username: "dispatcher",
  fullName: "Dispatcher",
  role: "dispatcher" as const,
  status: "active" as const,
};

describe("API access token refresh", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useAuthStore.setState({
      session: {
        accessToken: "expired-access",
        refreshToken: "refresh-1",
        user,
      },
    });
    vi.restoreAllMocks();
  });

  it("refreshes once and retries an unauthorized request", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(jsonResponse(401, {
        error: { code: "unauthorized", message: "Access token is invalid or expired" },
      }))
      .mockResolvedValueOnce(jsonResponse(200, {
        accessToken: "fresh-access",
        refreshToken: "refresh-2",
        user,
      }))
      .mockResolvedValueOnce(jsonResponse(200, {
        items: [],
        page: 1,
        pageSize: 100,
        total: 0,
      }));

    await expect(fetchUsers("expired-access")).resolves.toMatchObject({ items: [] });
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(useAuthStore.getState().session?.accessToken).toBe("fresh-access");
    expect(useAuthStore.getState().session?.refreshToken).toBe("refresh-2");
  });

  it("shares one refresh across parallel unauthorized requests", async () => {
    let resolveRefresh!: (response: Response) => void;
    const refreshResponse = new Promise<Response>((resolve) => {
      resolveRefresh = resolve;
    });
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = String(input);
      if (url.endsWith("/api/auth/refresh")) return refreshResponse;
      const headers = new Headers(init?.headers);
      if (headers.get("Authorization") === "Bearer fresh-access") {
        return jsonResponse(200, { items: [], page: 1, pageSize: 100, total: 0 });
      }
      return jsonResponse(401, {
        error: { code: "unauthorized", message: "expired" },
      });
    });

    const first = fetchUsers("expired-access");
    const second = fetchUsers("expired-access");
    await vi.waitFor(() => {
      expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith("/api/auth/refresh"))).toHaveLength(1);
    });
    resolveRefresh(jsonResponse(200, {
      accessToken: "fresh-access",
      refreshToken: "refresh-2",
      user,
    }));

    await Promise.all([first, second]);
    expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith("/api/auth/refresh"))).toHaveLength(1);
  });

  it("requests a bounded GPS history window", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse(200, { user, items: [] }),
    );

    await fetchGpsHistory("expired-access", "field-1");

    const requestUrl = new URL(String(fetchMock.mock.calls[0][0]), window.location.origin);
    expect(requestUrl.pathname).toBe("/api/users/field-1/gps-history");
    expect(requestUrl.searchParams.get("limit")).toBe("500");
    expect(requestUrl.searchParams.has("from")).toBe(true);
    expect(requestUrl.searchParams.has("to")).toBe(true);
  });
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
