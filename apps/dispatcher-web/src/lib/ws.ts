import type { ClientRealtimeEvent, ServerRealtimeEvent } from "@ptt-fleet/shared-types";

export type RealtimeStatus = "idle" | "connecting" | "connected" | "reconnecting";

interface RealtimeClientOptions {
  accessToken: string;
  onEvent: (event: ServerRealtimeEvent) => void;
  onBinary?: (data: ArrayBuffer) => void;
  onStatus: (status: RealtimeStatus) => void;
}

const heartbeatIntervalMs = 25_000;
const maximumReconnectDelayMs = 15_000;

export class RealtimeClient {
  private readonly options: RealtimeClientOptions;
  private socket: WebSocket | null = null;
  private heartbeatTimer: number | null = null;
  private reconnectTimer: number | null = null;
  private reconnectAttempt = 0;
  private stopped = false;

  constructor(options: RealtimeClientOptions) {
    this.options = options;
  }

  connect(): void {
    if (this.socket || this.stopped) {
      return;
    }

    this.options.onStatus(this.reconnectAttempt === 0 ? "connecting" : "reconnecting");
    const socket = new WebSocket(buildWebSocketUrl(this.options.accessToken));
    socket.binaryType = "arraybuffer";
    this.socket = socket;

    socket.addEventListener("open", () => {
      if (this.stopped) {
        socket.close(1000, "dispatcher cleanup");
        return;
      }
      this.reconnectAttempt = 0;
      this.options.onStatus("connected");
      this.startHeartbeat();
    });

    socket.addEventListener("message", (message) => {
      if (message.data instanceof ArrayBuffer) {
        this.options.onBinary?.(message.data);
        return;
      }
      if (typeof message.data !== "string") {
        return;
      }
      try {
        this.options.onEvent(JSON.parse(message.data) as ServerRealtimeEvent);
      } catch {
        // Ignore malformed server events; the connection can continue safely.
      }
    });

    socket.addEventListener("close", () => {
      this.socket = null;
      this.stopHeartbeat();
      if (!this.stopped) {
        this.scheduleReconnect();
      }
    });
  }

  disconnect(): void {
    this.stopped = true;
    this.stopHeartbeat();
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.close(1000, "dispatcher logout");
    }
    this.options.onStatus("idle");
  }

  send(event: ClientRealtimeEvent): boolean {
    if (this.socket?.readyState !== WebSocket.OPEN) {
      return false;
    }
    this.socket.send(JSON.stringify(event));
    return true;
  }

  sendBinary(data: ArrayBuffer): boolean {
    if (this.socket?.readyState !== WebSocket.OPEN) {
      return false;
    }
    this.socket.send(data);
    return true;
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = window.setInterval(() => {
      this.send({
        type: "heartbeat",
        timestamp: new Date().toISOString(),
        payload: {},
      });
    }, heartbeatIntervalMs);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      window.clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private scheduleReconnect(): void {
    this.reconnectAttempt += 1;
    this.options.onStatus("reconnecting");
    const delay = Math.min(1_000 * 2 ** (this.reconnectAttempt - 1), maximumReconnectDelayMs);
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }
}

function buildWebSocketUrl(accessToken: string): string {
  const configuredUrl = import.meta.env.VITE_WS_URL;
  const baseUrl =
    configuredUrl ||
    `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/ws`;
  const url = new URL(baseUrl);
  url.searchParams.set("token", accessToken);
  return url.toString();
}
