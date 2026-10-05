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
): 'ready' | 'stale' | 'missing' | 'conflict' {
  const speech = segment.speech
  if (!speech) return 'missing'
  if (
    speech.voiceId !== narration.confirmedVoiceId ||
    speech.bindingDigest !== narration.bindingDigest ||
    speech.inputHash !== segment.inputHash
  )
    return 'stale'
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
