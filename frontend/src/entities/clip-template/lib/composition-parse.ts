import { CLIP_COMPOSITION_LIMITS, CLIP_DESIGN } from '@/entities/clip-design/@x/clip-template'
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
export const compositionSpace =
  /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/u
export const trimCompositionSpace = (s: string) =>
  s.split(compositionSpace).filter(Boolean).join(' ')
const rowRoles = ['caption', 'label']
/** CDS-20's count: Korean syllables, excluding spaces, punctuation and symbols.
 * The same rule the renderer counts by, so the template's number, the owner's
 * counter and the rendered line are one measurement. */
export const compositionCharacters = (s: string) =>
  Array.from(s.replace(/[\s\p{P}\p{S}]/gu, '')).length
/** Reads a `chars` attribute (CLIP-116): a positive integer no larger than the
 * count the position already imposes. Absent is 0, meaning the position keeps
 * its derived cap. A limit of 0 is a position that imposes none. */
function declaredChars(n: CompositionNode, blame: CompositionNode, limit: number) {
  const raw = n.attributes.chars
  if (raw === undefined) return 0
  const value = Number(raw)
  if (!raw || /[^0-9]/.test(raw) || !Number.isSafeInteger(value) || value <= 0)
    problem(blame, 'invalid_max')
  if (limit > 0 && value > limit) problem(blame, 'invalid_max')
  return value
}
const typeChars = (type: string) =>
  CLIP_DESIGN.type[type as keyof typeof CLIP_DESIGN.type]?.chars ?? 0
/** The CDS-20 count a rendered position already imposes, and 0 for a position
 * that imposes none of its own — a badge or a caption, whose line and wrap rules
 * are CDS-25's and the repair ladder's rather than a character ceiling.
 *
 * A region row imposes none here either: how many lines its region draws and at
 * what size is the preset the PROJECT chose (CLIP-147), so the drawn position
 * holds the text to its count when it renders (CDS-77). An information row takes
 * its row role, and an element without rows reads as a caption when it carries
 * information. One derivation, exported so the editor bounds its control by the
 * number the parser enforces rather than one of its own (CLIP-116). */
