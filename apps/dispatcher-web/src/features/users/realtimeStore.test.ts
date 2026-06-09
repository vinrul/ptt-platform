import type { GpsUpdatedEvent, SosAckedEvent, SosCreatedEvent } from "@ptt-fleet/shared-types";
import { beforeEach, describe, expect, it } from "vitest";
import { useRealtimeStore } from "./realtimeStore";

describe("realtimeStore GPS updates", () => {
  beforeEach(() => {
    useRealtimeStore.getState().reset();
  });

  it("stores the latest location per user", () => {
    const event: GpsUpdatedEvent = {
      type: "gps.updated",
      timestamp: "2026-06-09T10:00:00Z",
      payload: {
        userId: "field-1",
        lat: -6.2,
        lng: 106.8,
        accuracy: 4.5,
        recordedAt: "2026-06-09T10:00:00Z",
      },
    };

    useRealtimeStore.getState().applyEvent(event);

    expect(useRealtimeStore.getState().locations["field-1"]).toEqual({
      lat: -6.2,
      lng: 106.8,
      speed: undefined,
      heading: undefined,
      accuracy: 4.5,
      recordedAt: "2026-06-09T10:00:00Z",
    });
  });

  it("tracks SOS create and acknowledgement", () => {
    const created: SosCreatedEvent = {
      type: "sos.created",
      timestamp: "2026-06-09T10:00:00Z",
      payload: {
        id: "sos-1",
        userId: "field-1",
        lat: -6.2,
        lng: 106.8,
        message: "Emergency",
        status: "open",
        createdAt: "2026-06-09T10:00:00Z",
      },
    };
    const acked: SosAckedEvent = {
      type: "sos.acked",
      timestamp: "2026-06-09T10:01:00Z",
      payload: {
        id: "sos-1",
        status: "ack",
        acknowledgedBy: "dispatcher-1",
        acknowledgedAt: "2026-06-09T10:01:00Z",
      },
    };

    useRealtimeStore.getState().applyEvent(created);
    expect(useRealtimeStore.getState().focusedSosId).toBe("sos-1");
    expect(useRealtimeStore.getState().sosAlerts["sos-1"].status).toBe("open");

    useRealtimeStore.getState().applyEvent(acked);
    expect(useRealtimeStore.getState().sosAlerts["sos-1"].status).toBe("ack");
  });
});
