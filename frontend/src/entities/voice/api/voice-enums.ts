import { VoicePromptPart as ProtoPart, VoiceSampleKind as ProtoKind } from '@/shared/api'
import type { VoicePromptPart, VoiceSampleKind } from '../model/types'

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
