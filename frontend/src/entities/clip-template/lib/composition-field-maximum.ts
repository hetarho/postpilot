import {
  CLIP_COMPOSITION_LIMITS,
  clipRegionSlotAt,
  clipRegionSlotBudget,
  clipRegionSlots,
} from '@/entities/clip-design/@x/clip-template'
import { type ClipRatioId, type ClipRegionPresets } from '@/entities/clip-design/@x/clip-template'
import type { ClipComposition } from '../model/composition'

/** The maximum ① holds an answer to. The parser's own number cannot count a
 *  region slot — which preset holds that line is the project's (CLIP-147) — so
 *  the surface that DOES know the presets narrows it here to the syllables that
 *  fit the slot at its floor on the project's ratio (CDS-86, CLIP-116, CLIP-117). */
export function clipFieldMaximum(
  document: ClipComposition,
  presets: ClipRegionPresets,
  key: string,
  ratio: ClipRatioId = 'vertical',
) {
  let max = document.maxima[key] ?? CLIP_COMPOSITION_LIMITS.answerChars
  const taken = { intro: 0, outro: 0 }
  for (const entry of document.outline) {
    if (entry.kind !== 'text') continue
    const element = document.elements[entry.index]
    const kind = element.role === 'hook' ? 'intro' : element.role === 'ending' ? 'outro' : undefined
    if (!kind) continue
    const preset = clipRegionSlots(kind, presets[kind])
    element.rows.forEach((row, i) => {
      const slot = preset[taken[kind] + i]
      if (!slot || (row.kind || element.kind) === 'ai') return
      if (!row.parts.some((part) => part.field === key)) return
      const at = clipRegionSlotAt(kind, presets[kind], ratio, taken[kind] + i)
      const budget = at ? clipRegionSlotBudget(at.spec, at.width) : 0
      if (budget > 0) max = Math.min(max, budget)
    })
    taken[kind] += element.rows.length || 1
  }
  return max
}
