// Query keys and the proto↔domain mappers for the memory entity.
import type { Transport } from '@connectrpc/connect'
import { ProtoMemoryKind, type ProtoMemory, type ProtoMemoryCandidate } from '@/shared/api'
import type { Memory, MemoryKind } from '../model/types'

const KIND_TO_PROTO: Record<MemoryKind, ProtoMemoryKind> = {
  preference: ProtoMemoryKind.PREFERENCE,
  persona: ProtoMemoryKind.PERSONA,
  place: ProtoMemoryKind.PLACE,
  person: ProtoMemoryKind.PERSON,
  history: ProtoMemoryKind.HISTORY,
}

const PROTO_TO_KIND = new Map<ProtoMemoryKind, MemoryKind>(
  Object.entries(KIND_TO_PROTO).map(([kind, wire]) => [wire, kind as MemoryKind]),
)

export function fromProtoKind(kind: ProtoMemoryKind): MemoryKind {
  // A row can only hold one of the five — the column's CHECK says so — so an unset value here
  // is an older server, and `preference` is the one reading that changes nothing about
  // retrieval's tag gate.
  return PROTO_TO_KIND.get(kind) ?? 'preference'
}

export function toProtoKind(kind: MemoryKind): ProtoMemoryKind {
  return KIND_TO_PROTO[kind]
}

export function toMemory(memory: ProtoMemory): Memory {
  return {
    id: memory.id,
    text: memory.text,
    kind: fromProtoKind(memory.kind),
    tags: [...memory.tags],
    sourcePostSlugs: [...memory.sourcePostSlugs],
    createdAt: memory.createdAt,
    updatedAt: memory.updatedAt,
    lastSeenAt: memory.lastSeenAt,
  }
}

/** One proposed fact from an extraction. It is not a memory and nothing stores it: the user
 *  checks the ones they want and each becomes an ordinary create (MEM-15). */
export interface MemoryCandidate {
  text: string
  kind: MemoryKind
  tags: string[]
}

export function toMemoryCandidate(candidate: ProtoMemoryCandidate): MemoryCandidate {
  return { text: candidate.text, kind: fromProtoKind(candidate.kind), tags: [...candidate.tags] }
}

/** Per account, like every other directory: an account switch on the same device must never
 *  read the previous account's facts. */
export function memoriesQueryKey(transport: Transport, ownerId: string) {
  return ['memories', transport, ownerId] as const
}
