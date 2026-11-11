import { forwardRef, useId, type ReactNode } from 'react'
import { Check, ChevronRight } from 'lucide-react'
import { clsx } from 'clsx'
import { buttonStyles } from '../button/buttonStyles'
import type { ButtonProps } from '../button/Button'
import { Spinner } from '../spinner/Spinner'
import { Typography } from '../typography/Typography'

export interface ChoiceButtonProps extends Omit<
  ButtonProps,
  'children' | 'title' | 'variant' | 'size'
> {
  title: ReactNode
  description?: ReactNode
  icon?: ReactNode
  trailing?: ReactNode
  selected?: boolean
}

/** An explained peer choice. Its description belongs to the choice, not a paragraph below it. */
export const ChoiceButton = forwardRef<HTMLButtonElement, ChoiceButtonProps>(function ChoiceButton(
  {
    title,
    description,
    icon,
    trailing,
    selected,
    pending = false,
    disabled,
    className,
    type = 'button',
    ...props
  },
  ref,
) {
  const id = useId()
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || pending}
      aria-busy={pending || undefined}
      aria-pressed={selected}
      aria-labelledby={`${id}-title`}
      aria-describedby={description ? `${id}-description` : undefined}
      className={buttonStyles({
        variant: 'secondary',
        className: clsx(
          'group @container w-full min-w-0 justify-start rounded-lg p-0 text-left',
          // The chosen card is ringed in the accent, the way ①'s template cards always were, so a
          // choice reads as made at a glance rather than by a small tick alone.
          selected && 'ring-stroke-accent ring-2',
          className,
        ),
      })}
      {...props}
    >
      <span
        className={clsx(
          'flex w-full min-w-0 items-center gap-4',
          description ? 'p-4 @md:p-6' : 'gap-2 p-3 @md:p-4',
          pending && 'opacity-0',
        )}
      >
        {icon && (
          <span aria-hidden="true" className="text-content-secondary hidden shrink-0 @md:block">
            {icon}
          </span>
        )}
        <span className="min-w-0 flex-1">
          {/* Narrow, the tick sits in the card's corner over space the title always leaves for
              it. Set above the title as a row of its own, it made the chosen card a row taller
              and the one let go a row shorter, so every choice jolted the cards under it. */}
          {selected && description && (
            <span
              aria-hidden="true"
              className="text-link-fg-current absolute top-4 right-4 @md:hidden"
            >
              <Check className="size-5" />
            </span>
          )}
          <Typography
            as="span"
            variant="fieldTitle"
            id={`${id}-title`}
            className={clsx('block break-words', description && 'pr-7 @md:pr-0')}
          >
            {title}
          </Typography>
          {description && (
            <Typography
              as="span"
              variant="body"
              id={`${id}-description`}
              className="text-content-secondary mt-2 block break-words"
            >
              {description}
            </Typography>
          )}
        </span>
        <span
          aria-hidden="true"
          className={clsx('text-content-secondary shrink-0', description && 'hidden @md:block')}
        >
          {trailing ??
            (selected ? <Check className="size-5" /> : <ChevronRight className="size-5" />)}
        </span>
      </span>
      {pending && (
        <span className="absolute inset-0 flex items-center justify-center">
          <Spinner />
        </span>
      )}
    </button>
  )
})
