package id.nuwiarul.pttfleet.audio

import android.annotation.SuppressLint
import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.AudioTrack
import android.media.MediaRecorder
import io.github.jaredmdobson.concentus.OpusApplication
import io.github.jaredmdobson.concentus.OpusDecoder
import io.github.jaredmdobson.concentus.OpusEncoder
import java.util.concurrent.ArrayBlockingQueue
import java.util.concurrent.ThreadPoolExecutor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class PttAudioEngine(
    private val onEncodedFrame: (ByteArray) -> Unit,
    private val onPlayedFrame: (Int) -> Unit,
    private val onError: (String) -> Unit,
) {
    private val capturing = AtomicBoolean(false)
    private val jitterBuffer = AudioJitterBuffer()
    private val playbackExecutor = ThreadPoolExecutor(
        1,
        1,
        0L,
        TimeUnit.MILLISECONDS,
        ArrayBlockingQueue(MAX_QUEUED_PLAYBACK_FRAMES),
        ThreadPoolExecutor.DiscardOldestPolicy(),
    )
    private var captureThread: Thread? = null
    private var audioRecord: AudioRecord? = null
    private var audioTrack: AudioTrack? = null
    private val encoder =
        OpusEncoder(CAPTURE_SAMPLE_RATE, CHANNELS, OpusApplication.OPUS_APPLICATION_VOIP).apply {
            bitrate = BITRATE
            complexity = 3
            useVBR = true
        }
    private val decoder = OpusDecoder(PLAYBACK_SAMPLE_RATE, CHANNELS)

    @SuppressLint("MissingPermission")
    fun startCapture() {
        if (!capturing.compareAndSet(false, true)) return

        val minimumBuffer = AudioRecord.getMinBufferSize(
            CAPTURE_SAMPLE_RATE,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
        )
        val recorder = AudioRecord(
            MediaRecorder.AudioSource.VOICE_COMMUNICATION,
            CAPTURE_SAMPLE_RATE,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
            maxOf(minimumBuffer, FRAME_SAMPLES * 8),
        )
        if (recorder.state != AudioRecord.STATE_INITIALIZED) {
            capturing.set(false)
            recorder.release()
            onError("Unable to initialize microphone")
            return
        }

        audioRecord = recorder
        recorder.startRecording()
        captureThread = Thread({
            val pcm = ShortArray(FRAME_SAMPLES)
            val encoded = ByteArray(MAX_OPUS_PACKET)
            while (capturing.get()) {
                var offset = 0
                while (capturing.get() && offset < pcm.size) {
                    val count = recorder.read(pcm, offset, pcm.size - offset)
                    if (count <= 0) break
                    offset += count
                }
                if (offset != pcm.size) continue
                runCatching {
                    val length = encoder.encode(pcm, 0, FRAME_SAMPLES, encoded, 0, encoded.size)
                    onEncodedFrame(encoded.copyOf(length))
                }.onFailure {
                    onError(it.message ?: "Opus encoding failed")
                    capturing.set(false)
                }
            }
        }, "ptt-audio-capture").also { it.start() }
    }

    fun stopCapture() {
        if (!capturing.compareAndSet(true, false)) return
        runCatching { audioRecord?.stop() }
        captureThread?.join(500)
        captureThread = null
        audioRecord?.release()
        audioRecord = null
    }

    fun play(sessionId: String, sequence: Long, opusPayload: ByteArray) {
        val readyFrames = jitterBuffer.add(
            BufferedAudioFrame(sessionId, sequence, opusPayload),
        )
        readyFrames.forEach { frame ->
            playbackExecutor.execute { playFrame(frame.payload) }
        }
    }

    private fun playFrame(opusPayload: ByteArray) {
        runCatching {
            val pcm = ShortArray(MAX_DECODE_SAMPLES)
            val samples = decoder.decode(
                opusPayload,
                0,
                opusPayload.size,
                pcm,
                0,
                MAX_DECODE_SAMPLES,
                false,
            )
            PcmGain.apply(pcm, samples, PLAYBACK_GAIN)
            val track = audioTrack ?: createAudioTrack().also {
                audioTrack = it
                it.setVolume(1f)
                it.play()
            }
            val written = track.write(pcm, 0, samples, AudioTrack.WRITE_BLOCKING)
            check(written >= 0) { "AudioTrack write failed with code $written" }
            onPlayedFrame(written)
        }.onFailure {
            onError("Opus playback failed (${opusPayload.size} bytes): ${it.message ?: "decode error"}")
        }
    }

    fun stopPlayback() {
        jitterBuffer.clear()
        playbackExecutor.queue.clear()
        playbackExecutor.execute {
            runCatching {
                audioTrack?.stop()
                audioTrack?.flush()
                audioTrack?.release()
            }
            audioTrack = null
        }
    }

    fun release() {
        stopCapture()
        jitterBuffer.clear()
        playbackExecutor.shutdownNow()
        audioTrack?.stop()
        audioTrack?.release()
        audioTrack = null
    }

    private fun createAudioTrack(): AudioTrack {
        val minimumBuffer = AudioTrack.getMinBufferSize(
            PLAYBACK_SAMPLE_RATE,
            AudioFormat.CHANNEL_OUT_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
        )
        return AudioTrack.Builder()
            .setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                    .build(),
            )
            .setAudioFormat(
                AudioFormat.Builder()
                    .setSampleRate(PLAYBACK_SAMPLE_RATE)
                    .setChannelMask(AudioFormat.CHANNEL_OUT_MONO)
                    .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                    .build(),
            )
            .setBufferSizeInBytes(maxOf(minimumBuffer, FRAME_SAMPLES * 8))
            .setTransferMode(AudioTrack.MODE_STREAM)
            .build()
    }

    private companion object {
        const val CAPTURE_SAMPLE_RATE = 16_000
        const val PLAYBACK_SAMPLE_RATE = 48_000
        const val CHANNELS = 1
        const val FRAME_SAMPLES = CAPTURE_SAMPLE_RATE * 20 / 1_000
        const val MAX_DECODE_SAMPLES = PLAYBACK_SAMPLE_RATE * 120 / 1_000
        const val BITRATE = 24_000
        const val MAX_OPUS_PACKET = 1_276
        const val PLAYBACK_GAIN = 1.8f
        const val MAX_QUEUED_PLAYBACK_FRAMES = 8
    }
}
