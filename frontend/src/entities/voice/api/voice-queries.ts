import { useMemo } from 'react'
import type { Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import {
  VoiceValueSource,
  type ProtoVoice,
  type ProtoVoiceProfile,
  type ProtoVoiceRef,
  type ProtoVoiceReadiness,
  type ProtoVoiceSample,
  type StructuredVoiceProfile as ProtoStructured,
  type VoiceProfileVersion as ProtoVersion,
} from '@/shared/api'
import {
  emptyVoice,
  type StructuredVoiceProfile,
  type Voice,
  type VoiceProfile,
  type VoiceReadiness,
  type VoiceRef,
  type VoiceSample,
  type VoiceSourceKind,
  type VoiceVersion,
} from '../model/types'
import { requirePromptPart, requireSampleKind } from './voice-enums'

const source = (value: VoiceValueSource): VoiceSourceKind =>
  value === VoiceValueSource.MEASURED
    ? 'measured'
    : value === VoiceValueSource.ANALYZED
      ? 'analyzed'
      : value === VoiceValueSource.MANUAL
        ? 'manual'
        : 'unknown'
const voiceValue = (
  value: { value: string; source: VoiceValueSource; unknown: boolean } | undefined,
) => ({
  value: value?.value ?? '',
  source: source(value?.source ?? VoiceValueSource.UNKNOWN),
  unknown: value?.unknown ?? true,
})

export function toVoice(voice: ProtoVoice | undefined): Voice {
  if (!voice) return emptyVoice()
  return {
    id: voice.id,
    name: voice.name,
    isDefault: voice.isDefault,
    deleted: voice.deleted,
    createdAt: voice.createdAt,
    updatedAt: voice.updatedAt,
    deletedAt: voice.deletedAt,
    made: voice.made,
    materialCount: voice.materialCount,
    analyzedAt: voice.analyzedAt,
    readinessPercent: voice.readinessPercent,
  }
}
/** A post's voice, or undefined for 말투 없음: the wire leaves the message unset rather than
 *  sending an empty one (POST-25). */
export function toVoiceRef(ref: ProtoVoiceRef | undefined): VoiceRef | undefined {
  if (!ref) return undefined
  return {
    id: ref.id,
    name: ref.name,
    deleted: ref.deleted,
    made: ref.made,
  }
}
export function toVoiceSample(sample: ProtoVoiceSample): VoiceSample {
  return {
    id: sample.id,
    kind: requireSampleKind(sample.kind),
    label: sample.label,
    promptKey: sample.promptKey,
    hasPhoto: sample.hasPhoto,
    chars: sample.chars,
    createdAt: sample.createdAt,
  }
}
export function toReadiness(readiness: ProtoVoiceReadiness | undefined): VoiceReadiness {
  return {
    percent: readiness?.percent ?? 0,
    sentences: readiness?.sentences ?? 0,
    needed: readiness?.needed ?? 0,
    missingParts: readiness?.missingParts.map(requirePromptPart) ?? [],
  }
}
export function toStructured(p: ProtoStructured | undefined): StructuredVoiceProfile {
  return {
    version: p?.meta?.version ?? 0n,
    updatedAt: p?.meta?.updatedAt ?? '',
    sourceCount: p?.meta?.sourceCount ?? 0,
    empty: p?.empty ?? true,
    lexical: {
      description: voiceValue(p?.lexical?.description),
      preferredWords:
        p?.lexical?.preferredWords.map((v) => ({
          word: v.word,
          alternatives: [...v.alternatives],
          weight: v.weight,
        })) ?? [],
      bannedWords: p?.lexical?.bannedWords.map((v) => ({ value: v.value, reason: v.reason })) ?? [],
      bannedPatterns:
        p?.lexical?.bannedPatterns.map((v) => ({ value: v.value, reason: v.reason })) ?? [],
    },
    endings: {
      baseRegister: voiceValue(p?.endings?.baseRegister),
      distribution:
        p?.endings?.distribution.map((v) => ({ ending: v.ending, ratio: v.ratio })) ?? [],
      bannedEndings: [...(p?.endings?.bannedEndings ?? [])],
      signatureEndings: [...(p?.endings?.signatureEndings ?? [])],
      constraints: [...(p?.endings?.constraints ?? [])],
    },
    syntax: {
      averageSentenceChars: p?.syntax?.averageSentenceChars ?? 0,
      sentenceLength: voiceValue(p?.syntax?.sentenceLength),
      connectiveStyle: voiceValue(p?.syntax?.connectiveStyle),
      preferredConnectives: [...(p?.syntax?.preferredConnectives ?? [])],
      nominalization: voiceValue(p?.syntax?.nominalization),
      passiveTendency: voiceValue(p?.syntax?.passiveTendency),
    },
    structure: {
      introPattern: voiceValue(p?.structure?.introPattern),
      closingPattern: voiceValue(p?.structure?.closingPattern),
      paragraphSentencesMin: p?.structure?.paragraphSentencesMin ?? 0,
      paragraphSentencesMax: p?.structure?.paragraphSentencesMax ?? 0,
      headingHabit: voiceValue(p?.structure?.headingHabit),
      listHabit: voiceValue(p?.structure?.listHabit),
      emojiUse: voiceValue(p?.structure?.emojiUse),
    },
    // No `?? 0` here: the wire carries axis presence, and collapsing absence into 0 is exactly
    // the bug this screen used to show — a neutral measurement the model never made.
    axes: {
      involvement: p?.axes?.involvement,
      narrativity: p?.axes?.narrativity,
      persuasionOvertness: p?.axes?.persuasionOvertness,
      abstractness: p?.axes?.abstractness,
      addresseeFocus: p?.axes?.addresseeFocus,
      humor: p?.axes?.humor,
    },
  }
}
export function toVoiceProfile(profile: ProtoVoiceProfile | undefined): VoiceProfile {
  return {
    voice: toVoice(profile?.voice),
    made: profile?.made ?? false,
    readiness: toReadiness(profile?.readiness),
    updatedAt: profile?.updatedAt ?? '',
    samples: profile?.samples.map(toVoiceSample) ?? [],
    activeJobId: profile?.activeJobId ?? '',
    structured: toStructured(profile?.structured),
  }
}
export function toVoiceVersion(version: ProtoVersion): VoiceVersion {
  return {
    version: version.version,
    profile: toStructured(version.profile),
    origin: version.origin,
    restoredFromVersion: version.restoredFromVersion,
    createdAt: version.createdAt,
    hasSample: version.hasSample,
  }
}

export function voiceVersionSampleQueryKey(
  transport: Transport,
  ownerId: string,
  voiceId: string,
  version: bigint,
) {
  return ['voice-version-sample', transport, ownerId, voiceId, version.toString()] as const
}

// Every key carries the account AND the voice (VOICE-56): two voices of one
// account are different aggregates that may contradict each other, and an account switch on the
// same device must never read the previous account's entry. The directory itself is per account.
export function voicesQueryKey(transport: Transport, ownerId: string) {
  return ['voices', transport, ownerId] as const
}
export function voiceProfileQueryKey(transport: Transport, ownerId: string, voiceId: string) {
  return ['voice-profile', transport, ownerId, voiceId] as const
}
export function voiceVersionsQueryKey(transport: Transport, ownerId: string, voiceId: string) {
  return ['voice-versions', transport, ownerId, voiceId] as const
}
/** One opened 학습 글, partitioned like every voice read (VOICE-56). */
export function voiceSampleQueryKey(
  transport: Transport,
  ownerId: string,
  voiceId: string,
  sampleId: string,
) {
  return ['voice-materials', transport, ownerId, voiceId, sampleId] as const
}
/** The shared prompt set is product copy, the same for every account. */
export function voicePromptsQueryKey(transport: Transport) {
  return ['voice-prompts', transport] as const
}

/** The profile entry as a cache target, for a caller that has to say "this job's completion makes
 *  that profile stale" without holding a transport of its own (ARCH-17). */
export function useVoiceProfileQueryKey(ownerId: string, voiceId: string) {
  const transport = useTransport()
  return useMemo(
    () => voiceProfileQueryKey(transport, ownerId, voiceId),
    [ownerId, transport, voiceId],
  )
}
