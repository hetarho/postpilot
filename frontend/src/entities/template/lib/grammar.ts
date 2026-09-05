/** The template grammar, client side (spec/legacy/tech/post-template-grammar.md).
 *
 *  This is a second implementation of one grammar — the authoritative parser is Go, in
 *  `backend/internal/template`. It exists because the builder has to parse and re-serialize
 *  while the user types, and a round trip through the server on every keystroke is not that.
 *
 *  The two are kept honest by ONE shared fixture file,
 *  `backend/internal/template/testdata/grammar/cases.json`, which both test suites read. A
 *  new rule lands there first. The server stays authoritative on a save. */

export type NodeKind = 'literal' | 'write' | 'slot' | 'note' | 'repeat'
export type SlotKind = 'photo' | 'place' | 'link'

const SLOT_KINDS: readonly string[] = ['photo', 'place', 'link']
/** Attached photos are the only countable material a post has. */
const EACH_VALUES: readonly string[] = ['photo']
const TAG_NAMES: readonly string[] = ['write', 'slot', 'note', 'repeat']

/** One parsed construct. `source` is the node's exact source slice and serialization re-emits
 *  it verbatim, which is what makes `serialize(parse(body)) === body` by construction — for a
 *  hand-written body as much as for a builder-produced one, with no canonical formatting pass
 *  that would reflow the author's own spacing.
 *
 *  `text` and `label` hold RAW inner slices; `decode` resolves the entities when a node's text
 *  is read for display, so a bare `&` in prose round-trips unchanged. */
export interface TemplateNode {
  kind: NodeKind
  source: string
  line: number
  /** write · note */
  text?: string
  /** slot */
  slotKind?: SlotKind
  label?: string
  /** repeat */
  each?: string
  children?: TemplateNode[]
}

export type ParseReason =
  | 'unknown_tag'
  | 'unclosed_tag'
  | 'unexpected_close'
  | 'malformed_tag'
  | 'missing_attribute'
  | 'unknown_slot_kind'
  | 'unknown_repeat_each'
  | 'nested_repeat'
  | 'empty_write'
  | 'empty_note'

export interface ParseFailure {
  /** 1-based, so it matches what the source editor shows. */
  line: number
  reason: ParseReason
}

export type ParseResult = { ok: true; nodes: TemplateNode[] } | { ok: false; failure: ParseFailure }

/** The ONE definition of "this text says nothing", shared with the Go parser.
 *
 *  It cannot be `.trim()` on one side and `strings.TrimSpace` on the other: the two disagree in
 *  both directions — Go's table has U+0085 and not U+FEFF, JavaScript's has U+FEFF and not
 *  U+0085 — so `<write>\uFEFF</write>` was refused here and accepted on the server. This is the
 *  union of the two, plus the zero-width characters a paste can carry; the shared fixtures pin
 *  any drift between the two copies. */
const BLANK = new Set([
  0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004,
  0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x200b, 0x200c, 0x200d, 0x2028, 0x2029, 0x202f,
  0x205f, 0x3000, 0xfeff,
])

function isBlank(value: string): boolean {
  for (const char of value) {
    if (!BLANK.has(char.codePointAt(0) ?? 0)) return false
  }
  return true
}

/** Resolves the entities the grammar recognizes. `&amp;` is resolved last so an escaped escape
 *  (`&amp;lt;`) decodes to the literal text `&lt;` rather than to `<`.
 *
 *  `&quot;` is in the set because an attribute value is quoted: a slot label like `네이버 "지도"`
 *  is ordinary free text a person types, and without an escape for it the builder would emit a
 *  body its own parser refuses. */
export function decode(raw: string): string {
  return raw
    .replaceAll('&lt;', '<')
    .replaceAll('&gt;', '>')
    .replaceAll('&quot;', '"')
    .replaceAll('&amp;', '&')
}

/** The inverse, for text the builder puts INTO a body. `>` is unambiguous on its own, so
 *  escaping it would churn bodies that never needed it. */
export function encode(text: string): string {
  return text.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('"', '&quot;')
}

export function serialize(nodes: readonly TemplateNode[]): string {
  return nodes.map((node) => node.source).join('')
}

export function parse(body: string): ParseResult {
  const scan = parseNodes(body, 0, false)
  if (!scan.ok) return scan
  if (scan.end !== body.length) {
    return { ok: false, failure: { line: lineAt(body, scan.end), reason: 'unexpected_close' } }
  }
  return { ok: true, nodes: scan.nodes }
}

