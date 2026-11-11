import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'

export interface GuideStep<T extends string> {
  value: T
  label: string
}

/** Guide: the slim progress a guided flow wears at its top — one rounded segment per step, the
 *  steps behind and the one on screen filled, the ones ahead a recess — with each step's short
 *  name under its segment, the current one in ink.
 *
 *  Every segment is a button when the flow lets the owner jump (`onSelect`): a busy owner who
 *  wants to change the photos from the summary taps 사진 rather than pressing 이전 twice. The
 *  current step carries `aria-current="step"`, so the position is announced, not only painted. */
export function GuideProgress<T extends string>({
  steps,
  current,
  onSelect,
  ariaLabel,
  className,
}: {
  steps: readonly GuideStep<T>[]
  current: T
  onSelect?: (value: T) => void
  ariaLabel: string
  className?: string
}) {
  const index = Math.max(
    0,
    steps.findIndex((step) => step.value === current),
  )
  return (
    <nav aria-label={ariaLabel} className={twMerge('w-full', className)}>
      <ol className="flex w-full gap-1.5">
        {steps.map((step, position) => {
          const reached = position <= index
          const here = position === index
          const body = (
            <>
              <span
                aria-hidden="true"
                className={clsx(
                  'duration-slow ease-standard block h-1.5 w-full rounded-full transition-colors',
                  reached ? 'bg-progress-step-done' : 'bg-progress-step-todo',
                )}
              />
              <span
                className={clsx(
                  'text-meta block truncate leading-tight',
                  here ? 'text-content-primary font-bold' : 'text-content-tertiary font-medium',
                )}
              >
                {step.label}
              </span>
            </>
          )
          return (
            <li key={step.value} className="min-w-0 flex-1">
              {onSelect ? (
                <button
                  type="button"
                  aria-current={here ? 'step' : undefined}
                  onClick={() => onSelect(step.value)}
                  className="flex min-h-11 w-full flex-col items-start justify-center gap-1.5 rounded-sm text-left"
                >
                  {body}
                </button>
              ) : (
                <span
                  aria-current={here ? 'step' : undefined}
                  className="flex min-h-11 w-full flex-col items-start justify-center gap-1.5"
                >
                  {body}
                </span>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