export function compositionPositionChars(role: string, row?: { role?: string; index: number }) {
  if (!row) return role === 'info' ? typeChars('caption') : 0
  return role === 'info' ? typeChars(row.role ?? '') : 0
}
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
  stored: boolean,
  template = false,
): CompositionElement {
  attributes(
    n,
    'id',
    'kind',
    'role',
    'position',
    'align',
    'basis',
    'start',
    'end',
    'chars',
    ...(stored ? ['style'] : []),
  )
  const a = n.attributes,
    kind = a.kind,
    role = a.role,
    // An entry that declares no basis takes the one its role is drawn at: the
    // output's opening for an intro entry, its end for an outro entry, the whole
    // output for a badge or a caption the narration has yet to time.
    basis =
      a.basis ?? (role === 'hook' ? 'output-start' : role === 'ending' ? 'output-end' : 'whole')
  if (kind !== 'fixed' && kind !== 'ai') problem(n, 'invalid_kind')
  if (
    role !== 'caption' &&
    role !== 'info' &&
    role !== 'badge' &&
    role !== 'hook' &&
    role !== 'ending'
  )
    problem(n, 'invalid_role')
  // Cut-bound information went with the sections, so `info` stays the one role
  // a template may not take (CLIP-4, CLIP-59). A caption is an outline entry
  // again (CLIP-112) and the narration places it.
  if (template && role === 'info') problem(n, 'unsupported_role')
  // A template entry declares no interval at all — the order it stands in is the
  // only position it has (CLIP-66, CLIP-112). A frozen snapshot keeps every
  // interval it was frozen with (CLIP-140).
  if (template && ['basis', 'start', 'end'].some((key) => Object.hasOwn(a, key)))
    problem(n, 'unsupported_basis')
  const region = role === 'hook' || role === 'ending'
  if (
    region &&
    !stored &&
    (inScene ||
      Object.hasOwn(a, 'position') ||
      Object.hasOwn(a, 'align') ||
      (role === 'hook' ? basis !== 'output-start' : basis !== 'output-end'))
  )
    problem(n, 'invalid_skeleton')
  const style = 'auto',
    position = a.position ?? 'auto',
    align = a.align ?? 'center'
  if (
    !['auto', 'top', 'upper_mid', 'lower_mid', 'bottom', 'header'].includes(position) ||
    (position === 'header' && role !== 'info' && role !== 'badge')
  )
    problem(n, 'invalid_position')
  if (!['left', 'center', 'right'].includes(align)) problem(n, 'invalid_align')
  if (basis !== 'whole' && basis !== 'output-start' && basis !== 'output-end' && basis !== 'cut')
    problem(n, 'invalid_basis')
  if (basis === 'cut' && !inScene) problem(n, 'invalid_basis')
  const ownLimit = compositionPositionChars(role) || l.copyChars
  const chars = declaredChars(n, n, ownLimit)
  const hasStart = Object.hasOwn(a, 'start'),
    hasEnd = Object.hasOwn(a, 'end')
  if (
    hasStart !== hasEnd ||
    (basis === 'whole' && hasStart) ||
    ((basis === 'output-start' || basis === 'output-end') && !hasStart && !region)
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
    startMs:
      region && !hasStart && (basis === 'output-start' || basis === 'output-end')
        ? role === 'hook'
          ? 0
          : -CLIP_DESIGN.timing.outro_default_s * 1000
        : startMs,
    endMs:
      region && !hasEnd && (basis === 'output-start' || basis === 'output-end')
        ? role === 'hook'
          ? CLIP_DESIGN.timing.intro_default_s * 1000
          : 0
        : endMs,
    chars,
    parts: [],
    rows: [],
    span: n.span,
  }
  if (n.children.some((c) => c.name === 'row')) {
    if (!['hook', 'ending', 'info'].includes(role)) problem(n, 'invalid_rows')
    t.rows = children(n).map((c, index) => {
      if (c.name !== 'row') problem(n, 'invalid_rows')
      const allowed =
        region && !stored
          ? ['kind', 'chars']
          : region
            ? ['role', 'kind', 'chars']
            : ['role', 'chars']
      if (Object.keys(c.attributes).some((k) => !allowed.includes(k)))
        problem(n, region ? 'invalid_skeleton' : 'unknown_attribute')
      const rowRole = region
        ? ''
        : stored && c.attributes.role !== 'label'
          ? 'caption'
          : c.attributes.role
      if (!region && !rowRoles.includes(rowRole)) problem(n, 'invalid_row_role')
      const rowKind = c.attributes.kind ?? kind
      if (rowKind !== 'fixed' && rowKind !== 'ai') problem(n, 'invalid_kind')
      // A region row's own count is the preset slot it lands in, which is the
      // project's and unknowable here (CLIP-147); the grammar's copy ceiling is
      // all this row can be held to. An information pair still takes its row
      // role's count.
      const slot = compositionPositionChars(role, { role: rowRole, index }) || l.copyChars
      return { role: rowRole, kind: rowKind, chars: declaredChars(c, n, slot), parts: parts(c) }
    })
  } else t.parts = parts(n)
  // How many of a region's lines are drawn is the project's preset to decide,
  // and a surplus line is a notice rather than a refusal (CLIP-147). What stays
  // refused is a region element whose text sits outside a row.
  if (region && !stored && t.parts.some((p) => p.field || trimCompositionSpace(p.literal)))
    problem(n, 'invalid_skeleton')
  for (const row of [{ kind, parts: t.parts }, ...t.rows]) {
    const max = row.kind === 'ai' ? l.guideChars : l.copyChars
    if (row.parts.reduce((n, p) => n + scalarLength(p.literal), 0) > max) problem(n, 'copy_limit')
  }
  return t
}

export function parseClipComposition(
  source: string,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): ClipComposition {
  return readClipComposition(source, limits, false)
}

export function readStoredClipComposition(
  source: string,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): ClipComposition {
  return readClipComposition(source, limits, true)
}

/** The grammar a TEMPLATE body must satisfy before it is saved (CLIP-4,
 * CLIP-59): fixed regions, the badge, fields, groups and guides. Footage
 * sections, scene-bound text and cut-relative timing are refused with the
 * construct named, mirroring the server's ParseTemplate; frozen project
 * snapshots keep reading through parseClipComposition (CLIP-140). */
export function parseClipTemplate(
  source: string,
  limits: CompositionLimits = CLIP_COMPOSITION_LIMITS,
): ClipComposition {
  return readClipComposition(source, limits, false, true)
}

