import type { ComponentType } from 'react'
import { Brain, LayoutList, ListChecks, Quote } from 'lucide-react'
import type { KitElement } from '../model/readiness'

/** Each element's glyph and hue, the same everywhere the kit appears (home, the kit's chips and
 *  each kit screen's head). */
export const KIT_LOOK: Readonly<
  Record<
    KitElement,
    { icon: ComponentType<{ className?: string }>; text: string; stroke: string; wash: string }
  >
> = {
  voices: {
    icon: Quote,
    text: 'text-kit-voice',
    stroke: 'stroke-kit-voice',
    wash: 'bg-kit-voice/15',
  },
  templates: {
    icon: LayoutList,
    text: 'text-kit-template',
    stroke: 'stroke-kit-template',
    wash: 'bg-kit-template/15',
  },
  guidelines: {
    icon: ListChecks,
    text: 'text-kit-guideline',
    stroke: 'stroke-kit-guideline',
    wash: 'bg-kit-guideline/15',
  },
  memories: {
    icon: Brain,
    text: 'text-kit-memory',
    stroke: 'stroke-kit-memory',
    wash: 'bg-kit-memory/15',
  },
}

/** Where each element is made ready. */
export const KIT_HREF: Readonly<Record<KitElement, string>> = {
  voices: '/voices',
  templates: '/templates',
  guidelines: '/guidelines',
  memories: '/memories',
}
