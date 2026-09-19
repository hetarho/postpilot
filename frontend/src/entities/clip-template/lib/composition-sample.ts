import { CLIP_COMPOSITION_PREVIEW } from '@/entities/clip-design/@x/clip-template'
import type { ClipComposition, CompositionInputs } from '../model/composition'
import { resolveClipComposition } from './composition-resolve'

/** The illustrative resolution the template preview draws. `narration` is the
 *  one sample caption sentence: a template declares none any more, and the
 *  preview still has to show the caption treatment against the skeleton and the
 *  badge, because that is the design decision the owner is making (CLIP-4). */
export function sampleClipComposition(
  doc: ClipComposition,
  durationMs: number,
  sample: (label: string, n: number) => string,
  narration = '',
) {
  const values = (group: string, n: number) =>
    Object.fromEntries(
      doc.fields.filter((f) => f.group === group).map((f) => [f.id, sample(f.label, n)]),
    )
  const input: CompositionInputs = { values: values('', 1), items: {}, cuts: [] }
  for (const group of doc.groups)
    input.items[group.id] = Array.from(
      { length: Math.min(group.max, Math.max(group.min, CLIP_COMPOSITION_PREVIEW.sampleItems)) },
      (_, i) => ({
        id: `sample_${i + 1}`,
        values: values(group.id, i + 1),
      }),
    )
  for (const section of doc.sections) {
    const group =
      section.repeat && section.repeat !== 'scenes'
        ? section.repeat
        : section.scope === 'item'
          ? (doc.groups[0]?.id ?? '')
          : ''
    const items = group ? input.items[group] : [{ id: '' }]
    for (const item of items)
      input.cuts.push({
        id: `sample_${input.cuts.length + 1}`,
        sectionId: section.id,
        sourceId: 'illustration',
        groupId: group,
        itemId: item.id,
        startMs: 0,
        endMs: 0,
        transitionMs: 0,
      })
  }
  if (!input.cuts.length)
    input.cuts.push({
      id: 'sample',
      sectionId: '',
      sourceId: 'illustration',
      groupId: '',
      itemId: '',
      startMs: 0,
      endMs: durationMs,
      transitionMs: 0,
    })
  input.cuts.forEach((cut, i) => {
    cut.endMs =
      Math.floor((durationMs * (i + 1)) / input.cuts.length) -
      Math.floor((durationMs * i) / input.cuts.length)
  })
  const timeline = resolveClipComposition(doc, input, CLIP_COMPOSITION_PREVIEW.expandedBytes)
  // A template that declares its own caption entries is previewed with those
  // (CLIP-112); the sample line is for one that declares none.
  if (!narration || doc.elements.some((e) => e.role === 'caption')) return timeline
  const start = Math.round(timeline.durationMs / 3),
    end = Math.min(timeline.durationMs, start + CLIP_COMPOSITION_PREVIEW.narrationMs)
  return {
    ...timeline,
    elements: [
      ...timeline.elements,
      {
        instanceId: 'sample-narration',
        cutId: '',
        groupId: '',
        itemId: '',
        element: {
          id: 'sample-narration',
          kind: 'fixed' as const,
          role: 'caption' as const,
          style: 'auto',
          position: 'auto',
          align: 'center',
          basis: 'whole' as const,
          startMs: null,
          endMs: null,
          chars: 0,
          parts: [{ literal: narration, field: '' }],
          rows: [],
          span: { start: 0, end: 0, line: 1 },
        },
        text: narration,
        rows: [],
        facts: [],
        startMs: start,
        endMs: end,
        authoredTiming: false,
      },
    ],
  }
}
