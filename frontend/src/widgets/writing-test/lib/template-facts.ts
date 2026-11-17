export interface FactAsk {
  label: string
  required: boolean
  prompt: string
  /** The same question stands in two or more groups and holds one answer (MODEL-30). */
  shared: boolean
}
/** One contestant template and the questions it asks, in the order it asks them. */
export interface FactGroup {
  key: string
  name: string
  asks: FactAsk[]
}
export interface FactAnswer {
  label: string
  text: string
  enabled: boolean
}

export const factFieldId = (groupIndex: number, label: string) => `test-fact-${groupIndex}-${label}`
export const factAnswered = (answers: readonly FactAnswer[], label: string) =>
  answers.some((answer) => answer.label === label && answer.enabled && !!answer.text.trim())
