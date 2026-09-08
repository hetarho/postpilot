import {
  askFields,
  TEMPLATE_PARSE_OPTIONS,
  type AskField,
  type Template,
} from '@/entities/template'
import type { PostTemplateAnswer } from '@/entities/post'

/** One field as ① renders it: the title to ask under, the flavor it feeds, and what this post
 *  has answered so far. */
export interface AnswerField extends AskField {
  text: string
  enabled: boolean
}

/** The fields the selected template declares, in BODY order, each carrying this post's answer.
 *
 *  A label with no stored answer renders switched ON with empty text: the field exists because
 *  the template requires that data, and a blank one is dropped at the freeze anyway, so an
 *  off-by-default field would silently mean "never ask" (POST-62).
 *
 *  No template, a template with no fields, or a body that does not parse all yield nothing — ①
 *  is not where a broken template is fixed. */
export function answerFields(
  template: Template | undefined,
  answers: readonly PostTemplateAnswer[],
): AnswerField[] {
  if (!template) return []
  const stored = new Map(answers.map((answer) => [answer.label, answer]))
  return askFields(template.body, TEMPLATE_PARSE_OPTIONS).map((field) => {
    const answer = stored.get(field.label)
    return { ...field, text: answer?.text ?? '', enabled: answer?.enabled ?? true }
  })
}

/** What the draft queue carries. It is the whole current set rather than the one field that
 *  changed: every entry is an upsert of that label, so sending all of them cannot disturb an
 *  answer this screen is not showing — one typed under another template, say. */
export function toAnswerPatch(fields: readonly AnswerField[]): PostTemplateAnswer[] {
  return fields.map((field) => ({
    label: field.label,
    text: field.text,
    enabled: field.enabled,
  }))
}

/** Applies one edit by label, leaving every other field alone. */
export function withAnswer(
  fields: readonly AnswerField[],
  label: string,
  change: Partial<Pick<AnswerField, 'text' | 'enabled'>>,
): AnswerField[] {
  return fields.map((field) => (field.label === label ? { ...field, ...change } : field))
}
