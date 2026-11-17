/** The owner's writing kit: the five things that make a post high quality AND theirs. Every figure
 *  here is read from the account's own data — a voice's own readiness meter, the lists of
 *  templates, guidelines and memories, the write stage's chosen model — and nothing is a made-up
 *  progress number. A list counts as ready from its first entry; only the voice has a real
 *  in-between, its readiness meter (VOICE-9). */

/** Around the picture clockwise from the top. 말투 and 기억 — what makes a post the owner's own —
 *  sit side by side at the top left; 템플릿, 지침 and the AI — what makes it good — fill the
 *  rest of the circle. */
export const KIT_KEYS = ['voice', 'template', 'guideline', 'ai', 'memory'] as const
export type KitKey = (typeof KIT_KEYS)[number]
/** `failed`: the piece's data could not be read, so nothing is claimed about it either way. */
export type KitState = 'loading' | 'failed' | 'empty' | 'partial' | 'ready'

export const KIT_HREF: Record<KitKey, string> = {
  voice: '/voices',
  template: '/templates',
  guideline: '/guidelines',
  ai: '/ai-models',
  memory: '/memories',
}

/** Where on its screen a piece is made ready, when that is not the screen's top: the writing AI
 *  is chosen under AI 토너먼트's 지금 쓰는 AI, below the comparisons that open the tab. */
export const KIT_HASH: Partial<Record<KitKey, string>> = { ai: 'ai-in-use' }

/** Where 홈's 다음 opens when it can go straight to the making: a template has one way to be
 *  made — the conversation — so its next step starts it rather than the list it will join. */
export const KIT_NEXT_HREF: Partial<Record<KitKey, string>> = { template: '/templates/new' }

/** What an element's one fact line names. Never a figure the account does not hold. */
export type KitFact =
  | { kind: 'loading' }
  | { kind: 'failed' }
  | { kind: 'none' }
  | { kind: 'voice'; name: string; percent: number; made: boolean }
  | { kind: 'count'; count: number; lead?: string }
  | { kind: 'defaultsOnly'; count: number }
  | { kind: 'model'; label: string }
  | { kind: 'unavailable' }

export interface KitElement {
  key: KitKey
  href: string
  state: KitState
  /** How much of the ring is drawn, 0–1: a voice's own meter, otherwise all or nothing. */
  fill: number
  fact: KitFact
}

const LOADING = (key: KitKey): KitElement => ({
  key,
  href: KIT_HREF[key],
  state: 'loading',
  fill: 0,
  fact: { kind: 'loading' },
})

/** A piece whose list could not be read: neither ready nor missing, and never the next step. */
export const failedElement = (key: KitKey): KitElement => ({
  key,
  href: KIT_HREF[key],
  state: 'failed',
  fill: 0,
  fact: { kind: 'failed' },
})

interface VoiceLike {
  name: string
  made: boolean
  readinessPercent: number
  isDefault: boolean
}

/** 말투: the voice a new post is written in — the default, else the most nearly made one. Ready
 *  once that voice is made (it has a published analysis); until then its own readiness meter. */
export function voiceElement(active: readonly VoiceLike[], pending: boolean): KitElement {
  if (pending) return LOADING('voice')
  const voice =
    active.find((candidate) => candidate.isDefault) ??
    active.find((candidate) => candidate.made) ??
    [...active].sort((a, b) => b.readinessPercent - a.readinessPercent)[0]
  if (!voice)
    return { key: 'voice', href: KIT_HREF.voice, state: 'empty', fill: 0, fact: { kind: 'none' } }
  const percent = voice.made ? 100 : Math.max(0, Math.min(100, Math.round(voice.readinessPercent)))
  return {
    key: 'voice',
    href: KIT_HREF.voice,
    state: voice.made ? 'ready' : 'partial',
    fill: percent / 100,
    fact: { kind: 'voice', name: voice.name, percent, made: voice.made },
  }
}

/** 템플릿 · 기억: ready from the first one. The fact names the most used template, so the picture
 *  says which shape the owner actually writes in. */
export function listElement(
  key: 'template' | 'memory',
  items: readonly { name?: string; postCount?: number }[],
  pending: boolean,
): KitElement {
  if (pending) return LOADING(key)
  if (items.length === 0)
    return { key, href: KIT_HREF[key], state: 'empty', fill: 0, fact: { kind: 'none' } }
  const lead =
    key === 'template'
      ? [...items].sort((a, b) => (b.postCount ?? 0) - (a.postCount ?? 0))[0]?.name
      : undefined
  return {
    key,
    href: KIT_HREF[key],
    state: 'ready',
    fill: 1,
    fact: { kind: 'count', count: items.length, lead },
  }
}

/** 지침: ready once the owner has written one of their own. Until then the product's 기본 지침
 *  still apply, and the fact says so rather than calling the writing rule-less. */
export function guidelineElement(own: number, defaultsInUse: number, pending: boolean): KitElement {
  if (pending) return LOADING('guideline')
  if (own > 0)
    return {
      key: 'guideline',
      href: KIT_HREF.guideline,
      state: 'ready',
      fill: 1,
      fact: { kind: 'count', count: own },
    }
  return {
    key: 'guideline',
    href: KIT_HREF.guideline,
    state: 'empty',
    fill: 0,
    fact: defaultsInUse > 0 ? { kind: 'defaultsOnly', count: defaultsInUse } : { kind: 'none' },
  }
}

/** 글 쓰는 AI: ready while the write stage has a usable model; a saved choice that can no longer
 *  be used asks to be chosen again. */
export function aiElement(
  selectedLabel: string | null,
  unavailable: boolean,
  pending: boolean,
): KitElement {
  if (pending) return LOADING('ai')
  if (selectedLabel)
    return {
      key: 'ai',
      href: KIT_HREF.ai,
      state: 'ready',
      fill: 1,
      fact: { kind: 'model', label: selectedLabel },
    }
  return {
    key: 'ai',
    href: KIT_HREF.ai,
    state: 'empty',
    fill: 0,
    fact: unavailable ? { kind: 'unavailable' } : { kind: 'none' },
  }
}

/** Which missing piece to point at first: nothing is written without the AI, then the owner's own
 *  voice, then the shape, the rules and the memories. */
const NEXT_ORDER: readonly KitKey[] = ['ai', 'voice', 'template', 'guideline', 'memory']

export function nextKitStep(elements: readonly KitElement[]): KitElement | undefined {
  for (const key of NEXT_ORDER) {
    const element = elements.find((candidate) => candidate.key === key)
    if (element && (element.state === 'empty' || element.state === 'partial')) return element
  }
  return undefined
}

export function kitSummary(elements: readonly KitElement[]) {
  const loading = elements.some((element) => element.state === 'loading')
  const ready = elements.filter((element) => element.state === 'ready').length
  return {
    loading,
    /** Some piece could not be read: the count would undercount it, so the screen says so. */
    failed: elements.some((element) => element.state === 'failed'),
    ready,
    total: elements.length,
    complete: !loading && ready === elements.length,
    /** The share the picture's halo glows with: the rings' own fills, averaged. */
    fill: elements.reduce((sum, element) => sum + element.fill, 0) / elements.length,
  }
}
