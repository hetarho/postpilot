export interface VoiceSample { id: string; label: string; chars: number; createdAt: string }
export type VoiceSourceKind = 'unknown' | 'measured' | 'analyzed' | 'manual'
export interface VoiceValue { value: string; source: VoiceSourceKind; unknown: boolean }
export interface VoiceRule {
  id: string; statement: string; layer: string; evidenceCount: number
  status: 'candidate' | 'active' | 'retired' | 'rejected' | 'unknown'
  origin: string; createdAt: string; lastEvidenceAt: string
}
export interface VoiceSource { id: string; postSlug: string; title: string; tags: string[]; excerpt: string; hasEmbedding: boolean; createdAt: string }
export interface VoiceFeedback { id: string; postSlug: string; kind: string; layer: string; processingState: string; createdAt: string }
/** Every axis is optional because absence is a real answer: an axis the analysis never measured is
 *  missing, not 0, and the screen shows it as 알 수 없음 next to the other unknown-capable fields. */
export interface VoiceAxes { involvement?: number; narrativity?: number; persuasionOvertness?: number; abstractness?: number; addresseeFocus?: number; humor?: number }
export interface StructuredVoiceProfile {
  version: bigint; updatedAt: string; sourceCount: number; empty: boolean
  lexical: { description: VoiceValue; preferredWords: Array<{ word: string; alternatives: string[]; weight: number }>; bannedWords: Array<{ value: string; reason: string }>; bannedPatterns: Array<{ value: string; reason: string }> }
  endings: { baseRegister: VoiceValue; distribution: Array<{ ending: string; ratio: number }>; bannedEndings: string[]; signatureEndings: string[]; constraints: string[] }
  syntax: { averageSentenceChars: number; sentenceLength: VoiceValue; connectiveStyle: VoiceValue; preferredConnectives: string[]; nominalization: VoiceValue; passiveTendency: VoiceValue }
  structure: { introPattern: VoiceValue; closingPattern: VoiceValue; paragraphSentencesMin: number; paragraphSentencesMax: number; headingHabit: VoiceValue; listHabit: VoiceValue; emojiUse: VoiceValue }
  axes: VoiceAxes
  rules: VoiceRule[]; sources: VoiceSource[]; feedback: VoiceFeedback[]
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
}

/** The voice a post is written in, as a post screen needs it — just enough to name it, including
 *  after the voice is deleted, since the post stays readable and exportable. */
export interface VoiceRef { id: string; name: string; deleted: boolean }

export interface VoiceProfile {
  voice: Voice
  styleguide: string; rules: string; legacyManualGuidance: string; updatedAt: string
  samples: VoiceSample[]; activeJobId: string; structured: StructuredVoiceProfile
  finalizedSourceCount: number; canValidate: boolean
}
export interface VoiceVersion { version: bigint; profile: StructuredVoiceProfile; origin: string; restoredFromVersion: bigint; createdAt: string }

export function emptyVoice(): Voice {
  return { id: '', name: '', isDefault: false, deleted: false, createdAt: '', updatedAt: '', deletedAt: '' }
}

const unknownValue = (): VoiceValue => ({ value: '', source: 'unknown', unknown: true })
export function emptyStructuredVoiceProfile(): StructuredVoiceProfile {
  return { version: 0n, updatedAt: '', sourceCount: 0, empty: true, lexical: { description: unknownValue(), preferredWords: [], bannedWords: [], bannedPatterns: [] }, endings: { baseRegister: unknownValue(), distribution: [], bannedEndings: [], signatureEndings: [], constraints: [] }, syntax: { averageSentenceChars: 0, sentenceLength: unknownValue(), connectiveStyle: unknownValue(), preferredConnectives: [], nominalization: unknownValue(), passiveTendency: unknownValue() }, structure: { introPattern: unknownValue(), closingPattern: unknownValue(), paragraphSentencesMin: 0, paragraphSentencesMax: 0, headingHabit: unknownValue(), listHabit: unknownValue(), emojiUse: unknownValue() }, axes: {}, rules: [], sources: [], feedback: [] }
}

export function isEmptyProfile(profile: Pick<VoiceProfile, 'structured' | 'styleguide' | 'samples' | 'finalizedSourceCount'>): boolean {
  return profile.structured.empty && profile.styleguide.trim() === '' && profile.samples.length === 0 && profile.finalizedSourceCount === 0
}

/** How a deleted voice is named wherever a post still points at it (spec/policy/posts.md). */
export const DELETED_VOICE_PREFIX = '삭제된 말투'

export function voiceRefLabel(voice: Pick<VoiceRef, 'name' | 'deleted'>): string {
  return voice.deleted ? `${DELETED_VOICE_PREFIX} · ${voice.name}` : voice.name
}

/** Why every AI action on a deleted-voice post is unavailable. One string, so generate, revise and
 *  finalize cannot explain the same server rule three different ways. The server enforces it;
 *  this only says so before the round trip. */
export const DELETED_VOICE_AI_REASON = '삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.'

export function activeVoices<T extends Pick<Voice, 'deleted'>>(voices: readonly T[]): T[] {
  return voices.filter((voice) => !voice.deleted)
}

export function deletedVoices<T extends Pick<Voice, 'deleted'>>(voices: readonly T[]): T[] {
  return voices.filter((voice) => voice.deleted)
}

export function defaultVoice<T extends Pick<Voice, 'deleted' | 'isDefault'>>(voices: readonly T[]): T | undefined {
  return voices.find((voice) => voice.isDefault && !voice.deleted)
}

/** The server's directory order — active before deleted, the default first, then by name and id —
 *  re-applied after a cache patch so an inserted or renamed voice lands where a refetch would put
 *  it. Plain string comparison, not a locale collation, because that is what SQLite's ORDER BY did. */
export function sortVoices<T extends Pick<Voice, 'id' | 'name' | 'isDefault' | 'deleted'>>(voices: readonly T[]): T[] {
  return [...voices].sort(
    (a, b) =>
      Number(a.deleted) - Number(b.deleted) ||
      Number(b.isDefault) - Number(a.isDefault) ||
      compare(a.name, b.name) ||
      compare(a.id, b.id),
  )
}

const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0)
