import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'

export interface GuideStep<T extends string> {
  value: T
  /** The step's short name under its segment. A count of like steps — the learning chat's
   *  answers, a review's candidates — names none, and the row is then the segments alone. */
  label?: string
}

/** Guide: the ONE way the app shows where the owner is in a sequence — the writing guide, the
 *  template wizards, first-use setup, the learning chat's answers and the candidate review. One
 *  rounded segment per step, the steps behind and the one on screen filled, the ones ahead a
 *  recess — with each step's short name under its segment, the current one in ink.
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
  const labelled = steps.some((step) => step.label)
  // Unlabelled segments carry no text row, so the row is only as tall as a segment.
  const cell = labelled
    ? 'flex min-h-11 w-full flex-col items-start justify-center gap-1.5'
    : 'flex w-full flex-col items-start justify-center'
  return (
    <nav aria-label={ariaLabel} className={twMerge('w-full', className)}>
      <ol className={clsx('flex w-full', labelled ? 'gap-1.5' : 'gap-1')}>
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
              {labelled ? (
                <span
                  className={clsx(
                    'text-meta block truncate leading-tight',
                    here ? 'text-content-primary font-bold' : 'text-content-tertiary font-medium',
                  )}
                >
                  {step.label}
                </span>
              ) : (
                here && (
                  <span className="sr-only">
                    {position + 1} / {steps.length}
                  </span>
                )
              )}
            </>
          )
          return (
            <li key={step.value} className="min-w-0 flex-1">
              {onSelect ? (
                <button
                  type="button"
                  aria-current={here ? 'step' : undefined}
                  onClick={() => onSelect(step.value)}
                  className={clsx(cell, 'rounded-sm text-left')}
                >
                  {body}
                </button>
              ) : (
                <span aria-current={here ? 'step' : undefined} className={cell}>
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
