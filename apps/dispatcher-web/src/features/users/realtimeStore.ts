import type {
  ConnectionReadyEvent,
  GpsUpdatedEvent,
  PresenceStatus,
  PresenceUpdatedEvent,
  ServerRealtimeEvent,
  UserSummary,
} from "@ptt-fleet/shared-types";
import { create } from "zustand";
import type { RealtimeStatus } from "../../lib/ws";

export interface PresenceEntry {
  status: PresenceStatus;
  lastSeenAt: string;
}

export interface UserLocation {
  lat: number;
  lng: number;
  speed?: number;
  heading?: number;
  accuracy?: number;
  recordedAt: string;
}

interface RealtimeState {
  connectionId: string | null;
  connectionStatus: RealtimeStatus;
  users: UserSummary[];
  presence: Record<string, PresenceEntry>;
  locations: Record<string, UserLocation>;
  setConnectionStatus: (status: RealtimeStatus) => void;
  setUsers: (users: UserSummary[]) => void;
  applyEvent: (event: ServerRealtimeEvent) => void;
  reset: () => void;
}

export const useRealtimeStore = create<RealtimeState>((set) => ({
  connectionId: null,
  connectionStatus: "idle",
  users: [],
  presence: {},
  locations: {},
  setConnectionStatus: (connectionStatus) => set({ connectionStatus }),
  setUsers: (users) => set({ users }),
  applyEvent: (event) => {
    if (event.type === "connection.ready") {
      set({ connectionId: (event as ConnectionReadyEvent).payload.connectionId });
      return;
    }
    if (event.type === "presence.updated") {
      const presenceEvent = event as PresenceUpdatedEvent;
      set((state) => ({
        presence: {
          ...state.presence,
          [presenceEvent.payload.userId]: {
            status: presenceEvent.payload.status,
            lastSeenAt: presenceEvent.payload.lastSeenAt,
          },
        },
      }));
      return;
    }
    if (event.type === "gps.updated") {
      const gpsEvent = event as GpsUpdatedEvent;
      set((state) => ({
        locations: {
          ...state.locations,
          [gpsEvent.payload.userId]: {
            lat: gpsEvent.payload.lat,
            lng: gpsEvent.payload.lng,
            speed: gpsEvent.payload.speed,
            heading: gpsEvent.payload.heading,
            accuracy: gpsEvent.payload.accuracy,
            recordedAt: gpsEvent.payload.recordedAt,
          },
        },
      }));
    }
  },
  reset: () =>
    set({
      connectionId: null,
      connectionStatus: "idle",
      users: [],
      presence: {},
      locations: {},
    }),
}));
