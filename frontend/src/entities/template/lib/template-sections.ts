import { savedHeadingLines } from './heading-references'
import { askFields } from './grammar'
import { TEMPLATE_PARSE_OPTIONS, type Template } from '../model/types'

/** The headings a template writes, else the labels of what it asks for. */
export function templateSections(template: Pick<Template, 'body'>, max: number): string[] {
  const headings = savedHeadingLines([template])
  const lines = headings.length
    ? headings
    : askFields(template.body, TEMPLATE_PARSE_OPTIONS).map((field) => field.label)
  return [...new Set(lines)].slice(0, max)
}
