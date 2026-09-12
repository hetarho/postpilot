import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import {
  CompositionProblem,
  type ClipComposition,
  type CompositionLimits,
  type CompositionNode,
  type CompositionElement,
  type CompositionPart,
  type CompositionSection,
  type CompositionSpan,
} from '../model/composition'
import {
  problem,
  readCompositionXML,
  scalarLength,
  serializeCompositionNode,
  xmlScalar,
} from './composition-xml'

export const compositionIdentifier = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/
const styles = ['clean', 'memo', 'bold', 'mark', 'simple']
export const compositionSpace =
  /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/u
export const trimCompositionSpace = (s: string) =>
  s.split(compositionSpace).filter(Boolean).join(' ')
const rowRoles = ['hook', 'title', 'mark', 'body', 'caption', 'label', 'badge']
export function validCompositionLimits(l: CompositionLimits) {
  return (Object.keys(CLIP_COMPOSITION_LIMITS) as (keyof CompositionLimits)[]).every((key) => {
    const value = l[key]
    return Number.isSafeInteger(value) && (key === 'autoInsetMs' ? value >= 0 : value > 0)
  })
}
function attributes(n: CompositionNode, ...allowed: string[]) {
  if (Object.keys(n.attributes).some((k) => !allowed.includes(k))) problem(n, 'unknown_attribute')
}
function content(n: CompositionNode) {
  if (n.children.some((c) => c.name !== '#text')) problem(n, 'unexpected_child')
  return n.children.map((c) => c.text).join('')
}
function children(n: CompositionNode) {
  if (n.children.some((c) => c.name === '#text' && trimCompositionSpace(c.text) !== ''))
    problem(n, 'unexpected_text')
  return n.children.filter((c) => c.name !== '#text')
}
/** Exact integer arithmetic, with no seconds-to-float-to-ms conversion. */
export function compositionMilliseconds(s: string, max: number): number | null {
  if (!/^-?[0-9]+(?:\.[0-9]{1,3})?$/.test(s)) return null
  const negative = s.startsWith('-'),
    [whole, fraction = ''] = s.replace(/^-/, '').split('.')
  const ms = Number(whole) * 1000 + Number(fraction.padEnd(3, '0'))
  if (!Number.isSafeInteger(ms) || ms > max) return null
  return negative && ms !== 0 ? -ms : ms
}
function readElement(
  n: CompositionNode,
  d: ClipComposition,
  scope: string,
  repeat: string,
  inScene: boolean,
  l: CompositionLimits,
): CompositionElement {
  attributes(n, 'id', 'kind', 'role', 'style', 'position', 'align', 'basis', 'start', 'end')
  const a = n.attributes,
    kind = a.kind,
    role = a.role,
    basis = a.basis
  if (kind !== 'fixed' && kind !== 'ai') problem(n, 'invalid_kind')
  if (
    role !== 'caption' &&
    role !== 'info' &&
    role !== 'badge' &&
    role !== 'hook' &&
    role !== 'ending'
  )
    problem(n, 'invalid_role')
  const style = a.style ?? 'auto',
    position = a.position ?? 'auto',
    align = a.align ?? 'center'
  if (style !== 'auto' && !d.styles.includes(style)) problem(n, 'invalid_style')
  if (
    !['auto', 'top', 'upper_mid', 'lower_mid', 'bottom', 'header'].includes(position) ||
    (position === 'header' && role !== 'info' && role !== 'badge')
  )
    problem(n, 'invalid_position')
  if (!['left', 'center', 'right'].includes(align)) problem(n, 'invalid_align')
  if (basis !== 'whole' && basis !== 'output-start' && basis !== 'output-end' && basis !== 'cut')
    problem(n, 'invalid_basis')
  if (basis === 'cut' && !inScene) problem(n, 'invalid_basis')
  const hasStart = Object.hasOwn(a, 'start'),
    hasEnd = Object.hasOwn(a, 'end')
  if (
    hasStart !== hasEnd ||
    (basis === 'whole' && hasStart) ||
    ((basis === 'output-start' || basis === 'output-end') && !hasStart)
  )
    problem(n, 'invalid_interval')
  const startMs = hasStart ? compositionMilliseconds(a.start, l.maxDurationMs) : null,
    endMs = hasEnd ? compositionMilliseconds(a.end, l.maxDurationMs) : null
  if (
    hasStart &&
    (startMs === null ||
      endMs === null ||
      startMs >= endMs ||
      (basis === 'output-end' ? endMs > 0 : startMs < 0))
  )
    problem(n, 'invalid_interval')
  const parts = (parent: CompositionNode): CompositionPart[] =>
    parent.children.map((c) => {
      if (c.name === '#text') return { literal: c.text, field: '' }
      if (c.name !== 'value') problem(n, 'unknown_tag')
      if (Object.keys(c.attributes).some((k) => k !== 'field')) problem(n, 'unknown_attribute')
      if (c.children.length) problem(n, 'unexpected_child')
      const ref = c.attributes.field
      const field = d.fields.find((f) => (f.group ? `${f.group}.${f.id}` : f.id) === ref)
      if (!field) problem(n, 'unknown_field')
      if (
        field.group &&
        (scope !== 'item' || (repeat !== '' && repeat !== 'scenes' && repeat !== field.group))
      )
        problem(n, 'binding_scope')
      return { literal: '', field: ref }
    })
  const t: CompositionElement = {
    id: a.id,
    kind,
    role,
    basis,
    style,
    position,
    align,
    startMs,
    endMs,
    parts: [],
    rows: [],
    span: n.span,
  }
  if (n.children.some((c) => c.name === 'row')) {
    if (!['hook', 'ending', 'info'].includes(role)) problem(n, 'invalid_rows')
    t.rows = children(n).map((c) => {
      if (c.name !== 'row') problem(n, 'invalid_rows')
      if (Object.keys(c.attributes).some((k) => k !== 'role')) problem(n, 'unknown_attribute')
      if (!rowRoles.includes(c.attributes.role)) problem(n, 'invalid_row_role')
      return { role: c.attributes.role, parts: parts(c) }
    })
  } else t.parts = parts(n)
  const max = kind === 'ai' ? l.guideChars : l.copyChars
  if (
    [t.parts, ...t.rows.map((r) => r.parts)].some(
      (ps) => ps.reduce((n, p) => n + scalarLength(p.literal), 0) > max,
    )
  )
    problem(n, 'copy_limit')
  return t
}

