/** The writing kit: the four things the owner teaches the AI so a post comes out in their own
 *  words — how the sentences sound (말투), what shape a post takes (템플릿), what the AI always
 *  keeps to (지침) and what it knows about the owner (기억).
 *
 *  Every figure here is read from the account's own lists; nothing is estimated. A ring draws one
 *  mark per thing the owner has made (or, for a voice still being made, its readiness share). */
export const KIT_ELEMENTS = ['voices', 'templates', 'guidelines', 'memories'] as const
export type KitElement = (typeof KIT_ELEMENTS)[number]
export type KitState = 'ready' | 'partial' | 'empty'

/** How a ring is drawn: a continuous share (a voice being made) or one mark per item. */
export type KitMarks =
  { kind: 'share'; percent: number } | { kind: 'count'; own: number; assumed: number }

export interface KitReading {
  key: KitElement
  state: KitState
  pending: boolean
  failed: boolean
  marks: KitMarks
}

/** The most marks a ring draws; past it the ring is simply full. */
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
 *    being made is partial, drawn at its readiness share.
 *  - 템플릿: at least one saved template. An unsaved AI draft is partial.
 *  - 지침: at least one guideline the owner wrote. Only the 기본 지침 in use is partial.
 *  - 기억: at least one memory. */
export function readWritingKit(inputs: KitInputs): KitReading[] {
  const made = inputs.voices.voices.filter((voice) => voice.made).length
  const making = inputs.voices.voices.filter((voice) => !voice.made)
  const voiceShare = made
    ? 100
    : Math.max(0, ...making.map((voice) => clampPercent(voice.readinessPercent)))
  return [
    {
      key: 'voices',
      state: made > 0 ? 'ready' : making.length > 0 ? 'partial' : 'empty',
      pending: inputs.voices.pending,
      failed: inputs.voices.failed,
      marks:
        made > 1
          ? { kind: 'count', own: made, assumed: 0 }
          : { kind: 'share', percent: voiceShare },
    },
    {
      key: 'templates',
      state:
        inputs.templates.saved > 0
          ? 'ready'
          : inputs.templates.unsavedDrafts > 0
            ? 'partial'
            : 'empty',
      pending: inputs.templates.pending,
      failed: inputs.templates.failed,
      marks: {
        kind: 'count',
        own: inputs.templates.saved,
        assumed: inputs.templates.unsavedDrafts,
      },
    },
    {
      key: 'guidelines',
      state:
        inputs.guidelines.own > 0
          ? 'ready'
          : inputs.guidelines.defaultsInUse > 0
            ? 'partial'
            : 'empty',
      pending: inputs.guidelines.pending,
      failed: inputs.guidelines.failed,
      marks: {
        kind: 'count',
        own: inputs.guidelines.own,
        assumed: inputs.guidelines.defaultsInUse,
      },
    },
    {
      key: 'memories',
      state: inputs.memories.count > 0 ? 'ready' : 'empty',
      pending: inputs.memories.pending,
      failed: inputs.memories.failed,
      marks: { kind: 'count', own: inputs.memories.count, assumed: 0 },
    },
  ]
}

export function readyCount(readings: readonly KitReading[]): number {
  return readings.filter((reading) => reading.state === 'ready').length
}

function clampPercent(value: number): number {
  return Number.isFinite(value) ? Math.min(100, Math.max(0, Math.round(value))) : 0
}