type Scan = { ok: true; nodes: TemplateNode[]; end: number } | { ok: false; failure: ParseFailure }

function parseNodes(body: string, from: number, inRepeat: boolean): Scan {
  const nodes: TemplateNode[] = []
  let literalStart = from
  let i = from

  const flushLiteral = (upTo: number) => {
    if (upTo > literalStart) {
      const raw = body.slice(literalStart, upTo)
      nodes.push({ kind: 'literal', source: raw, line: lineAt(body, literalStart), text: raw })
    }
  }

  while (i < body.length) {
    const next = body.indexOf('<', i)
    if (next < 0) break
    const tag = tagNameAt(body, next)
    if (!tag) {
      // A `<` followed by whitespace, a digit or punctuation is literal prose, so `3 < 5`
      // needs no escape.
      i = next + 1
      continue
    }
    if (!TAG_NAMES.includes(tag.name)) {
      return { ok: false, failure: { line: lineAt(body, next), reason: 'unknown_tag' } }
    }
    if (tag.isClose) {
      flushLiteral(next)
      return { ok: true, nodes, end: next }
    }
    flushLiteral(next)
    const parsed = parseTag(body, next, tag.name, inRepeat)
    if (!parsed.ok) return parsed
    nodes.push(parsed.node)
    i = parsed.after
    literalStart = parsed.after
  }
  flushLiteral(body.length)
  return { ok: true, nodes, end: body.length }
}

type TagResult =
  { ok: true; node: TemplateNode; after: number } | { ok: false; failure: ParseFailure }

function parseTag(body: string, at: number, name: string, inRepeat: boolean): TagResult {
  const line = lineAt(body, at)
  const head = parseTagHead(body, at, name)
  if (!head.ok) return head

  if (name === 'slot') {
    if (!head.selfClosing) {
      // `<slot ...></slot>` — a slot reserves a position, it does not wrap content, so a
      // closing tag means the author expected different semantics.
      return { ok: false, failure: { line, reason: 'malformed_tag' } }
    }
    const rawKind = head.attrs.get('kind')
    if (rawKind === undefined) {
      return { ok: false, failure: { line, reason: 'missing_attribute' } }
    }
    const kind = decode(rawKind)
    if (!SLOT_KINDS.includes(kind)) {
      return { ok: false, failure: { line, reason: 'unknown_slot_kind' } }
    }
    return {
      ok: true,
      node: {
        kind: 'slot',
        source: body.slice(at, head.after),
        line,
        slotKind: kind as SlotKind,
        label: head.attrs.get('label') ?? '',
      },
      after: head.after,
    }
  }

  if (name === 'write' || name === 'note') {
    if (head.selfClosing) return { ok: false, failure: { line, reason: 'malformed_tag' } }
    const inner = readTextBody(body, head.after, name, line)
    if (!inner.ok) return inner
    if (isBlank(decode(inner.text))) {
      return {
        ok: false,
        failure: { line, reason: name === 'note' ? 'empty_note' : 'empty_write' },
      }
    }
    return {
      ok: true,
      node: { kind: name, source: body.slice(at, inner.after), line, text: inner.text },
      after: inner.after,
    }
  }

  // repeat
  if (head.selfClosing) return { ok: false, failure: { line, reason: 'malformed_tag' } }
  if (inRepeat) return { ok: false, failure: { line, reason: 'nested_repeat' } }
  const rawEach = head.attrs.get('each')
  if (rawEach === undefined) return { ok: false, failure: { line, reason: 'missing_attribute' } }
  const each = decode(rawEach)
  if (!EACH_VALUES.includes(each)) {
    return { ok: false, failure: { line, reason: 'unknown_repeat_each' } }
  }
  const inner = parseNodes(body, head.after, true)
  if (!inner.ok) return inner
  const close = consumeClose(body, inner.end, name, line)
  if (!close.ok) return close
  return {
    ok: true,
    node: {
      kind: 'repeat',
      source: body.slice(at, close.after),
      line,
      each,
      children: inner.nodes,
    },
    after: close.after,
  }
}

type HeadResult =
  | { ok: true; attrs: Map<string, string>; selfClosing: boolean; after: number }
  | { ok: false; failure: ParseFailure }

/** Reads one opening tag's attribute list. A bare `key=value` is refused: accepting it would
 *  make `kind=photo/>` ambiguous about whether the slash is part of the value. */
