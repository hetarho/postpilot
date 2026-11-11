import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Typography, cardStyles, typographyStyles } from '@/shared/ui'

/** Every My work section reserves the same toolbar, including the aggregate without filters. */
export function WorkListToolbar({
  children,
  className,
}: {
  children?: ReactNode
  className?: string
}) {
  return (
    <div data-work-toolbar className={clsx('min-h-10 pointer-coarse:min-h-11', className)}>
      {children}
    </div>
  )
}

/** Mount the incoming body once, so data arriving replaces its skeleton without replaying motion. */
export function WorkListBody({
  children,
  label,
  className,
}: {
  children: ReactNode
  label?: string
  className?: string
}) {
  return (
    <section
      data-work-body
      aria-label={label}
      className={clsx('animate-step-in mt-6 flex shrink-0 flex-col gap-10', className)}
    >
      {children}
    </section>
  )
}

/** Keep the group heading and representative card geometry while either history is loading. */
export function WorkListLoading({ pending }: { pending: boolean }) {
  const { t } = useTranslation(['channel', 'common'])
  return (
    <>
      <Typography variant="meta" role="status" className="sr-only">
        {pending ? t('state.loading', { ns: 'common' }) : ''}
      </Typography>
      {pending && (
        <div aria-hidden="true" data-work-skeleton>
          <h2 className="mb-3 flex items-baseline gap-2">
            <span
              className={typographyStyles({
                variant: 'title',
                className: 'bg-surface-recessed animate-pulse rounded-md',
              })}
            >
              <span className="invisible">{t('work.inProgress')}</span>
            </span>
          </h2>
          <div className="grid gap-3 lg:grid-cols-2 lg:gap-4">
            {[0, 1].map((key) => (
              <div
                key={key}
                className={cardStyles({ padding: 'sm', className: 'flex min-h-32 gap-4 sm:p-5' })}
              >
                <span className="bg-surface-recessed size-12 shrink-0 animate-pulse rounded-lg sm:size-14" />
                <span className="flex min-w-0 flex-1 flex-col gap-3 py-1">
                  <span className="bg-surface-recessed h-5 w-3/4 animate-pulse rounded-md" />
                  <span className="bg-surface-recessed h-4 w-1/2 animate-pulse rounded-md" />
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </>
  )
}
