import type { ReactNode } from 'react'
import { twMerge } from '@/shared/lib'
import { typographyStyles } from '@/shared/ui'
import { templateSections } from '../lib/template-sections'
import type { Template } from '../model/types'

/** The order a template gives a post, in one glance: its fixed headings, or — for a template that
 *  has none — what it asks the owner for, in order. Client work over the saved text alone (no
 *  request), drawn as the post's skeleton so a list of templates reads as a list of SHAPES.
 *  `TemplatePreview` is the full post; this is the template at a glance, wherever one is listed
 *  (home's kit, the templates list).
 *
 *  With `children`, the outline becomes the post the template shapes: every section draws what it
 *  holds under its heading. `fill` draws exactly `max` sections — the template's own headings
 *  first, then sections with no heading of their own (home's sheet, whose three parts always
 *  show). Without a `template` the headings are the dashed outline of a template not made yet. */
export function TemplateOutline({
  template,
  max = 4,
  className,
  children,
  fill = false,
}: {
  template?: Pick<Template, 'body'>
  max?: number
  className?: string
  /** What section `index` holds, drawn under its heading. */
  children?: (index: number) => ReactNode
  fill?: boolean
}) {
  const sections = template ? templateSections(template, max) : []
  if (children)
    return (
      <span className={twMerge('flex min-w-0 flex-col gap-3', className)}>
        {Array.from(
          { length: fill || !template ? max : Math.max(sections.length, 1) },
          (_, index) => (
            <span key={index} className="flex min-w-0 flex-col gap-2">
              {template ? (
                sections[index] !== undefined && <Heading text={sections[index]} />
              ) : (
                <GhostHeading width={[42, 56, 34][index % 3]} />
              )}
              {children(index)}
            </span>
          ),
        )}
      </span>
    )
  return (
    <span className={twMerge('flex min-w-0 flex-col gap-1.5', className)}>
      {sections.length > 0
        ? sections.map((section) => <Heading key={section} text={section} />)
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

function Heading({ text }: { text: string }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <span aria-hidden="true" className="bg-stroke-accent h-4 w-1 shrink-0 rounded-full" />
      <span
        className={typographyStyles({
          variant: 'body',
          className: 'text-content-secondary min-w-0 truncate',
        })}
      >
        {text}
      </span>
    </span>
  )
}

/** A heading the template would write, before there is a template: its bar and an empty line. */
function GhostHeading({ width }: { width: number }) {
  return (
    <span aria-hidden="true" className="flex min-w-0 items-center gap-2">
      <span className="border-stroke-accent/60 h-4 w-1 shrink-0 rounded-full border border-dashed" />
      <span
        className="border-stroke-accent/50 block h-3 rounded-full border border-dashed"
        style={{ width: `${width}%` }}
      />
    </span>
  )
}
