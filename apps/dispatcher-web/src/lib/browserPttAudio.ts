import { OpusDecoder } from "opus-decoder";

export class BrowserPttAudio {
  private context: AudioContext | null = null;
  private decoder: OpusDecoder<48000> | null = null;
  private encoder: AudioEncoder | null = null;
  private stream: MediaStream | null = null;
  private source: MediaStreamAudioSourceNode | null = null;
  private captureNode: AudioWorkletNode | null = null;
  private captureWorkletLoaded = false;
  private captureBuffer: number[] = [];
  private recordingChunks: Float32Array[] = [];
  private recordingSampleRate = 48_000;
  private onRecordingComplete: ((recording: Blob) => void) | null = null;
  private captureTimestamp = 0;
  private lastLevelUpdateAt = 0;
  private nextPlaybackAt = 0;

  constructor(private readonly onError: (message: string) => void) {}

  async enableMonitor(): Promise<void> {
    const context = this.getContext();
    await context.resume();
    if (!this.decoder) {
      this.decoder = new OpusDecoder({
        sampleRate: 48_000,
        channels: 1,
        streamCount: 1,
        coupledStreamCount: 0,
        channelMappingTable: [0],
      });
      await this.decoder.ready;
    }
  }

  disableMonitor(): void {
    this.decoder?.free();
    this.decoder = null;
    this.nextPlaybackAt = 0;
  }

  playOpus(payload: Uint8Array, _sequence: bigint): void {
    void _sequence;
    if (!this.decoder) return;
    try {
      const decoded = this.decoder.decodeFrame(payload);
      if (decoded.errors.length > 0) {
        this.onError(decoded.errors[0]?.message ?? "Unable to decode PTT audio");
        return;
      }
      this.playPcm(decoded.channelData[0], decoded.sampleRate);
    } catch (error) {
      this.onError(messageOf(error, "Unable to decode PTT audio"));
    }
  }

  async startCapture(
    onFrame: (payload: Uint8Array) => void,
    onLevel?: (level: number) => void,
    onRecordingComplete?: (recording: Blob) => void,
  ): Promise<void> {
    if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
      throw new Error("Microphone browser requires HTTPS or localhost.");
    }
    if (this.encoder) return;

    if (typeof AudioEncoder === "undefined" || typeof AudioData === "undefined") {
      throw new Error("Browser ini belum mendukung WebCodecs Opus. Gunakan Chrome/Edge terbaru.");
    }

