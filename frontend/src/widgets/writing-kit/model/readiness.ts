/** The writing kit: the four things the owner teaches the AI so a post comes out in their own
 *  words — how the sentences sound (말투), what shape a post takes (템플릿), what the AI always
 *  keeps to (지침) and what it knows about the owner (기억).
 *
 *  Every figure here is read from the account's own lists; nothing is estimated. */
export const KIT_ELEMENTS = ['voices', 'templates', 'guidelines', 'memories'] as const
export type KitElement = (typeof KIT_ELEMENTS)[number]
export type KitState = 'ready' | 'partial' | 'empty'

export interface KitReading {
  key: KitElement
  state: KitState
  pending: boolean
  failed: boolean
  /** How many the owner made: made voices, saved templates, their own rules, memories. */
  own: number
  /** What stands in while the owner has made none: voices still being made, unsaved template
   *  drafts, 기본 지침 in use. Zero once the owner has made one — a stand-in no longer counts. */
  standIns: number
  /** How far the element has come, 0–100: 100 once ready; for a voice still being made, the
   *  best readiness share among them; otherwise 0. */
  share: number
}

/** The most items one element draws; past it the element is simply full. */
export const KIT_MAX_MARKS = 12

export interface KitInputs {
  voices: {
    pending: boolean
    failed: boolean
    /** Active (not deleted) voices. */
    voices: readonly { made: boolean; readinessPercent: number }[]
  }
  templates: { pending: boolean; failed: boolean; saved: number; unsavedDrafts: number }
  guidelines: { pending: boolean; failed: boolean; own: number; defaultsInUse: number }
  memories: { pending: boolean; failed: boolean; count: number }
}

/** What "ready" means, element by element:
 *  - 말투: a made voice exists (one that has a published analysis and can write). A voice still
 *    being made is partial, at its readiness share.
 *  - 템플릿: at least one saved template. An unsaved AI draft is partial.
 *  - 지침: at least one guideline the owner wrote. Only the 기본 지침 in use is partial.
 *  - 기억: at least one memory. */
export function readWritingKit(inputs: KitInputs): KitReading[] {
  const made = inputs.voices.voices.filter((voice) => voice.made).length
  const making = inputs.voices.voices.filter((voice) => !voice.made)
  return [
    element('voices', inputs.voices, made, making.length, {
      share: Math.max(0, ...making.map((voice) => clampPercent(voice.readinessPercent))),
    }),
    element('templates', inputs.templates, inputs.templates.saved, inputs.templates.unsavedDrafts),
    element(
      'guidelines',
      inputs.guidelines,
      inputs.guidelines.own,
      inputs.guidelines.defaultsInUse,
    ),
    element('memories', inputs.memories, inputs.memories.count, 0),
  ]
}

function element(
  key: KitElement,
  read: { pending: boolean; failed: boolean },
  own: number,
  standIns: number,
  partial: { share: number } = { share: 0 },
): KitReading {
  const state: KitState = own > 0 ? 'ready' : standIns > 0 ? 'partial' : 'empty'
  return {
    key,
    state,
    pending: read.pending,
    failed: read.failed,
    own,
    standIns: own > 0 ? 0 : standIns,
    share: state === 'ready' ? 100 : state === 'partial' ? partial.share : 0,
  }
}

export function readyCount(readings: readonly KitReading[]): number {
  return readings.filter((reading) => reading.state === 'ready').length
}

function clampPercent(value: number): number {
  return Number.isFinite(value) ? Math.min(100, Math.max(0, Math.round(value))) : 0
}
