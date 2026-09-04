import { encode, parse, type ParseFailure, type SlotKind, type TemplateNode } from '../lib/grammar'

/** What the structure editor manipulates: a flat list of blocks, with `repeat` the only one
 *  that nests (the grammar forbids a repeat inside a repeat).
 *
 *  This is a VIEW over the body, not a second source of truth. `toBody` serializes it and the
 *  body is what gets saved; `fromBody` reads it back. The pair round-trips byte for byte for
 *  anything the builder produced, which is what change 25 AC8 asks for — a hand-written body
 *  that parses opens here too, but its own spacing is normalized the moment it is edited
 *  visually, which is why the source mode exists. */
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
