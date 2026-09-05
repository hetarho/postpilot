import { encode, parse, type ParseFailure, type SlotKind, type TemplateNode } from '../lib/grammar'

/** What the composition editor manipulates: a flat list of blocks, with `repeat` the only one
 *  that nests (the grammar forbids a repeat inside a repeat).
 *
 *  This is a VIEW over the body, not a second source of truth. `toBody` serializes it and the
 *  body is what gets saved; `fromBody` reads it back. The pair round-trips byte for byte for
 *  anything the builder produced, which is what change 25 AC8 asks for — and it is the only
 *  way a body is authored, since the grammar itself is never shown to anyone (change 30). */
export type BuilderBlock =
  | { id: string; kind: 'write'; text: string }
  | { id: string; kind: 'text'; text: string }
  | { id: string; kind: 'slot'; slotKind: SlotKind; label: string }
  | { id: string; kind: 'note'; text: string }
  | { id: string; kind: 'repeat'; children: BuilderBlock[] }

export type BuilderBlockKind = BuilderBlock['kind'] | 'photo' | 'place' | 'link'

let sequence = 0
/** Local identity for React keys and for the reorder calls. It never reaches the body: two
 *  identical blocks must still be two rows, and the body has no place to keep an id. */
export function nextBlockId(): string {
  sequence += 1
  return `b${sequence}`
}

export function newBlock(kind: BuilderBlockKind): BuilderBlock {
  const id = nextBlockId()
  switch (kind) {
    case 'write':
      return { id, kind: 'write', text: '' }
    case 'text':
      return { id, kind: 'text', text: '' }
    case 'note':
      return { id, kind: 'note', text: '' }
    case 'repeat':
      return { id, kind: 'repeat', children: [] }
    case 'photo':
    case 'place':
    case 'link':
      return { id, kind: 'slot', slotKind: kind, label: '' }
    case 'slot':
      return { id, kind: 'slot', slotKind: 'place', label: '' }
  }
}

/** One block's source. A slot's label and a write's instruction are user text, so both are
 *  escaped on the way in — the parser decodes them on the way back out. */
function blockSource(block: BuilderBlock): string {
  switch (block.kind) {
    case 'write':
      return `<write>${encode(block.text)}</write>`
    case 'note':
      return `<note>${encode(block.text)}</note>`
    case 'text':
      // A literal is the one block whose text is NOT escaped as a tag body: it is the body's
      // own prose. Only a `<` that would start a tag needs hiding.
      return block.text.replaceAll('<', '&lt;')
    case 'slot': {
      // `encode` escapes the double quote too, which matters here and nowhere else: the value
      // is quoted, and a label like `네이버 "지도"` is ordinary free text a person types. Without
      // it the builder emitted a body its own parser refused, and the editor fell into source
      // mode on a valid keystroke.
      const label = block.label.trim()
      const labelAttr = label === '' ? '' : ` label="${encode(label)}"`
      return `<slot kind="${block.slotKind}"${labelAttr}/>`
    }
    case 'repeat':
      return `<repeat each="photo">\n${block.children.map(blockSource).join('\n')}\n</repeat>`
  }
}

/** Blocks are joined by ONE newline. That is the canonical form: `fromBody` strips exactly one
 *  newline from each side of a literal, so the pair is each other's inverse. A blank line the
 *  author wants lives inside a text block. */
export function toBody(blocks: readonly BuilderBlock[]): string {
  return blocks.map(blockSource).join('\n')
}

function stripOneNewline(value: string): string {
  let out = value
  if (out.startsWith('\n')) out = out.slice(1)
  if (out.endsWith('\n')) out = out.slice(0, -1)
  // Whitespace between two tags is separator, never content: a repeat with no children
  // serializes to an open/close pair around a newline, and that must not read back as a row.
  return out.trim() === '' ? '' : out
}

