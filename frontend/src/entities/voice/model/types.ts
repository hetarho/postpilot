import i18next from 'i18next'
import type { AppFailure, ContentLanguage, PostContent } from '@/shared/api'

export interface VoiceSample {
  id: string
  label: string
  chars: number
  createdAt: string
}
export type VoiceSourceKind = 'unknown' | 'measured' | 'analyzed' | 'manual'
export interface VoiceValue {
  value: string
  source: VoiceSourceKind
  unknown: boolean
}
/** Every axis is optional because absence is a real answer: an axis the analysis never measured is
 *  missing, not 0, and the screen shows it as 알 수 없음 next to the other unknown-capable fields. */
export interface VoiceAxes {
  involvement?: number
  narrativity?: number
  persuasionOvertness?: number
  abstractness?: number
  addresseeFocus?: number
  humor?: number
}
export interface StructuredVoiceProfile {
  version: bigint
  updatedAt: string
  sourceCount: number
  empty: boolean
  lexical: {
    description: VoiceValue
    preferredWords: Array<{ word: string; alternatives: string[]; weight: number }>
    bannedWords: Array<{ value: string; reason: string }>
    bannedPatterns: Array<{ value: string; reason: string }>
  }
  endings: {
    baseRegister: VoiceValue
    distribution: Array<{ ending: string; ratio: number }>
    bannedEndings: string[]
    signatureEndings: string[]
    constraints: string[]
  }
  syntax: {
    averageSentenceChars: number
    averageSentenceWords?: number
    sentenceLength: VoiceValue
    connectiveStyle: VoiceValue
    preferredConnectives: string[]
    nominalization: VoiceValue
    passiveTendency: VoiceValue
  }
  structure: {
    introPattern: VoiceValue
    closingPattern: VoiceValue
    paragraphSentencesMin: number
    paragraphSentencesMax: number
    headingHabit: VoiceValue
    listHabit: VoiceValue
    emojiUse: VoiceValue
  }
  axes: VoiceAxes
}

/** One of an account's writing voices (VOICE-1). A voice owns exactly one profile and
 *  every row that can change it. Deleting one leaves a tombstone rather than a hole: the posts
 *  written in it still name it, so `deleted` travels with the voice everywhere it is shown. */
export interface Voice {
  id: string
  name: string
  isDefault: boolean
  deleted: boolean
  createdAt: string
  updatedAt: string
  deletedAt: string
  sourceLanguage: ContentLanguage
  /** Whether the voice has a published analysis. Only a made voice can be assigned to a post or
   *  write one; one not yet made is still in the directory, 만드는 중 (POST-101). */
  made: boolean
}

/** The voice a post is written in, as a post screen needs it — just enough to name it, including
 *  after the voice is deleted, since the post stays readable and exportable. A post with 말투 없음
 *  has none (POST-25). */
export interface VoiceRef {
  id: string
  name: string
  deleted: boolean
  /** A voice not yet made refuses every AI action the way a deleted one does (POST-25). */
  made: boolean
  sourceLanguage: ContentLanguage | undefined
}

export interface VoiceProfile {
  voice: Voice
  updatedAt: string
  samples: VoiceSample[]
  activeJobId: string
  structured: StructuredVoiceProfile
  /** Why the seeding a described creation started failed, while the voice has no published
   *  version yet (VOICE-19). Stays after the job ended and after a reload. */
  seedFailure?: AppFailure
}
export interface VoiceVersion {
  version: bigint
  profile: StructuredVoiceProfile
  origin: string
  restoredFromVersion: bigint
  createdAt: string
  /** Whether this version carries a generation snapshot that can be previewed. Presence only —
   *  the snapshot is fetched per version, when the row is opened (VOICE-30). */
  hasSample: boolean
}

/** A copy of the raw AI output of the last post one profile version produced. It is what makes
 *  a version readable BEFORE it is adopted; a version that never produced a post has none. */
export interface VoiceVersionSample {
  content: PostContent
  createdAt: string
}

export function emptyVoice(): Voice {
  return {
    id: '',
    name: '',
    isDefault: false,
    deleted: false,
    createdAt: '',
    updatedAt: '',
    deletedAt: '',
    sourceLanguage: 'ko',
    made: false,
  }
}

