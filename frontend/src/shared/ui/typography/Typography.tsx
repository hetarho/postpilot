import type { ComponentPropsWithoutRef, ElementType } from 'react'
import { typographyStyles, type TypographyVariant } from './typographyStyles'

/** Semantics follow the variant until the caller says otherwise: a page's one `display` is its
 *  `h1`, a section `title` is an `h2`. `as` exists because the visual role and the outline level
 *  are independent — a third-level heading can still look like `title`, and a post row's display
 *  text is not a heading at all. */
const DEFAULT_ELEMENT: Record<TypographyVariant, ElementType> = {
  display: 'h1',
  title: 'h2',
  body: 'p',
  label: 'span',
  meta: 'span',
  eyebrow: 'span',
}

type TypographyProps<E extends ElementType> = {
  variant: TypographyVariant
  as?: E
  mono?: boolean
  className?: string
} & Omit<ComponentPropsWithoutRef<E>, 'as' | 'className'>

/** The §3 type roles as the one text component (design-language §3). Every piece of slice text —
 *  heading, prose, label, metadata — renders through this (or through `typographyStyles` when an
 *  element must keep its own component), never through ad-hoc `text-*`/`font-*` composition. */
export function Typography<E extends ElementType = 'p'>({
  variant,
  as,
  mono,
  className,
  ...props
}: TypographyProps<E>) {
  const Element = (as ?? DEFAULT_ELEMENT[variant]) as ElementType
  return <Element className={typographyStyles({ variant, mono, className })} {...props} />
}
