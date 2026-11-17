/** The writing kit: what the owner prepares so the AI writes high-quality posts in their own
 *  words. Each part's state comes from data the account already holds — nothing here is a score
 *  the client invents. `progress` (0…1) is only ever a real figure: the voice's own readiness
 *  share, or 0 / 1 for a part that either exists or does not.
 *
 *  What "ready" means, part by part:
 *  - 말투 (voice): a default voice that is made (its analysis is published). A voice still being
 *    learned is partial at its own `readinessPercent`; a made voice with no default is partial.
 *  - 템플릿 (template): at least one saved post template. An unsaved AI draft is partial.
 *  - 지침 (guideline): at least one of the owner's own post guidelines. Only 기본 지침 in use is
 *    partial — the AI already keeps some rules, none of them the owner's.
 *  - 기억 (memory): at least one memory.
 *  - 쓰는 AI (model): a usable write model is selected. A saved choice that can no longer be used
 *    is partial.
 *  - 목소리 (spoken): at least one confirmed spoken voice; an unfinished one is partial. It reads
 *    videos, so it is shown with the kit but not counted in the writing readiness. */
export type KitPartKey = 'voice' | 'template' | 'guideline' | 'memory' | 'model' | 'spoken'
export type KitState = 'loading' | 'failed' | 'empty' | 'partial' | 'ready'

export interface KitPart {
  key: KitPartKey
  state: KitState
  progress: number
  /** Which status line the part shows, with its values. */
  status: KitStatus
  href: string
  /** Something waiting for the owner here (guideline suggestions to review). */
  attention?: number
}

export type KitStatus =
  | { kind: 'loading' }
  | { kind: 'failed' }
  | { kind: 'none' }
  | { kind: 'named'; name: string }
  | { kind: 'count'; count: number }
  | { kind: 'learning'; percent: number }
  | { kind: 'noDefault' }
  | { kind: 'draft' }
  | { kind: 'defaultsOnly'; count: number }
  | { kind: 'ownAndDefaults'; own: number; defaults: number }
  | { kind: 'unusable' }

interface Query {
  isPending: boolean
  isError: boolean
}

export interface KitInput {
  voices: Query & {
    active: readonly {
      name: string
      made: boolean
      isDefault: boolean
      readinessPercent: number
    }[]
  }
  templates: Query & { count: number; drafts: number }
  guidelines: Query & { own: number; defaultsInUse: number; suggestions: number }
  memories: Query & { count: number }
  model: { isPending: boolean; selectedLabel: string | null; unusable: boolean }
  spoken: Query & { count: number; drafts: number }
}

/** The four writing parts and the AI, in the order the graphic and the readiness count use. */
export const WRITING_PARTS: readonly KitPartKey[] = [
  'voice',
  'template',
  'guideline',
  'memory',
  'model',
]

function pending(query: Query): KitStatus | undefined {
  if (query.isError) return { kind: 'failed' }
  if (query.isPending) return { kind: 'loading' }
  return undefined
}

function stateOf(status: KitStatus, ready: boolean, partial: boolean): KitState {
  if (status.kind === 'loading') return 'loading'
  if (status.kind === 'failed') return 'failed'
  return ready ? 'ready' : partial ? 'partial' : 'empty'
}

