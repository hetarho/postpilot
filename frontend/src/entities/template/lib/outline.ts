import { decode, parse, type TemplateNode } from './grammar'
import { TEMPLATE_PARSE_OPTIONS } from '../model/types'

/** One line of a template's outline: what the post it makes holds at that point, in order. */
export type OutlineRow =
  | { kind: 'heading'; text: string }
  | { kind: 'photo'; count: number }
  | { kind: 'write'; text: string }
  | { kind: 'ask'; text: string }
  | { kind: 'repeat'; text: string; rows: OutlineRow[] }

/** A template's body read as the post's outline — headings in the template's own words, photo
 *  positions, what the AI writes and what the writer is asked — for a directory to show the SHAPE
 *  a template gives a post without opening it. A body that does not parse has no outline. */
export function templateOutline(body: string): OutlineRow[] {
  const parsed = parse(body, TEMPLATE_PARSE_OPTIONS)
  if (!parsed.ok) return []
  const visit = (nodes: readonly TemplateNode[]): OutlineRow[] =>
    nodes.flatMap((node): OutlineRow[] => {
      switch (node.kind) {
        case 'literal':
          return decode(node.text ?? node.source)
            .split('\n')
            .map((line) => line.trim())
            .filter((line) => line !== '')
            .map((text) => ({ kind: 'heading', text }))
        case 'slot':
          return node.slotKind === 'photo' ? [{ kind: 'photo', count: node.count || 1 }] : []
        case 'write':
          return [{ kind: 'write', text: decode(node.text ?? '').trim() }]
        case 'ask':
          return [{ kind: 'ask', text: decode(node.label ?? '').trim() }]
        case 'repeat':
          return [
            {
              kind: 'repeat',
              text: decode(node.each ?? '').trim(),
              rows: visit(node.children ?? []),
            },
          ]
        default:
          return []
      }
    })
  return visit(parsed.nodes)
}
