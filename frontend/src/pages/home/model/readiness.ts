/** The four things the writer reads before it writes a post, in the order home draws them: the
 *  structure (템플릿), the voice (말투), what it knows (기억) and the rules (지침). */
export const KIT_ELEMENTS = ['template', 'voice', 'memory', 'guideline'] as const
export type KitElement = (typeof KIT_ELEMENTS)[number]

/** `partial` is a kit element that already shapes a post but not yet the owner's own way: a voice
 *  still being made (or made but not the default), or only the product's 기본 지침. */
export type KitLevel = 'loading' | 'failed' | 'empty' | 'partial' | 'ready'

export interface KitReadiness {
  element: KitElement
  level: KitLevel
  /** A voice still being made: the server's readiness meter share (VOICE-9). */
  percent?: number
}

export interface KitInputs {
  voices?: {
    pending: boolean
    failed: boolean
    active: readonly { made: boolean; isDefault: boolean; readinessPercent: number }[]
  }
  templates: { pending: boolean; failed: boolean; count: number }
  guidelines: { pending: boolean; failed: boolean; own: number; defaultsInUse: number }
  memories: { pending: boolean; failed: boolean; count: number }
}

/** What "ready" means, from the owner's own data and nothing invented:
 *  - 말투: a made voice is the default, so every new post is written in it. A voice that exists but
 *    is still being made, or is made but not the default, is partial.
 *  - 템플릿: at least one saved template a post can be shaped by.
 *  - 지침: at least one rule the owner wrote. Only the product's 기본 지침 in use is partial.
 *  - 기억: at least one saved fact the writer can draw on. */
export function kitReadiness(inputs: KitInputs): KitReadiness[] {
  const status = (query: { pending: boolean; failed: boolean }) =>
    query.failed ? 'failed' : query.pending ? 'loading' : undefined
  const voice = (): KitReadiness => {
    const voices = inputs.voices
    if (!voices) return { element: 'voice', level: 'loading' }
    const known = status(voices)
    if (known) return { element: 'voice', level: known }
    if (voices.active.length === 0) return { element: 'voice', level: 'empty' }
    if (voices.active.some((item) => item.isDefault && item.made))
      return { element: 'voice', level: 'ready' }
    const making = voices.active.filter((item) => !item.made)
    return {
      element: 'voice',
      level: 'partial',
      percent: making.length ? Math.max(...making.map((item) => item.readinessPercent)) : undefined,
    }
  }
  const counted = (
    element: 'template' | 'memory',
    query: { pending: boolean; failed: boolean; count: number },
  ): KitReadiness => ({
    element,
    level: status(query) ?? (query.count > 0 ? 'ready' : 'empty'),
  })
  const guideline: KitReadiness = {
    element: 'guideline',
    level:
      status(inputs.guidelines) ??
      (inputs.guidelines.own > 0
        ? 'ready'
        : inputs.guidelines.defaultsInUse > 0
          ? 'partial'
          : 'empty'),
  }
  const byElement: Record<KitElement, KitReadiness> = {
    template: counted('template', inputs.templates),
    voice: voice(),
    memory: counted('memory', inputs.memories),
    guideline,
  }
  return KIT_ELEMENTS.map((element) => byElement[element])
}

/** How many of the four are ready, and the first one that is not — the one home offers next. */
export function kitSummary(kit: readonly KitReadiness[]): {
  ready: number
  total: number
  settled: boolean
  next?: KitElement
} {
  const settled = kit.every((item) => item.level !== 'loading')
  const next = kit.find((item) => item.level === 'empty' || item.level === 'partial')
  return {
    ready: kit.filter((item) => item.level === 'ready').length,
    total: kit.length,
    settled,
    // The voice first: it is what makes a post the owner's own, so an unready voice is offered
    // ahead of an earlier layer.
    next: kit.some(
      (item) => item.element === 'voice' && (item.level === 'empty' || item.level === 'partial'),
    )
      ? 'voice'
      : next?.element,
  }
}
