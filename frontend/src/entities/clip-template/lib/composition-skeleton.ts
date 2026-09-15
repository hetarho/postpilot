import { CLIP_REGIONS, CLIP_TIMING } from '@/shared/config'
import type { CompositionNode } from '../model/composition'
import {
  compositionDraftTree,
  compositionNode,
  compositionOutline,
  patchCompositionSource,
} from './composition-author'
import { serializeCompositionNode } from './composition-xml'

export type CompositionDesign = { intro: 'a' | 'b'; caption: 'bold'; outro: 'b' | 'e' }
export function compositionDesign(source: string): CompositionDesign {
  try {
    const a = compositionDraftTree(source).attributes
    return {
      intro: a.intro === 'a' ? 'a' : 'b',
      caption: 'bold',
      outro: a.outro === 'b' ? 'b' : 'e',
    }
  } catch {
    return { intro: 'b', caption: 'bold', outro: 'e' }
  }
}
export const isCompositionRegion = (node: CompositionNode) =>
  node.name === 'text' && (node.attributes.role === 'hook' || node.attributes.role === 'ending')

export function compositionSlotCount(role: string, design: CompositionDesign) {
  return role === 'hook'
    ? CLIP_REGIONS.intro[design.intro].slots.length
    : CLIP_REGIONS.outro[design.outro].slots.length
}
const emptyRow = () => compositionNode('row', { kind: 'fixed' })
const filled = (node: CompositionNode): boolean =>
  node.name === 'value' || !!node.text.trim() || node.children.some(filled)

/** Preserve row order, bindings and effective authorship, including excess nonempty rows.
 * Excess rows stay visible as invalid_skeleton until the owner edits them. */
export function regionRows(nodes: CompositionNode[], count: number) {
  const rows = nodes.flatMap((node) => {
    const kind = node.attributes.kind === 'ai' ? 'ai' : 'fixed'
    if (!node.children.some((c) => c.name === 'row'))
      return node.children.some(filled) ? [compositionNode('row', { kind }, node.children)] : []
    return node.children.flatMap((child) =>
      child.name === 'row'
        ? [compositionNode('row', { kind: child.attributes.kind || kind }, child.children)]
        : filled(child)
          ? [compositionNode('row', { kind }, [child])]
          : [],
    )
  })
  while (rows.length > count && !filled(rows.at(-1)!)) rows.pop()
  while (rows.length < count) rows.push(emptyRow())
  return rows
}
function region(
  role: 'hook' | 'ending',
  design: CompositionDesign,
  id: string,
  originals: CompositionNode[] = [],
) {
  const first = originals[0],
    basis = role === 'hook' ? 'output-start' : 'output-end'
  const sameBasis = first?.attributes.basis === basis
  return compositionNode(
    'text',
    {
      id: first?.attributes.id || id,
      role,
      kind: 'fixed',
      basis,
      start:
        sameBasis && first.attributes.start !== undefined
          ? first.attributes.start
          : role === 'hook'
            ? '0'
            : String(-CLIP_TIMING.outro_default_s),
      end:
        sameBasis && first.attributes.end !== undefined
          ? first.attributes.end
          : role === 'hook'
            ? String(CLIP_TIMING.intro_default_s)
            : '0',
    },
    regionRows(originals, compositionSlotCount(role, design)),
  )
}

export function compositionSkeleton(
  intro: CompositionDesign['intro'],
  outro: CompositionDesign['outro'],
) {
  const design: CompositionDesign = { intro, caption: 'bold', outro }
  return serializeCompositionNode(
    compositionNode('clip', { version: '1', ...design, pace: 'steady' }, [
      region('hook', design, 'intro'),
      compositionNode('scene', { id: 'footage', scope: 'scene' }),
      region('ending', design, 'outro'),
    ]),
  )
}

/** Only region subtrees and the opening root tag change; content bytes remain untouched. */
export function rebuildCompositionSkeleton(
  source: string,
  design = compositionDesign(source),
  only?: 'hook' | 'ending',
) {
  const root = compositionDraftTree(source)
  const outline = compositionOutline(root),
    replacements: { node: CompositionNode; next: CompositionNode | null }[] = []
  const added: CompositionNode[] = [],
    ids = new Set(outline.map((r) => r.node.attributes.id))
  for (const role of ['hook', 'ending'] as const) {
    if (only && role !== only) continue
    const matches = outline.filter((r) => r.node.name === 'text' && r.node.attributes.role === role)
    let id = role === 'hook' ? 'intro' : 'outro'
    while (ids.has(id)) id += '_'
    ids.add(id)
    const next = region(
      role,
      design,
      id,
      matches.map((r) => r.node),
    )
    const direct = matches.find((r) => r.parent === root)
    for (const match of matches)
      replacements.push({ node: match.node, next: match === direct ? next : null })
    if (!direct) added.push(next)
  }
  let result = source
  for (const { node, next } of replacements.sort((a, b) => b.node.span.start - a.node.span.start))
    result = patchCompositionSource(result, node, next)
  const updated = compositionDraftTree(result)
  if (added.length) {
    const chars = Array.from(result)
    let close = updated.span.end - 1
    while (close > updated.span.start && chars[close] !== '<') close--
    // A root without children may be self-closing.
    if (chars[close + 1] !== '/')
      result = patchCompositionSource(result, updated, {
        ...updated,
        children: [...updated.children, ...added],
      })
    else {
      const offset = close
      result =
        chars.slice(0, offset).join('') +
        '\n' +
        added.map(serializeCompositionNode).join('\n') +
        '\n' +
        chars.slice(offset).join('')
    }
  }
  const current = compositionDraftTree(result),
    chars = Array.from(result)
  let end = current.span.start,
    quote = ''
  for (; end < chars.length; end++) {
    const char = chars[end]
    if (quote) {
      if (char === quote) quote = ''
    } else if (char === '"' || char === "'") quote = char
    else if (char === '>') {
      end++
      break
    }
  }
  const attributes: Record<string, string> = { ...current.attributes, ...design }
  delete attributes.styles
  const opening = serializeCompositionNode(compositionNode('clip', attributes)).replace(/\/>$/, '>')
  return chars.slice(0, current.span.start).join('') + opening + chars.slice(end).join('')
}
