import type { ProtoClipNarration } from '@/shared/api'
import type { ClipNarration } from '../model/spoken'

export function narrationFromProto(
  value: ProtoClipNarration | undefined,
): ClipNarration | undefined {
  if (!value) return undefined
  const volumePermille = value.volumePermille ?? 1000
  if (volumePermille < 0 || volumePermille > 1000) throw new Error('Invalid narration volume')
  return {
    enabled: value.enabled,
    confirmedVoiceId: value.confirmedVoiceId,
    bindingDigest: value.bindingDigest,
    volumePermille,
    segments: value.segments.map((s) => ({
      id: s.id,
      text: s.text,
      textRevision: s.textRevision,
      inputHash: s.inputHash,
      startMs: s.startMs,
      endMs: s.endMs,
      speech: s.speech
        ? {
            assetId: s.speech.assetId,
            voiceId: s.speech.voiceId,
            bindingDigest: s.speech.bindingDigest,
            inputHash: s.speech.inputHash,
            settingsHash: s.speech.settingsHash,
            audioHash: s.speech.audioHash,
            profileId: s.speech.profileId,
            profileRevision: Number(s.speech.profileRevision),
            samples: Number(s.speech.samples),
            sampleRate: s.speech.sampleRate,
            channels: s.speech.channels,
            timing: s.speech.timing.map((t) => ({
              text: t.text,
              startMs: t.startMs,
              endMs: t.endMs,
            })),
          }
        : undefined,
    })),
  }
}
export function narrationToProto(value: ClipNarration | undefined) {
  if (!value) return undefined
  return {
    ...value,
    segments: value.segments.map((s) => ({
      ...s,
      id: s.creation ? '' : s.id,
      speech: s.speech
        ? {
            ...s.speech,
            profileRevision: BigInt(s.speech.profileRevision),
            samples: BigInt(s.speech.samples),
          }
        : undefined,
    })),
  }
}
