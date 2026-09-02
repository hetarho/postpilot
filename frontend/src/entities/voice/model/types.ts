import i18next from 'i18next'
import type { ContentLanguage, PostContent } from '@/shared/api'

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
export type VoiceRuleLayer = 'lexical' | 'endings' | 'syntax' | 'structure' | 'axes' | 'unknown'
export interface VoiceRule {
  id: string
  statement: string
  layer: VoiceRuleLayer
  evidenceCount: number
  status: 'candidate' | 'active' | 'retired' | 'rejected' | 'unknown'
  origin: string
  createdAt: string
  lastEvidenceAt: string
}
export interface VoiceSource {
  id: string
  postSlug: string
  title: string
  tags: string[]
  excerpt: string
  hasEmbedding: boolean
  createdAt: string
}
export interface VoiceFeedback {
  id: string
  postSlug: string
  kind: string
  layer: VoiceRuleLayer
  processingState: string
  createdAt: string
}
export type VoiceValidationState = 'queued' | 'running' | 'partial' | 'failed' | 'done' | 'unknown'

export function voiceValidationState(value: string): VoiceValidationState {
  switch (value) {
    case 'queued':
    case 'running':
    case 'partial':
    case 'failed':
    case 'done':
      return value
    default:
      return 'unknown'
  }
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
  rules: VoiceRule[]
  sources: VoiceSource[]
  feedback: VoiceFeedback[]
}

/** One of an account's writing voices (spec/policy/voice.md). A voice owns exactly one profile and
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
}

/** The voice a post is written in, as a post screen needs it — just enough to name it, including
 *  after the voice is deleted, since the post stays readable and exportable. */
export interface VoiceRef {
  id: string
  name: string
  deleted: boolean
  sourceLanguage: ContentLanguage | undefined
}

export interface VoiceProfile {
  voice: Voice
  updatedAt: string
  samples: VoiceSample[]
  activeJobId: string
  structured: StructuredVoiceProfile
  finalizedSourceCount: number
  canValidate: boolean
}
export interface VoiceVersion {
  version: bigint
  profile: StructuredVoiceProfile
  origin: string
  restoredFromVersion: bigint
  createdAt: string
  /** Whether this version carries a generation snapshot that can be previewed. Presence only —
   *  the snapshot is fetched per version, when the row is opened (change 16). */
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
    rules: [],
    sources: [],
    feedback: [],
  }
}

// An empty profile is now exactly "nothing to learn from and nothing published": the free-text
// styleguide that used to count as content is gone (change 16).
export function isEmptyProfile(
  profile: Pick<VoiceProfile, 'structured' | 'samples' | 'finalizedSourceCount'>,
): boolean {
  return (
    profile.structured.empty && profile.samples.length === 0 && profile.finalizedSourceCount === 0
  )
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

export function voiceContentLanguageMismatch(
  contentLanguage: ContentLanguage | undefined,
  sourceLanguage: ContentLanguage | undefined,
): boolean {
  return Boolean(contentLanguage && sourceLanguage && contentLanguage !== sourceLanguage)
}

export function voiceContentLanguageMismatchReason(): string {
  return i18next.t('VOICE_CONTENT_LANGUAGE_MISMATCH', { ns: 'errors' })
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
