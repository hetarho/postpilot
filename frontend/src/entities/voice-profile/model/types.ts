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
export interface VoiceProfile {
  styleguide: string; rules: string; legacyManualGuidance: string; updatedAt: string
  samples: VoiceSample[]; activeJobId: string; structured: StructuredVoiceProfile
  finalizedSourceCount: number; canValidate: boolean
}
export interface VoiceVersion { version: bigint; profile: StructuredVoiceProfile; origin: string; restoredFromVersion: bigint; createdAt: string }

const unknownValue = (): VoiceValue => ({ value: '', source: 'unknown', unknown: true })
export function emptyStructuredVoiceProfile(): StructuredVoiceProfile {
  return { version: 0n, updatedAt: '', sourceCount: 0, empty: true, lexical: { description: unknownValue(), preferredWords: [], bannedWords: [], bannedPatterns: [] }, endings: { baseRegister: unknownValue(), distribution: [], bannedEndings: [], signatureEndings: [], constraints: [] }, syntax: { averageSentenceChars: 0, sentenceLength: unknownValue(), connectiveStyle: unknownValue(), preferredConnectives: [], nominalization: unknownValue(), passiveTendency: unknownValue() }, structure: { introPattern: unknownValue(), closingPattern: unknownValue(), paragraphSentencesMin: 0, paragraphSentencesMax: 0, headingHabit: unknownValue(), listHabit: unknownValue(), emojiUse: unknownValue() }, axes: {}, rules: [], sources: [], feedback: [] }
}

export function isEmptyProfile(profile: Pick<VoiceProfile, 'structured' | 'styleguide' | 'samples' | 'finalizedSourceCount'>): boolean {
  return profile.structured.empty && profile.styleguide.trim() === '' && profile.samples.length === 0 && profile.finalizedSourceCount === 0
}
