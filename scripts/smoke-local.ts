const apiUrl = (process.env.SMOKE_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
const password = process.env.SEED_PASSWORD ?? "ptt-local-123";

await main().catch((error: unknown) => {
  console.error(`Smoke test failed: ${error instanceof Error ? error.message : error}`);
  process.exit(1);
});

async function main() {
  await expectStatus("/healthz", 200);
  await expectStatus("/readyz", 200);

  const dispatcher = await login("dispatcher");
  const field1 = await login("field1");
  const field2 = await login("field2");

  const users = await api<{ items: unknown[] }>("/api/users?pageSize=100", dispatcher.accessToken);
  assert(users.items.length >= 4, "expected at least four seeded users");

  const groups = await api<{ items: { id: string; name: string }[] }>(
    "/api/groups",
    field1.accessToken,
  );
  const group = groups.items.find((item) => item.name === "Default Patrol");
  assert(group, "Default Patrol group not found for field1");

  const first = await openRealtime(field1.accessToken);
  const second = await openRealtime(field2.accessToken);
  const operator = await openRealtime(dispatcher.accessToken);
  try {
    await first.waitFor("connection.ready");
    await second.waitFor("connection.ready");
    await operator.waitFor("connection.ready");
    first.send("group.join", { groupId: group.id });
    second.send("group.join", { groupId: group.id });
    await first.waitFor("group.joined");
    await second.waitFor("group.joined");

    first.send("ptt.start", { groupId: group.id });
    const granted = await first.waitFor("ptt.granted");
    await first.waitFor("ptt.started");
    await second.waitFor("ptt.started");

    second.send("ptt.start", { groupId: group.id });
    await second.waitFor("ptt.busy");

    first.send("ptt.stop", { sessionId: granted.payload.sessionId });
    await first.waitFor("ptt.stopped");
    await second.waitFor("ptt.stopped");

    first.send("sos.create", { message: "Local smoke SOS", lat: -6.2, lng: 106.8 });
    const created = await operator.waitFor("sos.created");
    operator.send("sos.ack", { id: created.payload.id });
    await operator.waitFor("sos.acked");
  } finally {
    first.close();
    second.close();
    operator.close();
  }

  console.log("Smoke test passed: health, auth, users, groups, PTT lock, and SOS ack.");
}

async function login(username: string) {
  return api<{ accessToken: string; refreshToken: string }>(
    "/api/auth/login",
    undefined,
    {
      method: "POST",
      body: JSON.stringify({
        username,
        password,
        deviceName: `Smoke Test ${username}`,
        clientType: "web",
      }),
    },
  );
}

async function expectStatus(path: string, expected: number) {
  const response = await fetch(`${apiUrl}${path}`);
  assert(response.status === expected, `${path} returned HTTP ${response.status}`);
}

async function api<T>(path: string, token?: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const response = await fetch(`${apiUrl}${path}`, { ...options, headers });
  if (!response.ok) {
    throw new Error(`${path} returned HTTP ${response.status}: ${await response.text()}`);
  }
  return (await response.json()) as T;
}

async function openRealtime(accessToken: string) {
  const url = new URL(apiUrl.replace(/^http/, "ws") + "/ws");
  url.searchParams.set("token", accessToken);
  const socket = new WebSocket(url);
  const queued: RealtimeEvent[] = [];
  const waiters = new Map<string, ((event: RealtimeEvent) => void)[]>();

  socket.addEventListener("message", (message) => {
    if (typeof message.data !== "string") return;
    const event = JSON.parse(message.data) as RealtimeEvent;
    const waiter = waiters.get(event.type)?.shift();
    if (waiter) waiter(event);
    else queued.push(event);
  });
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("WebSocket open timeout")), 5_000);
    socket.addEventListener("open", () => {
      clearTimeout(timer);
      resolve();
    });
    socket.addEventListener("error", () => reject(new Error("WebSocket connection failed")));
  });

  return {
    send(type: string, payload: Record<string, unknown>) {
      socket.send(
        JSON.stringify({
          type,
          requestId: crypto.randomUUID(),
          timestamp: new Date().toISOString(),
          payload,
        }),
      );
    },
    waitFor(type: string): Promise<RealtimeEvent> {
      const existing = queued.findIndex((event) => event.type === type);
      if (existing >= 0) return Promise.resolve(queued.splice(existing, 1)[0]);
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error(`Timed out waiting for ${type}`)), 5_000);
        const resolver = (event: RealtimeEvent) => {
          clearTimeout(timer);
          resolve(event);
        };
        waiters.set(type, [...(waiters.get(type) ?? []), resolver]);
      });
    },
    close() {
      socket.close();
    },
  };
}

interface RealtimeEvent {
  type: string;
  payload: Record<string, string>;
}

function assert(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}
