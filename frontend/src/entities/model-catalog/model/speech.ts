import type { ModelRef } from './types'
import type { LevelName } from './level'

export interface SpeechProfileChoice {
  id: string
  revision: bigint
  label: string
  designModel: ModelRef
  designLabel: string
  speechModel: ModelRef
  speechLabel: string
  grade: LevelName | ''
  requiredPlan: string
  entitled: boolean
  available: boolean
  unavailableReason: string
  descriptionMax: number
  previewMin: number
  previewMax: number
  speechMax: number
  voiceReady: boolean
  exportReady: boolean
}

export interface SpeechCandidate {
  ref: ModelRef
  label: string
  design: boolean
  synthesis: boolean
  korean: boolean
  style: boolean
  speakerBoost: boolean
  requiresAlpha: boolean
  maxText: number
  tokenCostFactor: string
  characterCostMultiplier: string
  costDiscountMultiplier: string
}

export interface SpeechPriceComponent {
  unit: string
  usdPerUnit: string
  multiplier: string
  maximumUnits: string
  unitsPerInputCharacter: string
}
export interface SpeechOperationPrice {
  operation: string
  charges: SpeechPriceComponent[]
  source: string
  boundsSource: string
  checkedAt: string
  complete: boolean
}
export interface SpeechProfileBinding {
  designModel: ModelRef
  speechModel: ModelRef
  settings: {
    stability: number
    similarityBoost: number
    style: number
    speakerBoost: boolean
    speed: number
  }
  outputFormat: string
  descriptionMax: number
  previewMax: number
  speechMax: number
}
export interface AdminSpeechProfile {
  id: string
  revision: bigint
  label: string
  grade: LevelName | ''
  enabled: boolean
  binding: SpeechProfileBinding
  prices: SpeechOperationPrice[]
  voiceReady: boolean
  exportReady: boolean
}
export interface SpeechAdminBrowse {
  profiles: AdminSpeechProfile[]
  candidates: SpeechCandidate[]
  choices: SpeechProfileChoice[]
  fetchError: string
}
