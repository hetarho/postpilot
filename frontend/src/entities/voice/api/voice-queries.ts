import { useMemo } from 'react'
import type { Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import {
  type ProtoVoice,
  type ProtoVoiceAnalysis,
  type ProtoVoiceExample,
  type ProtoVoiceFingerprint,
  type ProtoVoiceNotice,
  type ProtoVoiceProfile,
  type ProtoVoiceRef,
  type ProtoVoiceReadiness,
  type ProtoVoiceSample,
} from '@/shared/api'
import {
  emptyVoice,
  type Voice,
  type VoiceAnalysis,
  type VoiceExample,
  type VoiceFingerprint,
  type VoiceNotice,
  type VoiceProfile,
  type VoiceReadiness,
  type VoiceRef,
  type VoiceSample,
} from '../model/types'
import {
  requireAiField,
  requireNoticeKind,
  requirePromptPart,
  requireSampleKind,
} from './voice-enums'

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
const example = (value: ProtoVoiceExample | undefined): VoiceExample | undefined =>
  value?.sentence ? { sentence: value.sentence, materialId: value.materialId } : undefined

/** The counted items as every screen shows them — the analysis, ②'s comparison, 검증. A missing
 *  item reads as unknown, never as zeros. */
export function toFingerprint(p: ProtoVoiceFingerprint | undefined): VoiceFingerprint {
  return {
    sentences: p?.sentences ?? 0,
    endings: {
      unknown: p?.endings?.unknown ?? true,
      da: p?.endings?.da ?? 0,
      haeyo: p?.endings?.haeyo ?? 0,
      seumnida: p?.endings?.seumnida ?? 0,
      other: p?.endings?.other ?? 0,
      suffixes:
        p?.endings?.suffixes.map((suffix) => ({ text: suffix.text, count: suffix.count })) ?? [],
      example: example(p?.endings?.example),
    },
    marks: {
      unknown: p?.marks?.unknown ?? true,
      exclaim: p?.marks?.exclaim ?? 0,
      question: p?.marks?.question ?? 0,
      tilde: p?.marks?.tilde ?? 0,
      ellipsis: p?.marks?.ellipsis ?? 0,
      period: p?.marks?.period ?? 0,
      none: p?.marks?.none ?? 0,
      repeat: p?.marks?.repeat ?? 0,
      example: example(p?.marks?.example),
    },
    emoji: {
      unknown: p?.emoji?.unknown ?? true,
      emoji: p?.emoji?.emoji ?? 0,
      hh: p?.emoji?.hh ?? 0,
      kk: p?.emoji?.kk ?? 0,
      tears: p?.emoji?.tears ?? 0,
      example: example(p?.emoji?.example),
    },
    shape: {
      unknown: p?.shape?.unknown ?? true,
      averageChars: p?.shape?.averageChars ?? 0,
      paragraphAverage: p?.shape?.paragraphAverage ?? 0,
      paragraphMin: p?.shape?.paragraphMin ?? 0,
      paragraphMax: p?.shape?.paragraphMax ?? 0,
      lineBreakShare: p?.shape?.lineBreakShare ?? 0,
      ownLine: p?.shape?.ownLine ?? false,
      example: example(p?.shape?.example),
    },
    openings: {
      unknown: p?.openings?.unknown ?? true,
      openings: [...(p?.openings?.openings ?? [])],
      closings: [...(p?.openings?.closings ?? [])],
      example: example(p?.openings?.example),
    },
    adverbs: {
      unknown: p?.adverbs?.unknown ?? true,
      none: p?.adverbs?.none ?? false,
      words:
        p?.adverbs?.words.map((word) => ({ word: word.word, perHundred: word.perHundred })) ?? [],
      example: example(p?.adverbs?.example),
    },
    person: {
      unknown: p?.person?.unknown ?? true,
      jeo: p?.person?.jeo ?? 0,
      uri: p?.person?.uri ?? 0,
      na: p?.person?.na ?? 0,
      dominant: p?.person?.dominant ?? '',
      example: example(p?.person?.example),
    },
    headings: {
      unknown: p?.headings?.unknown ?? true,
      count: p?.headings?.count ?? 0,
      emojiShare: p?.headings?.emojiShare ?? 0,
      questionShare: p?.headings?.questionShare ?? 0,
      numberedShare: p?.headings?.numberedShare ?? 0,
      listShare: p?.headings?.listShare ?? 0,
      marker: p?.headings?.marker ?? '',
      example: example(p?.headings?.example),
    },
  }
}

export function toVoiceAnalysis(analysis: ProtoVoiceAnalysis): VoiceAnalysis {
  return {
    counted: toFingerprint(analysis.counted),
    ai: {
      impression: analysis.ai?.impression ?? '',
      tics: analysis.ai?.tics.map((tic) => ({ phrase: tic.phrase, when: tic.when })) ?? [],
      signaturePhrases: [...(analysis.ai?.signaturePhrases ?? [])],
      examples:
        analysis.ai?.examples.map((cited) => ({
          field: requireAiField(cited.field),
          sentence: cited.sentence,
          materialId: cited.materialId,
        })) ?? [],
    },
    materialCount: analysis.materialCount,
    analyzeModel: analysis.analyzeModel,
    createdAt: analysis.createdAt,
  }
}

const toNotice = (notice: ProtoVoiceNotice | undefined): VoiceNotice => ({
  kind: requireNoticeKind(notice?.kind),
  count: notice?.count ?? 0,
})

export function toVoiceProfile(profile: ProtoVoiceProfile | undefined): VoiceProfile {
  return {
    voice: toVoice(profile?.voice),
    made: profile?.made ?? false,
    readiness: toReadiness(profile?.readiness),
    samples: profile?.samples.map(toVoiceSample) ?? [],
    activeJobId: profile?.activeJobId ?? '',
    ...(profile?.analysis ? { analysis: toVoiceAnalysis(profile.analysis) } : {}),
    hasPrevious: profile?.hasPrevious ?? false,
    notice: toNotice(profile?.notice),
  }
}

// Every key carries the account AND the voice (VOICE-56): two voices of one
// account are different aggregates that may contradict each other, and an account switch on the
// same device must never read the previous account's entry. The directory itself is per account.
export function voicesQueryKey(transport: Transport, ownerId: string) {
  return ['voices', transport, ownerId] as const
}
/** The voice as its tabs read it: the analysis, the 학습 글 list and the meter (VOICE-56). */
export function voiceAnalysisQueryKey(transport: Transport, ownerId: string, voiceId: string) {
  return ['voice-analysis', transport, ownerId, voiceId] as const
}
/** ②'s fingerprint comparison of one post at one content revision (POST-102). */
export function postFingerprintQueryKey(
  transport: Transport,
  ownerId: string,
  slug: string,
  revision: bigint,
) {
  return ['voice-post-fingerprint', transport, ownerId, slug, String(revision)] as const
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

/** The analysis entry as a cache target, for a caller that has to say "this job's completion
 *  makes that analysis stale" without holding a transport of its own (ARCH-17). */
export function useVoiceAnalysisQueryKey(ownerId: string, voiceId: string) {
  const transport = useTransport()
  return useMemo(
    () => voiceAnalysisQueryKey(transport, ownerId, voiceId),
    [ownerId, transport, voiceId],
  )
}
