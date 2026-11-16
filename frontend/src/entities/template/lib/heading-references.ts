import { HEADING_REFERENCES_MAX, TEMPLATE_ASK_LABEL_MAX_CHARS } from '../config'
import { TEMPLATE_PARSE_OPTIONS, type Template } from '../model/types'
import { decode, parse, type TemplateNode } from './grammar'

/** The heading lines of the owner's saved templates (TMPL-76): every line of a body's 고정 문구,
 *  trimmed, that is nonempty and short enough to be a heading, in directory order, each once, at
 *  most `HEADING_REFERENCES_MAX`. The title area is never read — it shapes the post's title, not
 *  its headings — and a body that does not parse gives none. */
export function savedHeadingLines(templates: readonly Pick<Template, 'body'>[]): string[] {
  const lines = new Set<string>()
  const visit = (nodes: readonly TemplateNode[]): boolean => {
    for (const node of nodes) {
      if (node.kind === 'literal') {
        for (const raw of decode(node.text ?? node.source).split('\n')) {
          const line = raw.trim()
          if (line === '' || Array.from(line).length > TEMPLATE_ASK_LABEL_MAX_CHARS) continue
          lines.add(line)
          if (lines.size === HEADING_REFERENCES_MAX) return false
        }
      }
      if (node.children && !visit(node.children)) return false
    }
    return true
  }
  for (const template of templates) {
    const parsed = parse(template.body, TEMPLATE_PARSE_OPTIONS)
    if (parsed.ok && !visit(parsed.nodes)) break
  }
  return [...lines]
}
