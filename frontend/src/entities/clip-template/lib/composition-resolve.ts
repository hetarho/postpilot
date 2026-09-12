import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import {
  CompositionProblem,
  type ClipComposition,
  type CompositionCut,
  type CompositionElement,
  type CompositionField,
  type CompositionInputs,
  type CompositionItem,
  type CompositionLimits,
  type CompositionPart,
  type CompositionTimeline,
  type ResolvedCompositionElement,
} from '../model/composition'
import {
  compositionIdentifier,
  validCompositionLimits,
  trimCompositionSpace,
} from './composition-parse'
import { scalarLength } from './composition-xml'

const fieldKey = (f: CompositionField) => (f.group ? `${f.group}.${f.id}` : f.id)
const valueOf = (values: Record<string, string>, key: string) =>
  Object.hasOwn(values, key) ? values[key] : ''
const bytes = (s: string) => new TextEncoder().encode(s).length

/** Resolves supplied real cuts; selection and AI writing are separate stages. */
export function resolveClipComposition(
  d: ClipComposition,
  input: CompositionInputs,
  maxExpandedBytes: number,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): CompositionTimeline {
  const fail = (id: string, line: number, reason: CompositionProblem['reason']): never => {
    throw new CompositionProblem(id, line, reason)
  }
  if (
    !validCompositionLimits(limits) ||
    !Number.isSafeInteger(maxExpandedBytes) ||
    maxExpandedBytes <= 0
  )
    fail('clip', 1, 'invalid_limits')
  if (input.cuts.length > limits.cuts) fail('clip', 1, 'cut_limit')
  const fields = new Map(d.fields.map((f) => [fieldKey(f), f]))
  const validateValues = (values: Record<string, string>, group: string, id: string) => {
    for (const k of Object.keys(values).sort()) {
      const v = values[k]
      if (!fields.has(group ? `${group}.${k}` : k)) fail(id, 1, 'unknown_field')
      if (scalarLength(v) > limits.answerChars) fail(id, 1, 'answer_limit')
    }
    for (const f of d.fields)
      if (f.group === group && f.required && !trimCompositionSpace(valueOf(values, f.id)))
        fail(fieldKey(f), f.span.line, 'required_binding')
  }
  validateValues(input.values, '', 'clip')
  const items = new Map<string, CompositionItem>()
  for (const group of Object.keys(input.items).sort()) {
    const values = input.items[group]
    if (!d.groups.includes(group)) fail(group, 1, 'unknown_group')
    if (values.length > limits.items) fail(group, 1, 'item_limit')
    for (const item of values) {
      const key = `${group}/${item.id}`
      if (!compositionIdentifier.test(item.id)) fail(group, 1, 'invalid_id')
      if (items.has(key)) fail(item.id, 1, 'duplicate_item')
      validateValues(item.values, group, item.id)
      items.set(key, item)
    }
  }
  const sections = new Map(d.sections.map((s) => [s.id, s])),
    starts: number[] = [],
    seen = new Set<string>()
  const output: CompositionTimeline = { durationMs: 0, elements: [] }
  let lastDuration = 0
  for (const [i, c] of input.cuts.entries()) {
    if (!compositionIdentifier.test(c.id) || !c.sourceId || seen.has(c.id))
      fail(c.id, 1, 'invalid_cut')
    seen.add(c.id)
    if (c.sectionId) {
      const s = sections.get(c.sectionId)
      if (!s) return fail(c.id, 1, 'unknown_section')
      if (s.repeat && s.repeat !== 'scenes' && c.groupId !== s.repeat)
        fail(c.id, 1, 'binding_scope')
    }
    if (!c.itemId !== !c.groupId) fail(c.id, 1, 'binding_scope')
    if (c.itemId && !items.has(`${c.groupId}/${c.itemId}`)) fail(c.id, 1, 'unknown_item')
    const duration = c.endMs - c.startMs
    if (
      ![c.startMs, c.endMs, c.transitionMs].every(Number.isSafeInteger) ||
      c.startMs < 0 ||
      duration <= 0 ||
      duration > limits.maxDurationMs ||
      c.transitionMs < 0 ||
      (i === 0 && c.transitionMs !== 0) ||
      (i > 0 && (c.transitionMs >= duration || c.transitionMs >= lastDuration))
    )
      fail(c.id, 1, 'invalid_cut')
    starts.push(output.durationMs - c.transitionMs)
    output.durationMs = starts[i] + duration
    lastDuration = duration
    if (output.durationMs > limits.maxDurationMs) fail(c.id, 1, 'duration_limit')
  }
  if (output.durationMs <= 0) fail('clip', 1, 'missing_footage')
  let budget = [...d.guidance, ...d.sections.flatMap((s) => s.guidance)].reduce(
    (n, g) => n + bytes(g),
    0,
  )
  const append = (t: CompositionElement, cut: CompositionCut | null, cutStart: number) => {
    let instanceId = t.id
    const groupId = cut?.groupId ?? '',
      itemId = cut?.itemId ?? '',
      cutId = cut?.id ?? '',
      values = itemId ? items.get(`${groupId}/${itemId}`)!.values : {}
    if (cut) {
      instanceId += `/${cutId}`
      if (itemId) instanceId += `/${groupId}/${itemId}`
    }
    const r: ResolvedCompositionElement = {
      instanceId,
      cutId,
      groupId,
      itemId,
      element: t,
      text: '',
      rows: [],
      facts: [],
      startMs: 0,
      endMs: 0,
      authoredTiming: t.basis !== 'cut' || t.startMs !== null,
    }
    let omit = false
    const bind = (parts: CompositionPart[]) => {
      const text = parts
        .map((p) => {
          if (!p.field) return p.literal
          const f = fields.get(p.field)!
          if (f.group && (f.group !== groupId || !itemId)) fail(t.id, t.span.line, 'binding_scope')
          const value = valueOf(f.group ? values : input.values, f.id)
          if (!trimCompositionSpace(value)) {
            if (f.required) fail(t.id, t.span.line, 'required_binding')
            omit = true
          }
          r.facts.push({ fieldId: f.id, groupId: f.group, itemId: f.group ? itemId : '', value })
          return value
        })
        .join('')
      if (scalarLength(text) > (t.kind === 'ai' ? limits.guideChars : limits.copyChars))
        fail(t.id, t.span.line, 'copy_limit')
      return text
    }
    r.text = bind(t.parts)
    r.rows = t.rows.map((row) => ({ role: row.role, text: bind(row.parts) }))
    if (omit) return
    let start = 0,
      end = output.durationMs
    switch (t.basis) {
      case 'whole':
        break
      case 'output-start':
        start = t.startMs!
        end = t.endMs!
        break
      case 'output-end':
        start = output.durationMs + t.startMs!
        end = output.durationMs + t.endMs!
        break
      case 'cut': {
        if (!cut) return fail(t.id, t.span.line, 'binding_scope')
        const duration = cut.endMs - cut.startMs
        start = t.startMs ?? limits.autoInsetMs
        end = t.endMs ?? duration - limits.autoInsetMs
        if (start < 0 || end > duration || start >= end) fail(t.id, t.span.line, 'interval_outside')
        start += cutStart
        end += cutStart
        break
      }
    }
    if (start < 0 || end > output.durationMs || start >= end)
      fail(t.id, t.span.line, 'interval_outside')
    r.startMs = start
    r.endMs = end
    budget +=
      bytes(r.text) +
      r.rows.reduce((n, row) => n + bytes(row.text), 0) +
      r.facts.reduce((n, f) => n + bytes(f.value), 0)
    if (budget > maxExpandedBytes) fail(t.id, t.span.line, 'expansion_limit')
    output.elements.push(r)
    if (output.elements.length > limits.cues) fail(t.id, t.span.line, 'cue_limit')
  }
  for (const t of d.elements) append(t, null, 0)
  for (const [i, c] of input.cuts.entries())
    for (const t of sections.get(c.sectionId)?.elements ?? []) append(t, c, starts[i])
  if (budget > maxExpandedBytes) fail('clip', 1, 'expansion_limit')
  return output
}