function readClipComposition(
  source: string,
  limits: CompositionLimits,
  stored: boolean,
  template = false,
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
  attributes(
    root,
    'version',
    'intro',
    'caption',
    'outro',
    'accent',
    'pace',
    ...(stored ? ['styles'] : []),
  )
  if (root.attributes.version !== '1') problem(root, 'unknown_version')
  // `intro`, `caption` and `outro` are still accepted on the root so every body
  // saved before CLIP r33 opens, and they decide nothing: the presets a clip
  // renders in are the project's (CLIP-14, CLIP-139, CLIP-144).
  const d: ClipComposition = {
    source,
    root,
    accent: root.attributes.accent ?? '',
    pace: root.attributes.pace ?? 'steady',
    fields: [],
    groups: [],
    guidance: [],
    stages: [],
    sections: [],
    elements: [],
    outline: [],
    maxima: {},
    minima: {},
  }
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
    attributes(n, 'id', 'label', 'required', 'chars')
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
      chars: declaredChars(n, n, limits.answerChars),
      span: n.span,
    })
    if (d.fields.length > limits.fields) problem(n, 'field_limit')
  }
  for (const n of ns) {
    if (n.name === 'field') field(n)
    if (n.name === 'group') {
      attributes(n, 'id', 'label', 'min', 'max')
      claim(n)
      const group = n.attributes.id
      if (group === 'scenes') problem(n, 'invalid_id')
      const label = n.attributes.label ?? ''
      if (scalarLength(label) > limits.labelChars) problem(n, 'field_limit')
      const bound = (key: string, fallback: number) => {
        const raw = n.attributes[key]
        if (raw === undefined) return fallback
        const value = Number(raw)
        if (!raw || /[^0-9]/.test(raw) || !Number.isSafeInteger(value) || value > limits.items)
          problem(n, 'invalid_item_bounds')
        return value
      }
      const min = bound('min', 0),
        max = bound('max', limits.items)
      if (min > max) problem(n, 'invalid_item_bounds')
      d.groups.push({ id: group, label, min, max, span: n.span })
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
  /** One named composition stage (CLIP-141): a short name and one line of intent,
   * bounded by the counts a label and a prompt already have and capped in number
   * so a body cannot script the clip stage by stage. A stage is a property of the
   * whole clip, so it lives at the root only — inside a scene or a repetition
   * `stage` stays an unknown tag. */
  const stage = (n: CompositionNode) => {
    attributes(n, 'name')
    const name = n.attributes.name ?? '',
      intent = content(n)
    if (
      !trimCompositionSpace(name) ||
      !trimCompositionSpace(intent) ||
      scalarLength(name) > limits.labelChars ||
      scalarLength(intent) > limits.promptChars
    )
      problem(n, 'stage_limit')
    d.stages.push({ name, intent, span: n.span })
    if (d.stages.length > limits.stages) problem(n, 'stage_limit')
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
        s.elements.push(readElement(c, d, scope, repeat, true, limits, stored, template))
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
      case 'stage':
        stage(n)
        d.outline.push({ kind: 'stage', index: d.stages.length - 1 })
        break
      case 'text':
        claim(n)
        d.elements.push(readElement(n, d, 'context', '', false, limits, stored, template))
        d.outline.push({ kind: 'text', index: d.elements.length - 1 })
        break
      case 'scene':
        if (template) problem(n, 'unsupported_section')
        section(n)
        break
      case 'repeat': {
        if (template) problem(n, 'unsupported_section')
        attributes(n, 'for')
        const over = n.attributes.for
        if (over !== 'scenes' && !d.groups.some((g) => g.id === over)) problem(n, 'unknown_repeat')
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
  d.maxima = fieldMaxima(d, limits)
  d.minima = groupMinima(d)
  return d
}

/** Gives every repeated item group the number of items it actually admits
 * (CLIP-119): its declared minimum, else one when any field of the group is
 * required, else zero. A required field that no item carries is satisfied by
 * nothing, so a group holding one admits at least one item even when the
 * template that saved it declared no minimum. */
function groupMinima(d: ClipComposition) {
  const required = new Set(d.fields.filter((f) => f.group && f.required).map((f) => f.group))
  const out: Record<string, number> = {}
  for (const g of d.groups) out[g.id] = g.min || (required.has(g.id) ? 1 : 0)
  return out
}

/** Folds every position a field's value reaches into one number (CLIP-117). A
 * position contributes the maximum it actually enforces — its authored value
 * when it declares one, otherwise the count its rendered position imposes — and
 * a position that imposes none contributes nothing. */
function fieldMaxima(d: ClipComposition, l: CompositionLimits) {
  const out: Record<string, number> = {}
  for (const f of d.fields)
    out[f.group ? `${f.group}.${f.id}` : f.id] =
      f.chars > 0 ? Math.min(f.chars, l.answerChars) : l.answerChars
  const fold = (parts: CompositionPart[], limit: number) => {
    if (limit <= 0) return
    for (const p of parts)
      if (p.field && p.field in out && limit < out[p.field]) out[p.field] = limit
  }
  // Only a position the ANSWER reaches contributes (CLIP-117). In an `ai` position the answer
  // is material the writer reads, not text that lands there, so that position's bound belongs
  // to what the model writes (CLIP-118) — folding it in capped a field at the length of the
  // line the model writes FROM it.
  const visit = (t: CompositionElement) => {
    if (!t.rows.length) {
      if (t.kind === 'ai') return
      return fold(t.parts, t.chars || compositionPositionChars(t.role))
    }
    // A region slot's own count belongs to the preset the PROJECT chose
    // (CLIP-147), so an undeclared region row contributes none here; the drawn
    // position still holds the text to its count when it renders (CDS-77).
    t.rows.forEach((row, index) => {
      if ((row.kind || t.kind) === 'ai') return
      fold(row.parts, row.chars || compositionPositionChars(t.role, { role: row.role, index }))
    })
  }
  d.elements.forEach(visit)
  for (const section of d.sections) section.elements.forEach(visit)
  return out
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
