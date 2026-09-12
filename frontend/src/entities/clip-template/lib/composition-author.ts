import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import { CompositionProblem, type CompositionNode } from '../model/composition'
import { readCompositionXML, serializeCompositionNode } from './composition-xml'

/** Syntax-only reading keeps incomplete control edits recoverable; the strict parser gates save. */
export function compositionDraftTree(source: string) {
  if (
    source.length > CLIP_COMPOSITION_LIMITS.sourceChars * 2 ||
    Array.from(source).length > CLIP_COMPOSITION_LIMITS.sourceChars
  )
    throw new CompositionProblem('clip', 1, 'source_limit')
  return readCompositionXML(source, CLIP_COMPOSITION_LIMITS)
}

export function compositionNode(
  name: string,
  attributes: Record<string, string> = {},
  children: CompositionNode[] = [],
  text = '',
): CompositionNode {
  return { name, attributes, children, text, span: { start: 0, end: 0, line: 1 } }
}
export const compositionLiteral = (text: string) => compositionNode('#text', {}, [], text)

/** Only an edited subtree is serialized; unrelated source bytes and literal whitespace survive. */
export function patchCompositionSource(
  source: string,
  target: CompositionNode,
  replacement: CompositionNode | null,
): string {
  const chars = Array.from(source)
  return (
    chars.slice(0, target.span.start).join('') +
    (replacement ? serializeCompositionNode(replacement) : '') +
    chars.slice(target.span.end).join('')
  )
}

export interface CompositionOutlineRow {
  key: string
  node: CompositionNode
  parent: CompositionNode
  group: string
  inScene: boolean
  scope: string
  repeat: string
}
export function compositionOutline(root: CompositionNode): CompositionOutlineRow[] {
  const rows: CompositionOutlineRow[] = []
  const walk = (
    parent: CompositionNode,
    path: string,
    group: string,
    inScene: boolean,
    scope: string,
    repeat: string,
  ) => {
    parent.children.forEach((node, i) => {
      if (node.name === '#text') return
      const key = `${path}.${i}`
      rows.push({ key, node, parent, group, inScene, scope, repeat })
      if (['group', 'scene', 'repeat'].includes(node.name))
        walk(
          node,
          key,
          node.name === 'group' ? node.attributes.id : group,
          inScene || node.name === 'scene',
          node.name === 'scene' ? (node.attributes.scope ?? 'scene') : scope,
          node.name === 'repeat' ? node.attributes.for : repeat,
        )
    })
  }
  walk(root, 'clip', '', false, '', '')
  return rows
}

export function newCompositionNode(name: string, label: string, inScene = false): CompositionNode {
  const id = `${name}_${crypto.randomUUID().replaceAll('-', '')}`
  switch (name) {
    case 'field':
      return compositionNode(name, { id, label, required: 'false' })
    case 'group':
      return compositionNode(name, { id }, [newCompositionNode('field', label)])
    case 'scene':
      return compositionNode(name, { id, scope: 'scene' })
    case 'repeat':
      return compositionNode(name, { for: 'scenes' }, [newCompositionNode('scene', label)])
    case 'guide':
      return compositionNode(name, {}, [compositionLiteral('')])
    default:
      return compositionNode(
        'text',
        { id, kind: 'fixed', role: 'caption', basis: inScene ? 'cut' : 'whole' },
        [compositionLiteral('')],
      )
  }
}

export const EMPTY_CLIP_COMPOSITION =
  '<clip version="1" styles="clean" pace="steady">\n  <scene id="footage" scope="scene"/>\n</clip>'
