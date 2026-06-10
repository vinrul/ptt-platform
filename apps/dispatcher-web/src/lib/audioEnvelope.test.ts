import { describe, expect, it } from "vitest";
import { decodeAudioDownlink, encodeAudioUplink } from "./audioEnvelope";

describe("audioEnvelope", () => {
  it("encodes the Android-compatible binary header", () => {
    const frame = new Uint8Array(
      encodeAudioUplink(
        "11111111-1111-4111-8111-111111111111",
        42n,
        Uint8Array.of(1, 2, 3),
      ),
    );

    expect(frame[0]).toBe(0x01);
    expect(frame.byteLength).toBe(28);
    expect(new DataView(frame.buffer).getBigUint64(17, false)).toBe(42n);
    expect(Array.from(frame.slice(25))).toEqual([1, 2, 3]);
  });

  it("decodes a downlink frame", () => {
    const data = encodeAudioUplink(
      "11111111-1111-4111-8111-111111111111",
      7n,
      Uint8Array.of(4, 5),
    );
    new Uint8Array(data)[0] = 0x02;

    expect(decodeAudioDownlink(data)).toEqual({
      sessionId: "11111111-1111-4111-8111-111111111111",
      sequence: 7n,
      payload: Uint8Array.of(4, 5),
    });
  });
});
