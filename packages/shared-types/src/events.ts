import type { PresenceStatus, UserRole } from "./models";

export interface RealtimeEvent<TType extends string, TPayload extends object> {
  type: TType;
  requestId?: string;
  timestamp: string;
  payload: TPayload;
}

export type ConnectionReadyEvent = RealtimeEvent<
  "connection.ready",
  {
    connectionId: string;
    userId: string;
    role: UserRole;
  }
>;

export type PresenceUpdatedEvent = RealtimeEvent<
  "presence.updated",
  {
    userId: string;
    status: PresenceStatus;
    lastSeenAt: string;
  }
>;

export type GroupJoinEvent = RealtimeEvent<"group.join", { groupId: string }>;

export type GroupJoinedEvent = RealtimeEvent<"group.joined", { groupId: string }>;

export type HeartbeatEvent = RealtimeEvent<"heartbeat", Record<string, never>>;

export type GpsUpdateEvent = RealtimeEvent<
  "gps.update",
  {
    lat: number;
    lng: number;
    speed?: number;
    heading?: number;
    accuracy?: number;
  }
>;

export type GpsUpdatedEvent = RealtimeEvent<
  "gps.updated",
  {
    userId: string;
    lat: number;
    lng: number;
    speed?: number;
    heading?: number;
    accuracy?: number;
    recordedAt: string;
  }
>;

export type SosCreatedEvent = RealtimeEvent<
  "sos.created",
  {
    id: string;
    userId: string;
    lat?: number;
    lng?: number;
    message: string;
    status: "open";
    createdAt: string;
  }
>;

export type SosCreateEvent = RealtimeEvent<
  "sos.create",
  {
    lat?: number;
    lng?: number;
    message: string;
  }
>;

export type SosAckEvent = RealtimeEvent<"sos.ack", { id: string }>;

export type SosAckedEvent = RealtimeEvent<
  "sos.acked",
  {
    id: string;
    status: "ack";
    acknowledgedBy: string;
    acknowledgedAt: string;
  }
>;

export type PttStateEvent =
  | RealtimeEvent<"ptt.granted", { sessionId: string; groupId: string }>
  | RealtimeEvent<"ptt.busy", { groupId: string; speakerUserId: string }>
  | RealtimeEvent<"ptt.started", { sessionId: string; groupId: string; speakerUserId: string }>
  | RealtimeEvent<
      "ptt.stopped",
      {
        sessionId: string;
        groupId: string;
        speakerUserId: string;
        reason: "user_stop" | "disconnect" | "timeout" | "server_error";
      }
    >;

export type ErrorEvent = RealtimeEvent<
  "error",
  {
    code: string;
    message: string;
    details: Record<string, unknown>;
  }
>;

export type ClientRealtimeEvent =
  | HeartbeatEvent
  | GroupJoinEvent
  | GpsUpdateEvent
  | SosCreateEvent
  | SosAckEvent;

export type ServerRealtimeEvent =
  | ConnectionReadyEvent
  | PresenceUpdatedEvent
  | GroupJoinedEvent
  | GpsUpdatedEvent
  | SosCreatedEvent
  | SosAckedEvent
  | PttStateEvent
  | ErrorEvent;