function parseTagHead(body: string, at: number, name: string): HeadResult {
  const line = lineAt(body, at)
  const attrs = new Map<string, string>()
  let i = at + 1 + name.length
  while (i < body.length) {
    while (i < body.length && isSpace(body[i])) i += 1
    if (i >= body.length) break
    if (body[i] === '>') return { ok: true, attrs, selfClosing: false, after: i + 1 }
    if (body[i] === '/') {
      if (body[i + 1] === '>') return { ok: true, attrs, selfClosing: true, after: i + 2 }
      return { ok: false, failure: { line, reason: 'malformed_tag' } }
    }
    const keyStart = i
    while (i < body.length && isAttrNameChar(body[i])) i += 1
    if (i === keyStart || i >= body.length || body[i] !== '=') {
      return { ok: false, failure: { line, reason: 'malformed_tag' } }
    }
    const key = body.slice(keyStart, i)
    i += 1
    const quote = body[i]
    if (quote !== '"' && quote !== "'") {
      return { ok: false, failure: { line, reason: 'malformed_tag' } }
    }
    i += 1
    const valueStart = i
    while (i < body.length && body[i] !== quote) i += 1
    if (i >= body.length) return { ok: false, failure: { line, reason: 'malformed_tag' } }
    attrs.set(key, body.slice(valueStart, i))
    i += 1
  }
  return { ok: false, failure: { line, reason: 'unclosed_tag' } }
}

type TextResult = { ok: true; text: string; after: number } | { ok: false; failure: ParseFailure }

/** Reads the inner text of a write or note. A known tag inside it is a malformed tag rather
 *  than a nested node: neither construct wraps content, so `<write>a <write>b` is a mistake
 *  with no reasonable reading. */
function readTextBody(body: string, from: number, name: string, openLine: number): TextResult {
  let i = from
  while (i < body.length) {
    const next = body.indexOf('<', i)
    if (next < 0) break
    const tag = tagNameAt(body, next)
    if (!tag) {
      i = next + 1
      continue
    }
    if (!tag.isClose) {
      return { ok: false, failure: { line: lineAt(body, next), reason: 'malformed_tag' } }
    }
    if (tag.name !== name) {
      return { ok: false, failure: { line: lineAt(body, next), reason: 'unexpected_close' } }
    }
    return { ok: true, text: body.slice(from, next), after: next + tag.name.length + 3 }
  }
  return { ok: false, failure: { line: openLine, reason: 'unclosed_tag' } }
}

type CloseResult = { ok: true; after: number } | { ok: false; failure: ParseFailure }

/** Steps over the closing tag that stopped a child parse, and reports the OPENING tag's line
 *  when nothing closed it — the line the author has to go fix. */
function consumeClose(body: string, at: number, name: string, openLine: number): CloseResult {
  if (at >= body.length) return { ok: false, failure: { line: openLine, reason: 'unclosed_tag' } }
  const tag = tagNameAt(body, at)
  if (!tag || !tag.isClose) {
    return { ok: false, failure: { line: openLine, reason: 'unclosed_tag' } }
  }
  if (tag.name !== name) {
    return { ok: false, failure: { line: lineAt(body, at), reason: 'unexpected_close' } }
  }
  return { ok: true, after: at + tag.name.length + 3 }
}

/** Decides whether the `<` at `at` opens or closes a tag. A name must be followed by a
 *  delimiter, so `<writer>` reads as an unknown tag rather than as `write` plus stray text. */
function tagNameAt(body: string, at: number): { name: string; isClose: boolean } | null {
  let i = at + 1
  let isClose = false
  if (body[i] === '/') {
    isClose = true
    i += 1
  }
  const start = i
  while (i < body.length && isTagNameChar(body[i])) i += 1
  if (i === start || i >= body.length) return null
  const name = body.slice(start, i)
  if (isClose) return body[i] === '>' ? { name, isClose } : null
  if (!isSpace(body[i]) && body[i] !== '>' && body[i] !== '/') return null
  return { name, isClose }
}

function lineAt(body: string, offset: number): number {
  const upTo = Math.min(offset, body.length)
  let lines = 1
  for (let i = 0; i < upTo; i += 1) if (body[i] === '\n') lines += 1
  return lines
}

const isSpace = (c: string) => c === ' ' || c === '\t' || c === '\n' || c === '\r'
const isTagNameChar = (c: string) => /[A-Za-z]/.test(c)
const isAttrNameChar = (c: string) => /[A-Za-z_-]/.test(c)
