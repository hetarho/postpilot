import {
  ProtoFingerprintFacetUnit,
  ProtoFingerprintItem,
  VoiceAiField as ProtoAiField,
  VoiceNoticeKind as ProtoNoticeKind,
  VoicePromptPart as ProtoPart,
  VoiceSampleKind as ProtoKind,
} from '@/shared/api'
import type { VoiceAiField, VoiceNotice, VoicePromptPart, VoiceSampleKind } from '../model/types'
import type { FingerprintFacetUnit, FingerprintItem } from '../model/fingerprint'

const PART_FROM_PROTO = new Map<ProtoPart, VoicePromptPart>([
  [ProtoPart.OPENING, 'opening'],
  [ProtoPart.DESCRIPTION, 'description'],
  [ProtoPart.CLOSING, 'closing'],
])

const KIND_FROM_PROTO = new Map<ProtoKind, VoiceSampleKind>([
  [ProtoKind.POST, 'post'],
  [ProtoKind.ANSWER, 'answer'],
])

/** A part this build does not know fails the read rather than landing in a group (ARCH-3). */
export function requirePromptPart(value: ProtoPart): VoicePromptPart {
  const part = PART_FROM_PROTO.get(value)
  if (!part) throw new Error(`unsupported voice prompt part enum: ${String(value)}`)
  return part
}

/** The same for a 학습 글's kind. */
export function requireSampleKind(value: ProtoKind): VoiceSampleKind {
  const kind = KIND_FROM_PROTO.get(value)
  if (!kind) throw new Error(`unsupported voice sample kind enum: ${String(value)}`)
  return kind
}

const AI_FIELD_FROM_PROTO = new Map<ProtoAiField, VoiceAiField>([
  [ProtoAiField.IMPRESSION, 'impression'],
  [ProtoAiField.TICS, 'tics'],
  [ProtoAiField.SIGNATURE_PHRASES, 'signature_phrases'],
])

const NOTICE_FROM_PROTO = new Map<ProtoNoticeKind, VoiceNotice['kind']>([
  [ProtoNoticeKind.UNSPECIFIED, 'none'],
  [ProtoNoticeKind.ADDED, 'added'],
  [ProtoNoticeKind.CHANGED, 'changed'],
])

/** The field an AI example shows; one this build does not know fails the read (ARCH-3). */
export function requireAiField(value: ProtoAiField): VoiceAiField {
  const field = AI_FIELD_FROM_PROTO.get(value)
  if (!field) throw new Error(`unsupported voice AI field enum: ${String(value)}`)
  return field
}

/** The notice kind; an unset notice is none. */
export function requireNoticeKind(value: ProtoNoticeKind | undefined): VoiceNotice['kind'] {
  const kind = NOTICE_FROM_PROTO.get(value ?? ProtoNoticeKind.UNSPECIFIED)
  if (!kind) throw new Error(`unsupported voice notice enum: ${String(value)}`)
  return kind
}

const ITEM_FROM_PROTO = new Map<ProtoFingerprintItem, FingerprintItem>([
  [ProtoFingerprintItem.ENDINGS, 'endings'],
  [ProtoFingerprintItem.MARKS, 'marks'],
  [ProtoFingerprintItem.EMOJI, 'emoji'],
  [ProtoFingerprintItem.SHAPE, 'shape'],
  [ProtoFingerprintItem.OPENINGS, 'openings'],
  [ProtoFingerprintItem.ADVERBS, 'adverbs'],
  [ProtoFingerprintItem.PERSON, 'person'],
  [ProtoFingerprintItem.HEADINGS, 'headings'],
])

const UNIT_FROM_PROTO = new Map<ProtoFingerprintFacetUnit, FingerprintFacetUnit>([
  [ProtoFingerprintFacetUnit.SHARE, 'share'],
  [ProtoFingerprintFacetUnit.PER_HUNDRED, 'per_hundred'],
  [ProtoFingerprintFacetUnit.CHARS, 'chars'],
  [ProtoFingerprintFacetUnit.SENTENCES, 'sentences'],
  [ProtoFingerprintFacetUnit.TEXT, 'text'],
])

/** A counted item this build does not know fails the read rather than a row with no name. */
export function requireFingerprintItem(value: ProtoFingerprintItem): FingerprintItem {
  const item = ITEM_FROM_PROTO.get(value)
  if (!item) throw new Error(`unsupported fingerprint item enum: ${String(value)}`)
  return item
}

/** The same for a facet's unit. */
export function requireFacetUnit(value: ProtoFingerprintFacetUnit): FingerprintFacetUnit {
  const unit = UNIT_FROM_PROTO.get(value)
  if (!unit) throw new Error(`unsupported fingerprint facet unit enum: ${String(value)}`)
  return unit
}