    const context = this.getContext();
    await context.resume();
    this.recordingChunks = [];
    this.recordingSampleRate = context.sampleRate;
    this.onRecordingComplete = onRecordingComplete ?? null;
    this.stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
    });

    this.encoder = new AudioEncoder({
      output: (chunk) => {
        const payload = new Uint8Array(chunk.byteLength);
        chunk.copyTo(payload);
        onFrame(payload);
      },
      error: (error) => this.onError(error.message),
    });
    this.encoder.configure({
      codec: "opus",
      sampleRate: context.sampleRate,
      numberOfChannels: 1,
      bitrate: 24_000,
    });

    await this.loadCaptureWorklet(context);
    this.source = context.createMediaStreamSource(this.stream);
    this.captureNode = new AudioWorkletNode(context, "ptt-capture-processor", {
      numberOfInputs: 1,
      numberOfOutputs: 0,
      channelCount: 1,
      channelCountMode: "explicit",
    });
    this.captureNode.port.onmessage = (event: MessageEvent<Float32Array>) => {
      this.emitInputLevel(event.data, onLevel);
      this.recordingChunks.push(new Float32Array(event.data));
      this.captureBuffer.push(...event.data);
      this.encodeBufferedAudio(context.sampleRate);
    };
    this.source.connect(this.captureNode);
  }

  async stopCapture(): Promise<void> {
    const wasCapturing = this.captureNode !== null || this.stream !== null;
    if (this.captureNode) {
      this.captureNode.port.onmessage = null;
      this.captureNode.port.close();
      this.captureNode.disconnect();
    }
    this.source?.disconnect();
    this.captureNode = null;
    this.source = null;
    this.stream?.getTracks().forEach((track) => track.stop());
    this.stream = null;
    this.captureBuffer = [];
    this.captureTimestamp = 0;
    this.lastLevelUpdateAt = 0;

    const encoder = this.encoder;
    this.encoder = null;
    if (encoder) {
      await encoder.flush().catch(() => undefined);
      encoder.close();
    }

    if (wasCapturing && this.recordingChunks.length > 0) {
      this.onRecordingComplete?.(createWavBlob(this.recordingChunks, this.recordingSampleRate));
    }
    this.recordingChunks = [];
    this.onRecordingComplete = null;
  }

  async release(): Promise<void> {
    await this.stopCapture();
    this.disableMonitor();
    await this.context?.close();
    this.context = null;
  }

  private getContext(): AudioContext {
    this.context ??= new AudioContext({ latencyHint: "interactive" });
    return this.context;
  }

  private async loadCaptureWorklet(context: AudioContext): Promise<void> {
    if (this.captureWorkletLoaded) return;
    await context.audioWorklet.addModule(
      `${import.meta.env.BASE_URL}worklets/ptt-capture-processor.js`,
    );
    this.captureWorkletLoaded = true;
  }

  private encodeBufferedAudio(sampleRate: number): void {
    if (typeof AudioData === "undefined" || !this.encoder) return;
    const frameSamples = Math.round(sampleRate * 0.02);

    while (this.captureBuffer.length >= frameSamples) {
      const samples = new Float32Array(this.captureBuffer.splice(0, frameSamples));
      const data = new AudioData({
        format: "f32",
        sampleRate,
        numberOfFrames: frameSamples,
        numberOfChannels: 1,
        timestamp: this.captureTimestamp,
        data: samples,
      });
      this.captureTimestamp += 20_000;
      this.encoder.encode(data);
      data.close();
    }
  }

  private emitInputLevel(samples: Float32Array, onLevel?: (level: number) => void): void {
    if (!onLevel) return;
    const now = performance.now();
    if (now - this.lastLevelUpdateAt < 65) return;
    this.lastLevelUpdateAt = now;

    let sumSquares = 0;
    for (const sample of samples) {
      sumSquares += sample * sample;
    }
    const rms = Math.sqrt(sumSquares / samples.length);
    onLevel(Math.min(1, rms * 5));
  }

  private playPcm(samples: Float32Array | undefined, sampleRate: number): void {
    if (!samples?.length) return;
    const context = this.getContext();
    const buffer = context.createBuffer(1, samples.length, sampleRate);
    buffer.copyToChannel(new Float32Array(samples), 0);
    const source = context.createBufferSource();
    source.buffer = buffer;
    source.connect(context.destination);
    const startsAt = Math.max(context.currentTime + 0.03, this.nextPlaybackAt);
    source.start(startsAt);
    this.nextPlaybackAt = startsAt + buffer.duration;
  }
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function createWavBlob(chunks: Float32Array[], sampleRate: number): Blob {
  const sampleCount = chunks.reduce((total, chunk) => total + chunk.length, 0);
  const buffer = new ArrayBuffer(44 + sampleCount * 2);
  const view = new DataView(buffer);
  writeAscii(view, 0, "RIFF");
  view.setUint32(4, 36 + sampleCount * 2, true);
  writeAscii(view, 8, "WAVE");
  writeAscii(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeAscii(view, 36, "data");
  view.setUint32(40, sampleCount * 2, true);

  let offset = 44;
  for (const chunk of chunks) {
    for (const sample of chunk) {
      const clamped = Math.max(-1, Math.min(1, sample));
      view.setInt16(offset, clamped < 0 ? clamped * 0x8000 : clamped * 0x7fff, true);
      offset += 2;
    }
  }
  return new Blob([buffer], { type: "audio/wav" });
}

function writeAscii(view: DataView, offset: number, value: string): void {
  for (let index = 0; index < value.length; index += 1) {
    view.setUint8(offset + index, value.charCodeAt(index));
  }
}
