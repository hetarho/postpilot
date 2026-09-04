import { twMerge } from 'tailwind-merge'
import type { TemplateRef } from '../model/types'

/** The 템플릿 a post is assigned to, as text beside the voice.
 *
 *  Renders nothing at all when the post has none: 없음 is the default, so a label on every
 *  unassigned row would be noise on the majority of the list. `min-w-0 truncate` because the
 *  name is user text in a flex row (design-language §8.5). */
export function TemplateRefLabel({
  template,
  className,
}: {
  template: Pick<TemplateRef, 'id' | 'name'>
  className?: string
}) {
  if (!template.id) return null
  return (
    <span className={twMerge('min-w-0 truncate', className)}>{template.name || template.id}</span>
  )
}
