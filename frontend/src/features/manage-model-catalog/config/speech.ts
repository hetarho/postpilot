export const SPEECH_ADJUSTMENT_MIN = 0
export const SPEECH_ADJUSTMENT_MAX = 1
export const SPEECH_ACCOUNT_TARIFF_DEFAULTS = {
  revision: 0n,
  designUsdPerUnit: '0.00008',
  speechUsdPerUnit: '0.00008',
  confirmationUsd: '0',
  source: 'https://elevenlabs.io/pricing/api',
  complete: false,
  checkedAt: '',
} as const

/** Public references reviewed on this date; not verified account tariffs. */
export const SPEECH_PUBLIC_PRICING = {
  source: 'https://elevenlabs.io/pricing/api',
  designSource:
    'https://help.elevenlabs.io/hc/en-us/articles/29315418701073-How-much-does-Voice-Design-cost',
  reviewedAt: '2026-10-06',
  characters: 1000,
  designUsd: '0.08',
  confirmationUsd: '0',
  promotionUntil: '2026-10-12',
  promotionExpiresAt: '2026-10-13T00:00:00+09:00',
} as const

export const SPEECH_MODEL_PRESENTATION: Readonly<
  Record<string, { name: string; usdPer1000?: string; promotionalUsd?: string }>
> = {
  eleven_multilingual_ttv_v2: { name: 'Voice Design v2' },
  eleven_ttv_v3: { name: 'Voice Design v3' },
  eleven_v4: { name: 'Eleven v4', usdPer1000: '0.08', promotionalUsd: '0.022' },
  eleven_v4_turbo: { name: 'Eleven v4 Turbo', usdPer1000: '0.04', promotionalUsd: '0.011' },
  eleven_v3: { name: 'Eleven v3', usdPer1000: '0.08' },
  eleven_v3_conversational: { name: 'Eleven v3 Conversational', usdPer1000: '0.04' },
  eleven_multilingual_v2: { name: 'Eleven Multilingual v2', usdPer1000: '0.08' },
  eleven_flash_v2_5: { name: 'Eleven Flash v2.5', usdPer1000: '0.04' },
  eleven_flash_v2: { name: 'Eleven Flash v2', usdPer1000: '0.04' },
  eleven_turbo_v2_5: { name: 'Eleven Turbo v2.5', usdPer1000: '0.04' },
  eleven_turbo_v2: { name: 'Eleven Turbo v2', usdPer1000: '0.04' },
}
