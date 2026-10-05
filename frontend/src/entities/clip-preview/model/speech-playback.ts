import type { ClipSpeechRef } from '@/entities/clip-plan/@x/clip-preview'
import { CLIP_SPEECH_PLAYBACK } from '../config/speech-playback'

export type SpeechAudioLoader = (speech: ClipSpeechRef, signal: AbortSignal) => Promise<ArrayBuffer>
export interface ScheduledSpeech {
  segmentId: string
  start: number
  duration: number
  speech: ClipSpeechRef
}
export class SpeechPlaybackError extends Error {
  constructor(
    readonly reason: 'decode' | 'memory' | 'unavailable' | 'gesture',
    readonly segmentId = '',
  ) {
    super(reason)
  }
}
/** Immutable PCM stays in memory only, bounded independently of encoded byte size. */
export class SpeechDecodeCache {
  private buffers = new Map<string, AudioBuffer>()
  get(key: string) {
    return this.buffers.get(key)
  }
  retain(keys: ReadonlySet<string>) {
    for (const key of this.buffers.keys()) if (!keys.has(key)) this.buffers.delete(key)
  }
  put(key: string, value: AudioBuffer) {
    const bytes = value.length * value.numberOfChannels * Float32Array.BYTES_PER_ELEMENT
    let total = bytes
    for (const [other, buffer] of this.buffers)
      if (other !== key)
        total += buffer.length * buffer.numberOfChannels * Float32Array.BYTES_PER_ELEMENT
    if (total > CLIP_SPEECH_PLAYBACK.decodedBytes) throw new SpeechPlaybackError('memory')
    this.buffers.set(key, value)
  }
  clear() {
    this.buffers.clear()
  }
}
export const speechDecodeKey = (s: ClipSpeechRef) =>
  `${s.audioHash}:${s.samples}:${s.sampleRate}:${s.channels}`

/** One output clock across cuts; a seek cancels every old node before new scheduling. */
export class SpeechPreviewTransport {
  private context?: AudioContext
  private nodes: AudioBufferSourceNode[] = []
  private gain?: GainNode
  private anchor?: { context: number; output: number }
  private position = 0
  private epoch = 0
  private abort?: AbortController
  private muted = false
  constructor(
    private readonly segments: readonly ScheduledSpeech[],
    private readonly duration: number,
    private readonly volume: number,
    private readonly load: SpeechAudioLoader,
    private readonly cache: SpeechDecodeCache,
    private readonly createContext = () =>
      new AudioContext({ sampleRate: CLIP_SPEECH_PLAYBACK.sampleRate }),
  ) {}
  get timeMs() {
    const elapsed =
      this.anchor && this.context ? Math.max(0, this.context.currentTime - this.anchor.context) : 0
    return Math.round(
      Math.min(this.duration, (this.anchor?.output ?? this.position) + elapsed) * 1000,
    )
  }
  get running() {
    return !!this.anchor && this.context?.state === 'running'
  }
  setMuted(muted: boolean) {
    this.muted = muted
    if (this.gain && this.context)
      this.gain.gain.setValueAtTime(muted ? 0 : this.volume, this.context.currentTime)
  }
  pause() {
    this.position = this.timeMs / 1000
    this.anchor = undefined
    ++this.epoch
    this.abort?.abort()
    for (const node of this.nodes) {
      try {
        node.stop()
      } catch {
        /* A cancelled scheduled node may already have ended. */
      }
      node.disconnect()
    }
    this.nodes = []
    this.gain?.disconnect()
    this.gain = undefined
  }
  async play(outputMs: number): Promise<boolean> {
    this.pause()
    this.position = Math.max(0, Math.min(this.duration, outputMs / 1000))
    const epoch = this.epoch
    const abort = (this.abort = new AbortController())
    // Resume is invoked in the click call stack, before any asynchronous fetching.
    try {
      this.context ??= this.createContext()
    } catch {
      throw new SpeechPlaybackError('decode')
    }
    const context = this.context
    const resumed = context.resume()
    const wanted = new Set(this.segments.map((s) => speechDecodeKey(s.speech)))
    this.cache.retain(wanted)
    let estimate = 0
    for (const key of wanted) {
      const s = this.segments.find((s) => speechDecodeKey(s.speech) === key)!
      estimate +=
        Math.ceil(s.duration * context.sampleRate) *
        s.speech.channels *
        Float32Array.BYTES_PER_ELEMENT
    }
    try {
      try {
        await resumed
      } catch {
        throw new SpeechPlaybackError('gesture')
      }
      abort.signal.throwIfAborted()
      if (estimate > CLIP_SPEECH_PLAYBACK.decodedBytes) throw new SpeechPlaybackError('memory')
      for (const segment of this.segments) {
        abort.signal.throwIfAborted()
        const key = speechDecodeKey(segment.speech)
        if (this.cache.get(key)) continue
        let bytes: ArrayBuffer
        try {
          bytes = await this.load(segment.speech, abort.signal)
        } catch {
          abort.signal.throwIfAborted()
          throw new SpeechPlaybackError('unavailable', segment.segmentId)
        }
        abort.signal.throwIfAborted()
        let buffer: AudioBuffer
        try {
          buffer = await context.decodeAudioData(bytes)
        } catch {
          throw new SpeechPlaybackError('decode', segment.segmentId)
        }
        abort.signal.throwIfAborted()
        if (
          buffer.numberOfChannels !== segment.speech.channels ||
          !Number.isFinite(buffer.duration) ||
          Math.abs(buffer.duration - segment.duration) >
            CLIP_SPEECH_PLAYBACK.durationToleranceSeconds
        )
          throw new SpeechPlaybackError('decode', segment.segmentId)
        this.cache.put(key, buffer)
      }
      if (epoch !== this.epoch || abort.signal.aborted) return false
      if (context.state !== 'running') throw new SpeechPlaybackError('gesture')
      this.gain = context.createGain()
      this.gain.gain.setValueAtTime(this.muted ? 0 : this.volume, context.currentTime)
      this.gain.connect(context.destination)
      this.anchor = { context: context.currentTime, output: this.position }
      for (const segment of this.segments) {
        const offset = Math.max(0, this.position - segment.start)
        if (offset >= segment.duration) continue
        const node = context.createBufferSource()
        node.buffer = this.cache.get(speechDecodeKey(segment.speech))!
        node.playbackRate.value = 1
        node.connect(this.gain)
        node.start(
          this.anchor.context + Math.max(0, segment.start - this.position),
          offset,
          segment.duration - offset,
        )
        this.nodes.push(node)
      }
      return true
    } catch (error) {
      if (epoch !== this.epoch || abort.signal.aborted) return false
      this.pause()
      throw error
    }
  }
  dispose() {
    this.pause()
    void this.context?.close()
    this.context = undefined
  }
}
