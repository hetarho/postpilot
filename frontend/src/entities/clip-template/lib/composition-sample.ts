import { CLIP_COMPOSITION_PREVIEW } from '@/entities/clip-design/@x/clip-template'
import type {
  ClipComposition,
  CompositionElement,
  CompositionInputs,
  CompositionTimeline,
  ResolvedCompositionElement,
} from '../model/composition'
import { resolveClipComposition } from './composition-resolve'

/** How the preview's illustration fills the clip (CLIP-170). */
export interface CompositionSampleOptions {
  /** The numbered sample sentence the captions continue with once the outline's own caption
   *  entries are placed; without it only those entries are shown. */
  caption?: (n: number) => string
  /** The styles the captions take in turn: the template's allowed styles, or the default alone
   *  when it allows none. */
  styles?: readonly string[]
}

/** The illustrative clip the template preview draws (CLIP-169, CLIP-170). The outline is
 *  resolved against sample answers, then timed as an illustration rather than as the entries'
 *  own timing: the intro alone in the first 2.5 s and the outro alone in the last 3 s, the badge
 *  over the whole clip, and captions of 3–4 s back to back between them — the outline's own
 *  caption entries first, in outline order, then numbered sample sentences — each in the next
 *  allowed style. A region with no entry leaves its span to the captions; a caption entry the
 *  span cannot hold is left out. Nothing here reaches a project. */
export function sampleClipComposition(
  doc: ClipComposition,
  durationMs: number,
  sample: (label: string, n: number) => string,
  options: CompositionSampleOptions = {},
): CompositionTimeline {
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
  const resolved = resolveClipComposition(doc, input, CLIP_COMPOSITION_PREVIEW.expandedBytes)
  return illustrate(resolved, options)
}

function illustrate(
  timeline: CompositionTimeline,
  { caption, styles = [] }: CompositionSampleOptions,
): CompositionTimeline {
  const total = timeline.durationMs
  const { introMs, outroMs, captionMaxMs } = CLIP_COMPOSITION_PREVIEW
  const of = (role: CompositionElement['role']) =>
    timeline.elements.filter((e) => e.element.role === role)
  const intro = of('hook'),
    outro = of('ending')
  const from = intro.length ? Math.min(introMs, total) : 0
  const to = outro.length ? Math.max(from, total - outroMs) : total
  const span = to - from
  const at = (entry: ResolvedCompositionElement, startMs: number, endMs: number) => ({
    ...entry,
    startMs,
    endMs,
  })
  // The outline's own captions in the order it declares them; a caption repeated per cut
  // follows its cuts.
  const own = timeline.elements
    .map((entry, i) => ({ entry, i }))
    .filter(({ entry }) => entry.element.role === 'caption')
    .sort((a, b) => a.entry.element.span.start - b.entry.element.span.start || a.i - b.i)
    .map(({ entry }) => entry)
  const slots = span > 0 ? Math.ceil(span / captionMaxMs) : 0
  const captions: ResolvedCompositionElement[] = []
  for (let i = 0; i < slots; i++) {
    const entry =
      own[i] ?? (caption && sampleCaption(i - own.length + 1, caption(i - own.length + 1)))
    if (!entry) break
    const style = styles.length ? styles[i % styles.length] : entry.element.style
    captions.push({
      ...at(
        entry,
        from + Math.floor((span * i) / slots),
        from + Math.floor((span * (i + 1)) / slots),
      ),
      element: { ...entry.element, style },
    })
  }
  // Any other text keeps its own timing inside the span the regions leave it (CLIP-170).
  const others = timeline.elements
    .filter((e) => e.element.role === 'info')
    .map((e) => at(e, Math.max(e.startMs, from), Math.min(e.endMs, to)))
    .filter((e) => e.startMs < e.endMs)
  return {
    durationMs: total,
    elements: [
      ...intro.map((e) => at(e, 0, from)),
      ...others,
      ...captions,
      ...outro.map((e) => at(e, to, total)),
      ...of('badge').map((e) => at(e, 0, total)),
    ],
  }
}

function sampleCaption(n: number, text: string): ResolvedCompositionElement {
  return {
    instanceId: `sample-caption-${n}`,
    cutId: '',
    groupId: '',
    itemId: '',
    element: {
      id: `sample-caption-${n}`,
      kind: 'fixed',
      role: 'caption',
      style: 'auto',
      position: 'auto',
      align: 'center',
      basis: 'whole',
      startMs: null,
      endMs: null,
      chars: 0,
      parts: [{ literal: text, field: '' }],
      rows: [],
      span: { start: 0, end: 0, line: 1 },
    },
    text,
    rows: [],
    facts: [],
    startMs: 0,
    endMs: 0,
    authoredTiming: false,
  }
}
