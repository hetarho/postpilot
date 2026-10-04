import type { ProtoSpokenDraft, ProtoSpokenVoice, ProtoSpokenProfileSnapshot } from '@/shared/api'
import type { SpokenDraft, SpokenVoice, SpokenProfile, SpokenPhase } from '../model/types'

function profile(p: ProtoSpokenProfileSnapshot | undefined): SpokenProfile {
  if (!p || !p.id || p.revision <= 0n) throw new Error('Missing spoken profile snapshot')
  const grade =
    p.grade === 'free' ||
    p.grade === 'value' ||
    p.grade === 'balanced' ||
    p.grade === 'premium' ||
    p.grade === 'top'
      ? p.grade
      : null
  return {
    id: p.id,
    revision: p.revision,
    providerId: p.providerId,
    designModelId: p.designModelId,
    speechModelId: p.speechModelId,
    designLabel: p.designLabel,
    speechLabel: p.speechLabel,
    grade,
    descriptionMax: p.descriptionMax,
    previewMax: p.previewMax,
    speechMax: p.speechMax,
    outputFormat: p.outputFormat,
  }
}
function phase(value: string): SpokenPhase {
  if (
    value === 'editing' ||
    value === 'candidates' ||
    value === 'selected' ||
    value === 'confirmed'
  )
    return value
  throw new Error('Unknown spoken draft phase')
}
export function toSpokenDraft(d: ProtoSpokenDraft | undefined): SpokenDraft {
  if (!d) throw new Error('Missing spoken draft')
  return {
    id: d.id,
    revision: d.revision,
    name: d.name,
    description: d.description,
    previewText: d.previewText,
    profile: profile(d.profile),
    phase: phase(d.phase),
    candidates: d.candidates.map((c) => ({
      id: c.id,
      assetId: c.assetId,
      durationMs: Number(c.durationMs),
      auditionedAt: c.auditionedAt,
    })),
    selectedCandidateId: d.selectedCandidateId,
    confirmedVoiceId: d.confirmedVoiceId,
    qualificationSessionId: d.qualificationSessionId,
    createdAt: d.createdAt,
    updatedAt: d.updatedAt,
  }
}
export function toSpokenVoice(v: ProtoSpokenVoice | undefined): SpokenVoice {
  if (!v) throw new Error('Missing spoken voice')
  return {
    id: v.id,
    revision: v.revision,
    name: v.name,
    description: v.description,
    previewText: v.previewText,
    profile: profile(v.profile),
    sampleAssetId: v.sampleAssetId,
    sampleDurationMs: Number(v.sampleDurationMs),
    createdAt: v.createdAt,
    removedAt: v.removedAt,
  }
}
