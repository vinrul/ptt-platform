const headerSize = 25;

export interface AudioDownlinkFrame {
  sessionId: string;
  sequence: bigint;
  payload: Uint8Array;
}

export function encodeAudioUplink(
  sessionId: string,
  sequence: bigint,
  opusPayload: Uint8Array,
): ArrayBuffer {
  const frame = new Uint8Array(headerSize + opusPayload.byteLength);
  frame[0] = 0x01;
  frame.set(uuidToBytes(sessionId), 1);
  new DataView(frame.buffer).setBigUint64(17, sequence, false);
  frame.set(opusPayload, headerSize);
  return frame.buffer;
}

export function decodeAudioDownlink(data: ArrayBuffer): AudioDownlinkFrame | null {
  const frame = new Uint8Array(data);
  if (frame.byteLength <= headerSize || frame[0] !== 0x02) {
    return null;
  }
  return {
    sessionId: bytesToUuid(frame.subarray(1, 17)),
    sequence: new DataView(data).getBigUint64(17, false),
    payload: frame.slice(headerSize),
  };
}

function uuidToBytes(uuid: string): Uint8Array {
  const hex = uuid.replaceAll("-", "");
  if (!/^[0-9a-f]{32}$/i.test(hex)) {
    throw new Error("Invalid PTT session UUID");
  }
  return Uint8Array.from(hex.match(/.{2}/g) ?? [], (value) => Number.parseInt(value, 16));
}

function bytesToUuid(bytes: Uint8Array): string {
  const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20),
  ].join("-");
}
