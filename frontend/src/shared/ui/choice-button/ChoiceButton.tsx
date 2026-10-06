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
  description: ReactNode
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
      aria-describedby={`${id}-description`}
      className={buttonStyles({
        variant: 'secondary',
        className: clsx(
          'group w-full min-w-0 justify-start rounded-lg p-5 text-left sm:p-6',
          className,
        ),
      })}
      {...props}
    >
      <span className={clsx('flex w-full min-w-0 items-center gap-4', pending && 'opacity-0')}>
        {icon && (
          <span aria-hidden="true" className="text-content-secondary shrink-0">
            {icon}
          </span>
        )}
        <span className="min-w-0 flex-1">
          <Typography
            as="span"
            variant="fieldTitle"
            id={`${id}-title`}
            className="block break-words"
          >
            {title}
          </Typography>
          <Typography
            as="span"
            variant="body"
            id={`${id}-description`}
            className="text-content-secondary mt-2 block break-words"
          >
            {description}
          </Typography>
        </span>
        <span aria-hidden="true" className="text-content-secondary shrink-0">
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
