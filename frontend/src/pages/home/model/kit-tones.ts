import type { ComponentType } from 'react'
import { Brain, LayoutList, Quote, ShieldCheck } from 'lucide-react'
import type { KitElement } from './readiness'

export const KIT_ICONS: Record<KitElement, ComponentType<{ className?: string }>> = {
  template: LayoutList,
  voice: Quote,
  memory: Brain,
  guideline: ShieldCheck,
}

/** Each element's ink — the colour the product already gives that kind of thing: the template's
 *  headings are the accent bars of its outline, a voice's habits are the review's voice-habit
 *  marks, a memory is the owner's own fact (the review's owner origin), and a rule is the warm
 *  stamp in the margin. Home's sheet, its legend and its readiness dots use the same four. */
export const KIT_TONES: Record<
  KitElement,
  { fill: string; ring: string; line: string; soft: string }
> = {
  template: {
    fill: 'bg-badge-accent-bg text-badge-accent-fg',
    ring: 'ring-kit-ink-template text-badge-accent-fg',
    line: 'bg-kit-ink-template',
    soft: 'bg-badge-accent-bg',
  },
  voice: {
    fill: 'bg-voice-habit-highlight text-voice-habit-foreground',
    ring: 'ring-kit-ink-voice text-voice-habit-foreground',
    line: 'bg-kit-ink-voice',
    soft: 'bg-voice-habit-highlight',
  },
  memory: {
    fill: 'bg-origin-owner-highlight text-origin-owner-foreground',
    ring: 'ring-kit-ink-memory text-origin-owner-foreground',
    line: 'bg-kit-ink-memory',
    soft: 'bg-origin-owner-highlight',
  },
  guideline: {
    fill: 'bg-origin-ai-highlight text-origin-ai-foreground',
    ring: 'ring-kit-ink-guideline text-origin-ai-foreground',
    line: 'bg-kit-ink-guideline',
    soft: 'bg-origin-ai-highlight',
  },
}
