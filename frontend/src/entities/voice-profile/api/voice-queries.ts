import type { Transport } from '@connectrpc/connect'
import { VoiceLayer, VoiceRuleStatus, VoiceValueSource, type ProtoVoiceProfile, type ProtoVoiceSample, type StructuredVoiceProfile as ProtoStructured, type VoiceProfileVersion as ProtoVersion } from '@/shared/api'
import type { StructuredVoiceProfile, VoiceProfile, VoiceSample, VoiceSourceKind, VoiceVersion } from '../model/types'

const layer = (value: VoiceLayer): string => value === VoiceLayer.LEXICAL ? 'lexical' : value === VoiceLayer.ENDINGS ? 'endings' : value === VoiceLayer.SYNTAX ? 'syntax' : value === VoiceLayer.STRUCTURE ? 'structure' : value === VoiceLayer.AXES ? 'axes' : 'unknown'
const status = (value: VoiceRuleStatus): 'candidate' | 'active' | 'retired' | 'rejected' | 'unknown' => value === VoiceRuleStatus.CANDIDATE ? 'candidate' : value === VoiceRuleStatus.ACTIVE ? 'active' : value === VoiceRuleStatus.RETIRED ? 'retired' : value === VoiceRuleStatus.REJECTED ? 'rejected' : 'unknown'
const source = (value: VoiceValueSource): VoiceSourceKind => value === VoiceValueSource.MEASURED ? 'measured' : value === VoiceValueSource.ANALYZED ? 'analyzed' : value === VoiceValueSource.MANUAL ? 'manual' : 'unknown'
const voiceValue = (value: { value: string; source: VoiceValueSource; unknown: boolean } | undefined) => ({ value: value?.value ?? '', source: source(value?.source ?? VoiceValueSource.UNKNOWN), unknown: value?.unknown ?? true })

export function toVoiceSample(sample: ProtoVoiceSample): VoiceSample { return { id: sample.id, label: sample.label, chars: sample.chars, createdAt: sample.createdAt } }
export function toStructured(p: ProtoStructured | undefined): StructuredVoiceProfile {
  return {
    version: p?.meta?.version ?? 0n, updatedAt: p?.meta?.updatedAt ?? '', sourceCount: p?.meta?.sourceCount ?? 0, empty: p?.empty ?? true,
    lexical: { description: voiceValue(p?.lexical?.description), preferredWords: p?.lexical?.preferredWords.map((v) => ({ word: v.word, alternatives: [...v.alternatives], weight: v.weight })) ?? [], bannedWords: p?.lexical?.bannedWords.map((v) => ({ value: v.value, reason: v.reason })) ?? [], bannedPatterns: p?.lexical?.bannedPatterns.map((v) => ({ value: v.value, reason: v.reason })) ?? [] },
    endings: { baseRegister: voiceValue(p?.endings?.baseRegister), distribution: p?.endings?.distribution.map((v) => ({ ending: v.ending, ratio: v.ratio })) ?? [], bannedEndings: [...(p?.endings?.bannedEndings ?? [])], signatureEndings: [...(p?.endings?.signatureEndings ?? [])], constraints: [...(p?.endings?.constraints ?? [])] },
    syntax: { averageSentenceChars: p?.syntax?.averageSentenceChars ?? 0, sentenceLength: voiceValue(p?.syntax?.sentenceLength), connectiveStyle: voiceValue(p?.syntax?.connectiveStyle), preferredConnectives: [...(p?.syntax?.preferredConnectives ?? [])], nominalization: voiceValue(p?.syntax?.nominalization), passiveTendency: voiceValue(p?.syntax?.passiveTendency) },
    structure: { introPattern: voiceValue(p?.structure?.introPattern), closingPattern: voiceValue(p?.structure?.closingPattern), paragraphSentencesMin: p?.structure?.paragraphSentencesMin ?? 0, paragraphSentencesMax: p?.structure?.paragraphSentencesMax ?? 0, headingHabit: voiceValue(p?.structure?.headingHabit), listHabit: voiceValue(p?.structure?.listHabit), emojiUse: voiceValue(p?.structure?.emojiUse) },
    // No `?? 0` here: the wire carries axis presence, and collapsing absence into 0 is exactly
    // the bug this screen used to show — a neutral measurement the model never made.
    axes: { involvement: p?.axes?.involvement, narrativity: p?.axes?.narrativity, persuasionOvertness: p?.axes?.persuasionOvertness, abstractness: p?.axes?.abstractness, addresseeFocus: p?.axes?.addresseeFocus, humor: p?.axes?.humor },
    rules: p?.contrastRules.map((v) => ({ id: v.id, statement: v.statement, layer: layer(v.layer), evidenceCount: v.evidenceCount, status: status(v.status), origin: v.origin, createdAt: v.createdAt, lastEvidenceAt: v.lastEvidenceAt })) ?? [],
    sources: p?.fewShotBank.map((v) => ({ id: v.id, postSlug: v.postSlug, title: v.title, tags: [...v.tags], excerpt: v.excerpt, hasEmbedding: v.hasEmbedding, createdAt: v.createdAt })) ?? [],
    feedback: p?.feedbackLog.map((v) => ({ id: v.id, postSlug: v.postSlug, kind: v.kind, layer: layer(v.layer), processingState: v.processingState, createdAt: v.createdAt })) ?? [],
  }
}
export function toVoiceProfile(profile: ProtoVoiceProfile | undefined): VoiceProfile {
  return { styleguide: profile?.styleguide ?? '', rules: profile?.rules ?? '', legacyManualGuidance: profile?.legacyManualGuidance ?? '', updatedAt: profile?.updatedAt ?? '', samples: profile?.samples.map(toVoiceSample) ?? [], activeJobId: profile?.activeJobId ?? '', structured: toStructured(profile?.structured), finalizedSourceCount: profile?.finalizedSourceCount ?? 0, canValidate: profile?.canValidate ?? false }
}
export function toVoiceVersion(version: ProtoVersion): VoiceVersion { return { version: version.version, profile: toStructured(version.profile), origin: version.origin, restoredFromVersion: version.restoredFromVersion, createdAt: version.createdAt } }
export function voiceProfileQueryKey(transport: Transport, ownerId: string) { return ['voice-profile', transport, ownerId] as const }
export function voiceVersionsQueryKey(transport: Transport, ownerId: string) { return ['voice-versions', transport, ownerId] as const }
export function voiceConfirmationsQueryKey(transport: Transport, ownerId: string) { return ['voice-confirmations', transport, ownerId] as const }
export function voiceValidationsQueryKey(transport: Transport, ownerId: string) { return ['voice-validations', transport, ownerId] as const }
export function voiceComparisonQueryKey(transport: Transport, ownerId: string, id: string) { return ['voice-rule-comparison', transport, ownerId, id] as const }
export function voiceValidationQueryKey(transport: Transport, ownerId: string, id: string) { return ['voice-validation', transport, ownerId, id] as const }
