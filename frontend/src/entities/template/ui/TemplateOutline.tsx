import { twMerge } from '@/shared/lib'
import { typographyStyles } from '@/shared/ui'
import { templateSections } from '../lib/template-sections'
import type { Template } from '../model/types'

/** The order a template gives a post, in one glance: its fixed headings, or — for a template that
 *  has none — what it asks the owner for, in order. Client work over the saved text alone (no
 *  request), drawn as the post's skeleton so a list of templates reads as a list of SHAPES.
 *  `TemplatePreview` is the full post; this is the template at a glance, wherever one is listed
 *  (home's kit, the templates list). */
export function TemplateOutline({
  template,
  max = 4,
  className,
}: {
  template: Pick<Template, 'body'>
  max?: number
  className?: string
}) {
  const sections = templateSections(template, max)
  return (
    <span className={twMerge('flex min-w-0 flex-col gap-1.5', className)}>
      {sections.length > 0
        ? sections.map((section) => (
            <span key={section} className="flex min-w-0 items-center gap-2">
              <span aria-hidden="true" className="bg-stroke-accent h-4 w-1 shrink-0 rounded-full" />
              <span
                className={typographyStyles({
                  variant: 'body',
                  className: 'text-content-secondary min-w-0 truncate',
                })}
              >
                {section}
              </span>
            </span>
          ))
        : [80, 60, 70].map((width) => (
            <span
              key={width}
              aria-hidden="true"
              className="bg-divider block h-2 rounded-full"
              style={{ width: `${width}%` }}
            />
          ))}
    </span>
  )
}