export function parseClipComposition(
  source: string,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): ClipComposition {
  if (!validCompositionLimits(limits)) throw new CompositionProblem('clip', 1, 'invalid_limits')
  if (
    Array.from(source).some((c) => {
      const code = c.codePointAt(0)!
      return code >= 0xd800 && code <= 0xdfff
    })
  )
    throw new CompositionProblem('clip', 1, 'invalid_unicode')
  if (scalarLength(source) > limits.sourceChars)
    throw new CompositionProblem('clip', 1, 'source_limit')
  if (Array.from(source).some((c) => !xmlScalar(c.codePointAt(0)!)))
    throw new CompositionProblem('clip', 1, 'invalid_unicode')
  const root = readCompositionXML(source, limits)
  if (root.name !== 'clip') problem(root, 'unknown_tag')
  attributes(root, 'version', 'styles', 'accent', 'pace')
  if (root.attributes.version !== '1') problem(root, 'unknown_version')
  const d: ClipComposition = {
    source,
    root,
    styles: (root.attributes.styles ?? 'clean').split(compositionSpace).filter(Boolean),
    accent: root.attributes.accent ?? '',
    pace: root.attributes.pace ?? 'steady',
    fields: [],
    groups: [],
    guidance: [],
    sections: [],
    elements: [],
  }
  if (
    !d.styles.length ||
    new Set(d.styles).size !== d.styles.length ||
    d.styles.some((s) => !styles.includes(s))
  )
    problem(root, 'invalid_style')
  if (!['', 'coral', 'amber', 'lime', 'teal', 'blue', 'violet', 'pink'].includes(d.accent))
    problem(root, 'invalid_accent')
  if (!['steady', 'rapid'].includes(d.pace)) problem(root, 'invalid_pace')
  const ns = children(root),
    ids = new Set<string>()
  const claim = (n: CompositionNode, prefix = '') => {
    const id = n.attributes.id
    if (!compositionIdentifier.test(id ?? '')) problem(n, 'invalid_id')
    if (ids.has(prefix + id)) problem(n, 'duplicate_id')
    ids.add(prefix + id)
  }
  const field = (n: CompositionNode, group = '') => {
    attributes(n, 'id', 'label', 'required')
    claim(n, group ? `${group}.` : '')
    const prompt = content(n),
      label = n.attributes.label ?? '',
      required = n.attributes.required ?? 'false'
    if (
      !trimCompositionSpace(label) ||
      scalarLength(label) > limits.labelChars ||
      scalarLength(prompt) > limits.promptChars
    )
      problem(n, 'field_limit')
    if (required !== 'true' && required !== 'false') problem(n, 'invalid_required')
    d.fields.push({
      id: n.attributes.id,
      group,
      label,
      prompt,
      required: required === 'true',
      span: n.span,
    })
    if (d.fields.length > limits.fields) problem(n, 'field_limit')
  }
  for (const n of ns) {
    if (n.name === 'field') field(n)
    if (n.name === 'group') {
      attributes(n, 'id')
      claim(n)
      const group = n.attributes.id
      if (group === 'scenes') problem(n, 'invalid_id')
      d.groups.push(group)
      const fs = children(n)
      if (!fs.length) problem(n, 'empty_group')
      for (const f of fs) {
        if (f.name !== 'field') problem(f, 'unknown_tag')
        field(f, group)
      }
    }
  }
  const guide = (n: CompositionNode) => {
    attributes(n)
    const v = content(n)
    if (scalarLength(v) > limits.guideChars) problem(n, 'guide_limit')
    return v
  }
  const section = (n: CompositionNode, repeat = '') => {
    attributes(n, 'id', 'scope')
    claim(n)
    const scope = n.attributes.scope ?? 'scene'
    if (
      !['scene', 'item', 'context'].includes(scope) ||
      (repeat !== '' && repeat !== 'scenes' && scope !== 'item')
    )
      problem(n, 'invalid_scope')
    const s: CompositionSection = {
      id: n.attributes.id,
      scope,
      repeat,
      guidance: [],
      elements: [],
      span: n.span,
    }
    for (const c of children(n)) {
      if (c.name === 'guide') s.guidance.push(guide(c))
      else if (c.name === 'text') {
        claim(c)
        s.elements.push(readElement(c, d, scope, repeat, true, limits))
      } else problem(c, 'unknown_tag')
    }
    d.sections.push(s)
  }
  for (const n of ns) {
    switch (n.name) {
      case 'field':
      case 'group':
        break
      case 'guide':
        d.guidance.push(guide(n))
        break
      case 'text':
        claim(n)
        d.elements.push(readElement(n, d, 'context', '', false, limits))
        break
      case 'scene':
        section(n)
        break
      case 'repeat': {
        attributes(n, 'for')
        const over = n.attributes.for
        if (over !== 'scenes' && !d.groups.includes(over)) problem(n, 'unknown_repeat')
        const scenes = children(n)
        if (!scenes.length) problem(n, 'empty_repeat')
        for (const c of scenes) {
          if (c.name !== 'scene') problem(c, 'unknown_tag')
          section(c, over)
        }
        break
      }
      default:
        problem(n, 'unknown_tag')
    }
  }
  return d
}

export function replaceCompositionNode(
  d: ClipComposition,
  id: string,
  replacement: CompositionNode,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): ClipComposition {
  const find = (n: CompositionNode, group = ''): CompositionNode | undefined => {
    const key = n.name === 'field' && group ? `${group}.${n.attributes.id}` : n.attributes.id
    return key === id
      ? n
      : n.children.map((c) => find(c, n.name === 'group' ? n.attributes.id : group)).find(Boolean)
  }
  const target = find(d.root)
  if (!target) throw new CompositionProblem(id, 1, 'unknown_element')
  return replaceCompositionSpan(d, target.span, replacement, limits)
}

export function replaceCompositionSpan(
  d: ClipComposition,
  span: CompositionSpan,
  replacement: CompositionNode,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): ClipComposition {
  const find = (n: CompositionNode): boolean =>
    (n.span.start === span.start && n.span.end === span.end && n.span.line === span.line) ||
    n.children.some(find)
  if (!find(d.root)) throw new CompositionProblem('clip', span.line, 'unknown_element')
  const chars = Array.from(d.source)
  return parseClipComposition(
    chars.slice(0, span.start).join('') +
      serializeCompositionNode(replacement) +
      chars.slice(span.end).join(''),
    limits,
  )
}
