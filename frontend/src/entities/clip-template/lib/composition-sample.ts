import { CLIP_COMPOSITION_PREVIEW } from '@/shared/config'
import type { ClipComposition, CompositionInputs } from '../model/composition'
import { resolveClipComposition } from './composition-resolve'

export function sampleClipComposition(
  doc: ClipComposition,
  durationMs: number,
  sample: (label: string, n: number) => string,
) {
  const values = (group: string, n: number) =>
    Object.fromEntries(
      doc.fields.filter((f) => f.group === group).map((f) => [f.id, sample(f.label, n)]),
    )
  const input: CompositionInputs = { values: values('', 1), items: {}, cuts: [] }
  for (const group of doc.groups)
    input.items[group] = Array.from({ length: CLIP_COMPOSITION_PREVIEW.sampleItems }, (_, i) => ({
      id: `sample_${i + 1}`,
      values: values(group, i + 1),
    }))
  for (const section of doc.sections) {
    const group =
      section.repeat && section.repeat !== 'scenes'
        ? section.repeat
        : section.scope === 'item'
          ? (doc.groups[0] ?? '')
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
  return resolveClipComposition(doc, input, CLIP_COMPOSITION_PREVIEW.expandedBytes)
}