export function kitParts(input: KitInput): Record<KitPartKey, KitPart> {
  const voice = (): KitPart => {
    const waiting = pending(input.voices)
    const chosen = input.voices.active.find((candidate) => candidate.isDefault)
    const made = input.voices.active.filter((candidate) => candidate.made)
    const learning = input.voices.active.filter((candidate) => !candidate.made)
    const best = Math.max(0, ...learning.map((candidate) => candidate.readinessPercent))
    const ready = !!chosen?.made
    const status: KitStatus =
      waiting ??
      (ready
        ? { kind: 'named', name: chosen.name }
        : made.length > 0
          ? { kind: 'noDefault' }
          : learning.length > 0
            ? { kind: 'learning', percent: best }
            : { kind: 'none' })
    return {
      key: 'voice',
      state: stateOf(status, ready, input.voices.active.length > 0),
      progress: ready || made.length > 0 ? 1 : learning.length > 0 ? best / 100 : 0,
      status,
      href: '/voices',
    }
  }
  const template = (): KitPart => {
    const waiting = pending(input.templates)
    const { count, drafts } = input.templates
    const status: KitStatus =
      waiting ??
      (count > 0 ? { kind: 'count', count } : drafts > 0 ? { kind: 'draft' } : { kind: 'none' })
    return {
      key: 'template',
      state: stateOf(status, count > 0, drafts > 0),
      progress: count > 0 ? 1 : 0,
      status,
      href: '/templates',
    }
  }
  const guideline = (): KitPart => {
    const waiting = pending(input.guidelines)
    const { own, defaultsInUse, suggestions } = input.guidelines
    const status: KitStatus =
      waiting ??
      (own > 0
        ? { kind: 'ownAndDefaults', own, defaults: defaultsInUse }
        : defaultsInUse > 0
          ? { kind: 'defaultsOnly', count: defaultsInUse }
          : { kind: 'none' })
    return {
      key: 'guideline',
      state: stateOf(status, own > 0, defaultsInUse > 0),
      progress: own > 0 ? 1 : 0,
      status,
      href: '/guidelines',
      attention: waiting ? undefined : suggestions || undefined,
    }
  }
  const memory = (): KitPart => {
    const waiting = pending(input.memories)
    const { count } = input.memories
    const status: KitStatus = waiting ?? (count > 0 ? { kind: 'count', count } : { kind: 'none' })
    return {
      key: 'memory',
      state: stateOf(status, count > 0, false),
      progress: count > 0 ? 1 : 0,
      status,
      href: '/memories',
    }
  }
  const model = (): KitPart => {
    const { selectedLabel, unusable } = input.model
    const status: KitStatus = input.model.isPending
      ? { kind: 'loading' }
      : selectedLabel
        ? { kind: 'named', name: selectedLabel }
        : unusable
          ? { kind: 'unusable' }
          : { kind: 'none' }
    return {
      key: 'model',
      state: stateOf(status, !!selectedLabel, unusable),
      progress: selectedLabel ? 1 : 0,
      status,
      href: '/ai-models',
    }
  }
  const spoken = (): KitPart => {
    const waiting = pending(input.spoken)
    const { count, drafts } = input.spoken
    const status: KitStatus =
      waiting ??
      (count > 0 ? { kind: 'count', count } : drafts > 0 ? { kind: 'draft' } : { kind: 'none' })
    return {
      key: 'spoken',
      state: stateOf(status, count > 0, drafts > 0),
      progress: count > 0 ? 1 : 0,
      status,
      href: '/spoken-voices',
    }
  }
  return {
    voice: voice(),
    template: template(),
    guideline: guideline(),
    memory: memory(),
    model: model(),
    spoken: spoken(),
  }
}

export interface KitSummary {
  ready: number
  total: number
  settled: boolean
  /** The first writing part that is not ready, in kit order — the one the graphic lights. */
  next: KitPartKey | undefined
  phase: 'loading' | 'empty' | 'partial' | 'ready'
}

export function kitSummary(parts: Record<KitPartKey, KitPart>): KitSummary {
  const writing = WRITING_PARTS.map((key) => parts[key])
  const settled = writing.every((part) => part.state !== 'loading')
  const ready = writing.filter((part) => part.state === 'ready').length
  const next = settled ? writing.find((part) => part.state !== 'ready')?.key : undefined
  // A new account already has the product's 기본 지침 and usually a prepared write model, so
  // "started" counts only what the owner made: a voice, a template, a memory or a rule of their own.
  const started = writing.some(
    (part) =>
      part.key !== 'model' &&
      (part.state === 'ready' || (part.state === 'partial' && part.key !== 'guideline')),
  )
  return {
    ready,
    total: writing.length,
    settled,
    next,
    phase: !settled
      ? 'loading'
      : ready === writing.length
        ? 'ready'
        : started
          ? 'partial'
          : 'empty',
  }
}
