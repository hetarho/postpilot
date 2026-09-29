import i18next from 'i18next'

/** What a 학습 글 is: a post the owner wrote by hand and pasted, or an answer to one of the shared
 *  prompts (VOICE-59). */
export type VoiceSampleKind = 'post' | 'answer'

/** One 학습 글 as the list shows it: a post by its label, an answer by its prompt. */
export interface VoiceSample {
  id: string
  kind: VoiceSampleKind
  /** Empty for an answer, whose prompt text is product copy. */
  label: string
  promptKey: string
  hasPhoto: boolean
  chars: number
  createdAt: string
}

/** One 학습 글 opened: its full text and, for a photo answer, a view URL minted on that read. */
export interface VoiceSampleDetail {
  sample: VoiceSample
  body: string
  photoUrl: string
  photoWidth: number
  photoHeight: number
}

/** Where in a post a prompt's answer belongs (VOICE-60). */
export type VoicePromptPart = 'opening' | 'description' | 'closing'

/** One of the shared prompts every voice answers. The text is product copy, Korean in both
 *  locales (LANG-14). */
export interface VoicePrompt {
  key: string
  part: VoicePromptPart
  photo: boolean
  text: string
}

/** How far the 학습 글 are from 말투 만들기 (VOICE-32): the sentence share, held below 100 while a
 *  part is missing, which `missingParts` names. */
export interface VoiceReadiness {
  percent: number
  sentences: number
  needed: number
  missingParts: VoicePromptPart[]
}
/** The sentence an item is shown with and the 학습 글 it came from ('' for a measured text). */
export interface VoiceExample {
  sentence: string
  materialId: string
}

/** The fingerprint's eight counted items (VOICE-24), each in its own unit: shares are 0…1,
 *  rates are per 100 sentences, and an item below its threshold is unknown rather than 0. */
export interface VoiceFingerprint {
  sentences: number
  endings: {
    unknown: boolean
    da: number
    haeyo: number
    seumnida: number
    other: number
    suffixes: Array<{ text: string; count: number }>
    example?: VoiceExample
  }
  marks: {
    unknown: boolean
    exclaim: number
    question: number
    tilde: number
    ellipsis: number
    period: number
    none: number
    repeat: number
    example?: VoiceExample
  }
  emoji: {
    unknown: boolean
    emoji: number
    hh: number
    kk: number
    tears: number
    example?: VoiceExample
  }
  shape: {
    unknown: boolean
    averageChars: number
    paragraphAverage: number
    paragraphMin: number
    paragraphMax: number
    lineBreakShare: number
    ownLine: boolean
    example?: VoiceExample
  }
  openings: { unknown: boolean; openings: string[]; closings: string[]; example?: VoiceExample }
  adverbs: {
    unknown: boolean
    /** A real answer: no lexicon word repeats. */
    none: boolean
    words: Array<{ word: string; perHundred: number }>
    example?: VoiceExample
  }
  person: {
    unknown: boolean
    jeo: number
    uri: number
    na: number
    /** 저, 우리 or 나; '' when no form is frequent enough. */
    dominant: string
    example?: VoiceExample
  }
  headings: {
    unknown: boolean
    count: number
    emojiShare: number
    questionShare: number
    numberedShare: number
    listShare: number
    marker: string
    example?: VoiceExample
  }
}

/** Which part of the AI's reading an example shows. */
export type VoiceAiField = 'impression' | 'tics' | 'signature_phrases'

/** What the analysis call wrote: only what cannot be counted (VOICE-24). */
export interface VoiceAiPart {
  impression: string
  tics: Array<{ phrase: string; when: string }>
  signaturePhrases: string[]
  examples: Array<{ field: VoiceAiField; sentence: string; materialId: string }>
}

/** One analysis (VOICE-26). */
export interface VoiceAnalysis {
  counted: VoiceFingerprint
  ai: VoiceAiPart
  materialCount: number
  analyzeModel: string
  createdAt: string
}

/** How the 학습 글 moved since the current analysis read them (VOICE-21). */
export interface VoiceNotice {
  kind: 'none' | 'added' | 'changed'
  count: number
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
  /** Whether the voice has a published analysis. Only a made voice can be assigned to a post or
   *  write one; one not yet made is still in the directory, 만드는 중 (POST-101). */
  made: boolean
  /** The directory row's meta line (VOICE-52): how many 학습 글 the voice holds, and when its
   *  current analysis was published ('' until it is made). */
  materialCount: number
  analyzedAt: string
  /** The readiness meter's share until the voice is made (VOICE-9); 0 once it is. */
  readinessPercent: number
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
}

export interface VoiceProfile {
  voice: Voice
  /** Whether an analysis is published (VOICE-25). */
  made: boolean
  readiness: VoiceReadiness
  samples: VoiceSample[]
  activeJobId: string
  /** The current analysis, absent until the voice is made. */
  analysis?: VoiceAnalysis
  /** Whether 이전 분석으로 되돌리기 has something to return to (VOICE-30). */
  hasPrevious: boolean
  notice: VoiceNotice
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
    made: false,
    materialCount: 0,
    analyzedAt: '',
    readinessPercent: 0,
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

/** The day a voice's current analysis was published, as its directory row writes it:
 *  `YYYY.MM.DD` in the viewer's own calendar (VOICE-52). '' for an unparsable or absent time. */
export function voiceAnalysisDate(value: string): string {
  const date = new Date(value)
  if (!value || Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}.${pad(date.getMonth() + 1)}.${pad(date.getDate())}`
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
