export const SPOKEN_LIMITS = {
  segments: 32,
  segmentCharacters: 500,
  scriptCharacters: 2000,
} as const
export interface ClipDerivedCaption {
  segmentId: string
  textRevision: number
  textEdited: boolean
  timingEdited: boolean
}
export interface ClipSpeechRef {
  assetId: string
  voiceId: string
  bindingDigest: string
  inputHash: string
  settingsHash: string
  audioHash: string
  profileId: string
  profileRevision: number
  samples: number
  sampleRate: number
  channels: number
  timing: { text: string; startMs: number; endMs: number }[]
}
export interface ClipSpokenSegment {
  id: string
  text: string
  textRevision: number
  inputHash: string
  startMs: number
  endMs: number
  speech?: ClipSpeechRef
  creation?: boolean
}
export interface ClipNarration {
  enabled: boolean
  confirmedVoiceId: string
  bindingDigest: string
  volumePermille: number
  segments: ClipSpokenSegment[]
}
export function speechDurationMs(speech: ClipSpeechRef): number {
  return Math.ceil((speech.samples * 1000) / speech.sampleRate)
}
export function spokenState(
  narration: ClipNarration,
  segment: ClipSpokenSegment,
  durationMs = Infinity,
): 'ready' | 'stale' | 'missing' | 'conflict' {
  const speech = segment.speech
  if (!speech) return 'missing'
  if (
    speech.voiceId !== narration.confirmedVoiceId ||
    speech.bindingDigest !== narration.bindingDigest ||
    speech.inputHash !== segment.inputHash
  )
    return 'stale'
  const index = narration.segments.findIndex((s) => s.id === segment.id)
  const previousEnd = narration.segments[index - 1]?.endMs ?? 0
  if (
    !Number.isSafeInteger(segment.startMs) ||
    !Number.isSafeInteger(segment.endMs) ||
    segment.startMs < previousEnd ||
    segment.endMs <= segment.startMs ||
    segment.endMs > durationMs
  )
    return 'conflict'
  if (segment.startMs + speechDurationMs(speech) > segment.endMs) return 'conflict'
  return 'ready'
}
export function cloneNarration(narration: ClipNarration): ClipNarration {
  return {
    ...narration,
    segments: narration.segments.map((s) => ({
      ...s,
      speech: s.speech
        ? { ...s.speech, timing: s.speech.timing.map((t) => ({ ...t })) }
        : undefined,
    })),
  }
}

export function spokenScriptValid(narration?: ClipNarration): boolean {
  if (!narration) return true
  const n = narration
  return (
    n.segments.length <= SPOKEN_LIMITS.segments &&
    Number.isSafeInteger(n.volumePermille) &&
    n.volumePermille >= 0 &&
    n.volumePermille <= 1000 &&
    n.segments.every(
      (s) =>
        s.text.trim().length > 0 &&
        Array.from(s.text).length <= SPOKEN_LIMITS.segmentCharacters &&
        Number.isSafeInteger(s.startMs) &&
        Number.isSafeInteger(s.endMs) &&
        s.startMs >= 0 &&
        s.endMs > s.startMs,
    ) &&
    n.segments.reduce((sum, s) => sum + Array.from(s.text).length, 0) <=
      SPOKEN_LIMITS.scriptCharacters
  )
}

export function spokenRenderReady(
  narration: ClipNarration | undefined,
  durationMs: number,
): boolean {
  return (
    !narration?.enabled ||
    (!!narration.confirmedVoiceId &&
      !!narration.bindingDigest &&
      narration.segments.length > 0 &&
      narration.segments.every((s) => spokenState(narration, s, durationMs) === 'ready'))
  )
}