const unknownValue = (): VoiceValue => ({ value: '', source: 'unknown', unknown: true })
export function emptyStructuredVoiceProfile(): StructuredVoiceProfile {
  return {
    version: 0n,
    updatedAt: '',
    sourceCount: 0,
    empty: true,
    lexical: {
      description: unknownValue(),
      preferredWords: [],
      bannedWords: [],
      bannedPatterns: [],
    },
    endings: {
      baseRegister: unknownValue(),
      distribution: [],
      bannedEndings: [],
      signatureEndings: [],
      constraints: [],
    },
    syntax: {
      averageSentenceChars: 0,
      averageSentenceWords: undefined,
      sentenceLength: unknownValue(),
      connectiveStyle: unknownValue(),
      preferredConnectives: [],
      nominalization: unknownValue(),
      passiveTendency: unknownValue(),
    },
    structure: {
      introPattern: unknownValue(),
      closingPattern: unknownValue(),
      paragraphSentencesMin: 0,
      paragraphSentencesMax: 0,
      headingHabit: unknownValue(),
      listHabit: unknownValue(),
      emojiUse: unknownValue(),
    },
    axes: {},
  }
}

/** The value standing for 말투 없음 wherever a post's voice is chosen. Empty, because that is
 *  exactly what the wire carries to clear an assignment (a present empty `voice_id`, POST-24). */
export const NO_VOICE_VALUE = ''

/** How 말투 없음 is written wherever it can be chosen or confirmed. */
export function noVoiceLabel(): string {
  return i18next.t('noVoice', { ns: 'voices' })
}

export function voiceRefLabel(voice: Pick<VoiceRef, 'name' | 'deleted'>): string {
  return voice.deleted ? i18next.t('deletedRef', { ns: 'voices', name: voice.name }) : voice.name
}

/** Why every AI action on a deleted-voice post is unavailable. One string, so generate, revise and
 *  finalize cannot explain the same server rule three different ways. The server enforces it;
 *  this only says so before the round trip. */
export function deletedVoiceAIReason(): string {
  return i18next.t('deletedAiReason', { ns: 'voices' })
}

/** The same for a voice not yet made: every AI action waits until it is, or until the post is
 *  moved to another voice (POST-25). */
export function unmadeVoiceAIReason(): string {
  return i18next.t('unmadeAiReason', { ns: 'voices' })
}

/** Why a post's voice refuses AI work, or '' when it does not: a deleted voice first, since
 *  restoring it is the way out even when it was never made, then one not yet made. A post with
 *  말투 없음 writes with no voice, so nothing is refused (GEN-25). */
export function voiceAIRefusal(voice: Pick<VoiceRef, 'deleted' | 'made'> | undefined): string {
  if (!voice) return ''
  if (voice.deleted) return deletedVoiceAIReason()
  if (!voice.made) return unmadeVoiceAIReason()
  return ''
}

export function activeVoices<T extends Pick<Voice, 'deleted'>>(voices: readonly T[]): T[] {
  return voices.filter((voice) => !voice.deleted)
}

export function deletedVoices<T extends Pick<Voice, 'deleted'>>(voices: readonly T[]): T[] {
  return voices.filter((voice) => voice.deleted)
}

export function defaultVoice<T extends Pick<Voice, 'deleted' | 'isDefault'>>(
  voices: readonly T[],
): T | undefined {
  return voices.find((voice) => voice.isDefault && !voice.deleted)
}

/** The server's directory order — active before deleted, the default first, then by name and id —
 *  re-applied after a cache patch so an inserted or renamed voice lands where a refetch would put
 *  it. Plain string comparison, not a locale collation, because that is what SQLite's ORDER BY did. */
export function sortVoices<T extends Pick<Voice, 'id' | 'name' | 'isDefault' | 'deleted'>>(
  voices: readonly T[],
): T[] {
  return [...voices].sort(
    (a, b) =>
      Number(a.deleted) - Number(b.deleted) ||
      Number(b.isDefault) - Number(a.isDefault) ||
      compare(a.name, b.name) ||
      compare(a.id, b.id),
  )
}

const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0)
