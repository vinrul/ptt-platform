import type { GpsUpdatedEvent } from "@ptt-fleet/shared-types";
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
});
