import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import type { BrowserMediaSourceAccess } from '@/shared/lib'
import { browserAudioPlan } from './browser-audio-plan'
import {
  loadVerifiedSpeechBuffer,
  prepareBrowserAudioSources,
  type BrowserPreparedAudioSources,
} from './composition-audio'
import {
  SpeechDecodeCache,
  speechDecodeKey,
  SpeechPlaybackError,
  type SpeechAudioLoader,
} from './speech-playback'

/** One gesture-resumed output clock for source-only, speech-only, mixed and silent drafts. */
export class BrowserCompositionPlayback {
  private context?: AudioContext
  private nodes: AudioNode[] = []
  private sources: AudioBufferSourceNode[] = []
  private output?: GainNode
  private anchor?: { context: number; output: number }
  private position = 0
  private epoch = 0
  private controller?: AbortController
  private prepared?: BrowserPreparedAudioSources
  private cache = new SpeechDecodeCache()
  private muted = true
  private disposed = false
  private readonly schedule
  constructor(
    private readonly plan: ClipEditPlan,
    private readonly access: (
      fingerprint: string,
      signal: AbortSignal,
    ) => Promise<BrowserMediaSourceAccess>,
    private readonly loadSpeech: SpeechAudioLoader,
    private readonly createContext = () => new AudioContext({ sampleRate: 48000 }),
  ) {
    this.schedule = browserAudioPlan(plan)
  }
  get timeMs() {
    return Math.round(
      Math.min(
        this.schedule.sampleFrames / this.schedule.sampleRate,
        (this.anchor?.output ?? this.position) +
          (this.anchor && this.context
            ? Math.max(0, this.context.currentTime - this.anchor.context)
            : 0),
      ) * 1000,
    )
  }
  get running() {
    return !!this.anchor && this.context?.state === 'running'
  }
  setMuted(value: boolean) {
    this.muted = value
    if (this.context && this.output)
      this.output.gain.setValueAtTime(value ? 0 : 1, this.context.currentTime)
  }
  pause() {
    this.position = this.timeMs / 1000
    this.anchor = undefined
    this.epoch++
    this.controller?.abort()
    this.controller = undefined
    for (const source of this.sources) {
      try {
        source.stop()
      } catch {
        /* An ended source is already silent. */
      }
    }
    for (const node of this.nodes) node.disconnect()
    this.sources = []
    this.nodes = []
    this.output = undefined
  }
  async play(outputMs: number): Promise<boolean> {
    this.pause()
    if (this.disposed) return false
    this.position = Math.min(
      this.schedule.sampleFrames / this.schedule.sampleRate,
      Math.max(0, outputMs / 1000),
    )
    const epoch = this.epoch,
      controller = (this.controller = new AbortController()),
      signal = controller.signal
    // Synchronous resume in the user's click stack precedes fetching/Worker preparation.
    try {
      this.context ??= this.createContext()
    } catch {
      throw new SpeechPlaybackError('decode')
    }
    const context = this.context,
      resumed = context.resume()
    try {
      await resumed
      signal.throwIfAborted()
      if (context.sampleRate !== this.schedule.sampleRate || context.state !== 'running')
        throw new SpeechPlaybackError('gesture')
      const prepared =
        this.prepared ??
        (await prepareBrowserAudioSources(this.plan, this.access, signal, { strict: false }))
      signal.throwIfAborted()
      if (epoch !== this.epoch || this.disposed) return false
      this.prepared = prepared
      for (const spoken of this.schedule.speech) {
        const key = speechDecodeKey(spoken.speech)
        if (!this.cache.get(key))
          this.cache.put(
            key,
            await loadVerifiedSpeechBuffer(context, spoken.speech, this.loadSpeech, signal),
          )
        signal.throwIfAborted()
      }
      if (epoch !== this.epoch || this.disposed) return false
      this.output = context.createGain()
      this.output.gain.value = this.muted ? 0 : 1
      this.output.connect(context.destination)
      const sourceBus = context.createGain(),
        speechBus = context.createGain()
      sourceBus.connect(this.output)
      speechBus.connect(this.output)
      speechBus.gain.value = this.schedule.narrationVolume
      const dip = 10 ** (CLIP_DESIGN.audio.hook_dip_db / 20),
        start = this.position,
        now = context.currentTime
      sourceBus.gain.setValueAtTime(
        this.schedule.dip.some((window) => start >= window.start && start < window.end) ? dip : 1,
        now,
      )
      for (const window of this.schedule.dip) {
        if (window.start >= start) sourceBus.gain.setValueAtTime(dip, now + window.start - start)
        if (window.end >= start) sourceBus.gain.setValueAtTime(1, now + window.end - start)
      }
      this.nodes.push(this.output, sourceBus, speechBus)
      this.anchor = { context: now, output: start }
      for (const { cut, channels } of prepared.cuts) {
        const offset = Math.max(0, start - cut.start),
          duration = cut.frames / this.schedule.sampleRate
        if (offset >= duration) continue
        const node = context.createBufferSource(),
          gain = context.createGain(),
          buffer = context.createBuffer(2, cut.frames, this.schedule.sampleRate)
        channels.forEach((channel, index) => buffer.copyToChannel(channel, index))
        node.buffer = buffer
        node.playbackRate.value = 1
        const begins = Math.max(start, cut.start),
          at = now + begins - start
        const envelope = Math.max(
          0,
          Math.min(
            cut.fadeIn ? (begins - cut.start) / cut.fadeIn : 1,
            cut.fadeOut ? 1 - (begins - cut.fadeOutStart) / cut.fadeOut : 1,
            1,
          ),
        )
        gain.gain.setValueAtTime(envelope, at)
        if (cut.fadeIn && cut.start + cut.fadeIn > begins)
          gain.gain.linearRampToValueAtTime(1, now + cut.start + cut.fadeIn - start)
        if (cut.fadeOut) {
          if (cut.fadeOutStart > begins) gain.gain.setValueAtTime(1, now + cut.fadeOutStart - start)
          if (cut.fadeOutStart + cut.fadeOut > begins)
            gain.gain.linearRampToValueAtTime(0, now + cut.fadeOutStart + cut.fadeOut - start)
        }
        node.connect(gain).connect(sourceBus)
        node.start(at, offset, duration - offset)
        this.nodes.push(node, gain)
        this.sources.push(node)
      }
      for (const spoken of this.schedule.speech) {
        const offset = Math.max(0, start - spoken.start)
        if (offset >= spoken.duration) continue
        const node = context.createBufferSource()
        node.buffer = this.cache.get(speechDecodeKey(spoken.speech))!
        node.playbackRate.value = 1
        node.connect(speechBus)
        node.start(now + Math.max(0, spoken.start - start), offset, spoken.duration - offset)
        this.nodes.push(node)
        this.sources.push(node)
      }
      return true
    } catch (error) {
      if (signal.aborted || epoch !== this.epoch || this.disposed) return false
      this.pause()
      throw error
    }
  }
  measurements() {
    return {
      liveNodes: this.nodes.length,
      preparedCuts: this.prepared?.cuts.length ?? 0,
      sourceResources: this.prepared?.sourceResources,
      speechIssues: this.schedule.speechIssues,
      epoch: this.epoch,
    }
  }
  dispose() {
    this.pause()
    this.disposed = true
    this.prepared = undefined
    this.cache.clear()
    void this.context?.close()
    this.context = undefined
  }
}
