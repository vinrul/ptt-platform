import { afterEach, describe, expect, it, vi } from "vitest";
import { createRequestId } from "./requestId";

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("createRequestId", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses crypto.randomUUID when available", () => {
    const requestId = "123e4567-e89b-42d3-a456-426614174000";
    vi.stubGlobal("crypto", { randomUUID: () => requestId });

    expect(createRequestId()).toBe(requestId);
  });

  it("creates a UUID when randomUUID is unavailable", () => {
    vi.stubGlobal("crypto", {
      getRandomValues: (bytes: Uint8Array) => {
        bytes.fill(0xab);
        return bytes;
      },
    });

    expect(createRequestId()).toMatch(uuidPattern);
  });
});
