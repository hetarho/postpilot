export const SPEECH_PRICE_COMPONENTS_MAX = 8
export const SPEECH_PROFILE_LABEL_MAX = 100
export const SPEECH_OPERATIONS = ['voice_design', 'voice_confirm', 'speech'] as const
export const SPEECH_UNITS = [
  'characters',
  'supplier_credits',
  'character_cost',
  'requests',
  'seconds',
] as const
export const SPEECH_BINDING_DEFAULTS = {
  settings: { stability: 0.5, similarityBoost: 0.75, style: 0, speakerBoost: false, speed: 1 },
  outputFormat: 'mp3_44100_128',
  descriptionMax: 1000,
  previewMax: 1000,
  speechMax: 1000,
} as const
