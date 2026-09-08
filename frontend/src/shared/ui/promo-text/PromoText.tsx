import type { ComponentPropsWithoutRef, ElementType } from 'react'
import { twMerge } from 'tailwind-merge'
import { Typography } from '../typography/Typography'
import type { TypographyVariant } from '../typography/typographyStyles'

type PromoTextProps<E extends ElementType> = {
  variant: TypographyVariant
  as?: E
  className?: string
} & Omit<ComponentPropsWithoutRef<E>, 'as' | 'className'>

/** A type role painted as gradient text, for the figure a promotional surface leads with
 *  (THEME-37): the price a plan card compares by, the count its grant buys.
 *
 *  It is `Typography` with the promotional gradient clipped to the glyphs, so the size, weight
 *  and tracking stay the role's and only the ink changes. Both stops are contrast-audited
 *  foundations (the accent and the info hue), which is what keeps gradient text readable where a
 *  hue chosen for glow would not be. The name carries the exemption: `PromoText` in an ordinary
 *  form is the exception spreading. */
export function PromoText<E extends ElementType = 'p'>({ className, ...props }: PromoTextProps<E>) {
  return (
    <Typography
      {...(props as PromoTextProps<ElementType>)}
      className={twMerge('bg-promo-text bg-clip-text text-transparent', className)}
    />
  )
}