function fromNodes(
  nodes: readonly TemplateNode[],
  decodeText: (raw: string) => string,
): BuilderBlock[] {
  const blocks: BuilderBlock[] = []
  for (const node of nodes) {
    switch (node.kind) {
      case 'literal': {
        const text = stripOneNewline(node.text ?? '')
        // A literal that was only the separator between two tags carries no content of its
        // own, so it becomes no row rather than an empty one.
        if (text === '') break
        blocks.push({ id: nextBlockId(), kind: 'text', text: decodeText(text) })
        break
      }
      case 'write':
        blocks.push({ id: nextBlockId(), kind: 'write', text: decodeText(node.text ?? '') })
        break
      case 'note':
        blocks.push({ id: nextBlockId(), kind: 'note', text: decodeText(node.text ?? '') })
        break
      case 'slot':
        blocks.push({
          id: nextBlockId(),
          kind: 'slot',
          slotKind: node.slotKind ?? 'place',
          label: decodeText(node.label ?? ''),
        })
        break
      case 'repeat':
        blocks.push({
          id: nextBlockId(),
          kind: 'repeat',
          children: fromNodes(node.children ?? [], decodeText),
        })
        break
    }
  }
  return blocks
}

export type BodyRead = { ok: true; blocks: BuilderBlock[] } | { ok: false; failure: ParseFailure }

export function fromBody(body: string, decodeText: (raw: string) => string): BodyRead {
  const result = parse(body)
  if (!result.ok) return { ok: false, failure: result.failure }
  return { ok: true, blocks: fromNodes(result.nodes, decodeText) }
}

/** Moves `from` so that it sits at `to`, clamped. Splice-based rather than swap-based: a drag
 *  across four rows is one move, not three swaps. */
export function reorder<T>(items: readonly T[], from: number, to: number): T[] {
  const next = [...items]
  const clamped = Math.max(0, Math.min(next.length - 1, to))
  const [moved] = next.splice(from, 1)
  next.splice(clamped, 0, moved)
  return next
}

/** Whether a block contributes anything to the body yet.
 *
 *  The structure editor keeps a row for a block the moment it is added, before anything is
 *  typed into it. An empty `<write></write>` does NOT parse — deliberately, since a write with
 *  no instruction has nothing to ask for — so an incomplete row is held in the editor's own
 *  state and left out of the body until it says something. Without this the builder would
 *  produce a body its own parser refuses, the instant a block is added. */
export function isCompleteBlock(block: BuilderBlock): boolean {
  switch (block.kind) {
    case 'write':
    case 'note':
    case 'text':
      return block.text.trim() !== ''
    case 'slot':
    case 'repeat':
      return true
  }
}

/** The body a block list contributes, incomplete rows omitted. */
export function toValidBody(blocks: readonly BuilderBlock[]): string {
  return toBody(
    blocks
      .filter(isCompleteBlock)
      .map((block) =>
        block.kind === 'repeat'
          ? { ...block, children: block.children.filter(isCompleteBlock) }
          : block,
      ),
  )
}

/** What a person actually picks from the palette. `slot` is deliberately absent: the three slot
 *  KINDS are the choices, and a bare "slot" would be a command with no meaning — which is also
 *  why it is the one `BuilderBlockKind` with no copy of its own. */
export type PaletteKind = 'write' | 'text' | 'photo' | 'place' | 'link' | 'note' | 'repeat'

/** The kind key a row shows, in that same vocabulary — so one set of strings names the button
 *  that creates a block and the badge that identifies it afterwards. */
export function blockKindKey(block: BuilderBlock): PaletteKind {
  return block.kind === 'slot' ? block.slotKind : block.kind
}

/** The one line a collapsed row shows for a block: the block's own text and nothing else.
 *
 *  Never the grammar. The composition is read as an outline of the post, so what a row shows is
 *  what would end up on the page — a `<write>`'s instruction, a literal's prose, a slot's label —
 *  and never the tags that carry them (change 30 A9).
 *
 *  Newlines collapse to spaces because the row is one line: a literal holding a paragraph break
 *  would otherwise silently render as one line with a gap in it. A block with nothing typed yet
 *  returns "", and the row says so in its own words rather than showing an empty line.
 */
export function blockSummary(block: BuilderBlock): string {
  switch (block.kind) {
    case 'write':
    case 'note':
    case 'text':
      return block.text.replace(/\s+/g, ' ').trim()
    case 'slot':
      return block.label.replace(/\s+/g, ' ').trim()
    case 'repeat':
      // A repeat's content IS its children, and they are rows of their own directly beneath it.
      return ''
  }
}

