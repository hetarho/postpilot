import {
  CLIP_COMPOSITION_LIMITS,
  CLIP_DESIGN,
  CLIP_REGIONS,
} from '@/entities/clip-design/@x/clip-template'
import { type ClipRegionPresets } from '@/entities/clip-design/@x/clip-template'
import type { ClipComposition } from '../model/composition'

const slots = (kind: 'intro' | 'outro', preset: string) =>
  kind === 'intro'
    ? (CLIP_REGIONS.intro[preset as keyof typeof CLIP_REGIONS.intro]?.slots ?? [])
    : (CLIP_REGIONS.outro[preset as keyof typeof CLIP_REGIONS.outro]?.slots ?? [])

/** The maximum ① holds an answer to. The parser's own number cannot count a
 *  region slot — which preset holds that line is the project's (CLIP-147) — so
 *  the surface that DOES know the presets narrows it here, and an answer that
 *  cannot be typed too long cannot fail at generation (CLIP-116, CLIP-117). */
export function clipFieldMaximum(
  document: ClipComposition,
  presets: ClipRegionPresets,
  key: string,
) {
  let max = document.maxima[key] ?? CLIP_COMPOSITION_LIMITS.answerChars
  const taken = { intro: 0, outro: 0 }
  for (const entry of document.outline) {
    if (entry.kind !== 'text') continue
    const element = document.elements[entry.index]
    const kind = element.role === 'hook' ? 'intro' : element.role === 'ending' ? 'outro' : undefined
    if (!kind) continue
    const preset = slots(kind, presets[kind])
    element.rows.forEach((row, i) => {
      const slot = preset[taken[kind] + i]
      if (!slot || (row.kind || element.kind) === 'ai') return
      if (!row.parts.some((part) => part.field === key)) return
      const chars = CLIP_DESIGN.type[slot.type as keyof typeof CLIP_DESIGN.type]?.chars
      if (chars) max = Math.min(max, chars)
    })
    taken[kind] += element.rows.length || 1
  }
  return max
}