/** One block, with everything needed to ADDRESS it: which group it belongs to and where in that
 *  group it sits. */
export interface OutlineRow {
  block: BuilderBlock
  /** 0 at the top level, 1 inside a repeat. The grammar allows no third level. */
  depth: number
  /** The repeat this row belongs to, or null at the top level. */
  parentId: string | null
  /** Its index among its siblings — what a reorder and an insertion address. */
  index: number
}

/** Every block in reading order, parents before their children.
 *
 *  It is an addressing projection, not the render shape: the editor renders a repeat's children
 *  as a nested list so a reorder cannot take a block out of its repeat. This exists so a lookup
 *  by block id — "where is this, and what is next to it" — is one pass over the tree rather than
 *  a recursive search repeated at every call site. */
export function outline(blocks: readonly BuilderBlock[]): OutlineRow[] {
  const rows: OutlineRow[] = []
  blocks.forEach((block, index) => {
    rows.push({ block, depth: 0, parentId: null, index })
    if (block.kind === 'repeat') {
      block.children.forEach((child, childIndex) => {
        rows.push({ block: child, depth: 1, parentId: block.id, index: childIndex })
      })
    }
  })
  return rows
}

/** Where the next block goes. A null parent is the top level; a parent id is inside that repeat.
 *  `index` is the position among that parent's children, so `children.length` is "at the end". */
export interface Position {
  parentId: string | null
  index: number
}

export function endPosition(blocks: readonly BuilderBlock[]): Position {
  return { parentId: null, index: blocks.length }
}

/** The position a row leaves behind once it is touched: directly after it, and INSIDE its repeat
 *  when it is a child of one. This is what makes one toolbar unambiguous — the user's last action
 *  is what says where the next block belongs (change 30 A7).
 *
 *  Touching the repeat's own row aims INSIDE it rather than after it: a repeat exists to hold
 *  children, so the block that follows selecting one is almost always its first child. */
export function positionAfter(blocks: readonly BuilderBlock[], blockId: string): Position | null {
  const row = outline(blocks).find((candidate) => candidate.block.id === blockId)
  if (!row) return null
  if (row.block.kind === 'repeat')
    return { parentId: row.block.id, index: row.block.children.length }
  return { parentId: row.parentId, index: row.index + 1 }
}

/** Whether a kind may be inserted at a position. The grammar forbids a repeat inside a repeat, and
 *  that rule lives HERE rather than in the palette: hiding the button is the affordance, and this
 *  is the enforcement — so a position that drifts cannot produce a body the parser refuses. */
export function canInsert(kind: BuilderBlockKind, position: Position): boolean {
  return !(kind === 'repeat' && position.parentId !== null)
}

/** Inserts one new block at a position, returning the new list and the block itself so the caller
 *  can move the insertion point past it and open it for editing.
 *
 *  A refused insertion (a repeat inside a repeat) or an unknown parent returns the list unchanged
 *  and no block, rather than falling back to the end: silently putting a block somewhere other
 *  than where the screen said it would go is the one failure a single toolbar cannot afford. */
export function insertAt(
  blocks: readonly BuilderBlock[],
  position: Position,
  kind: BuilderBlockKind,
): { blocks: BuilderBlock[]; inserted: BuilderBlock | null } {
  if (!canInsert(kind, position)) return { blocks: [...blocks], inserted: null }
  const inserted = newBlock(kind)
  if (position.parentId === null) {
    const next = [...blocks]
    next.splice(clampIndex(position.index, blocks.length), 0, inserted)
    return { blocks: next, inserted }
  }
  let found = false
  const next = blocks.map((block) => {
    if (block.id !== position.parentId || block.kind !== 'repeat') return block
    found = true
    const children = [...block.children]
    children.splice(clampIndex(position.index, block.children.length), 0, inserted)
    return { ...block, children }
  })
  return found ? { blocks: next, inserted } : { blocks: [...blocks], inserted: null }
}

function clampIndex(index: number, length: number): number {
  return Math.max(0, Math.min(length, index))
}
