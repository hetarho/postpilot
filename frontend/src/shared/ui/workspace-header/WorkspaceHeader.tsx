import type { ReactNode } from 'react'
import { ArrowLeft } from 'lucide-react'
import { buttonStyles } from '../button/buttonStyles'
import { Typography } from '../typography/Typography'

/** Presentation only: the owner resolves the destination and flushes its work before leaving. */
export function WorkspaceHeader({
  returnTo,
  contextTitle,
  steps,
  actions,
  status,
}: {
  returnTo: { href: string; label: string; onReturn?: () => void | Promise<void> }
  contextTitle?: string
  steps?: ReactNode
  actions?: ReactNode
  status?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-2">
      {contextTitle && (
        <Typography variant="label" as="p" className="text-content-secondary w-full break-words">
          {contextTitle}
        </Typography>
      )}
      <a
        href={returnTo.href}
        aria-label={returnTo.label}
        className={buttonStyles({ variant: 'secondary', size: 'icon', className: 'shadow-sm' })}
        onClick={(event) => {
          if (
            !returnTo.onReturn ||
            event.defaultPrevented ||
            event.button !== 0 ||
            event.metaKey ||
            event.ctrlKey ||
            event.shiftKey ||
            event.altKey
          )
            return
          event.preventDefault()
          void returnTo.onReturn()
        }}
      >
        <ArrowLeft aria-hidden="true" className="size-5" />
        <span className="sr-only">{returnTo.label}</span>
      </a>
      {steps && (
        <div className="order-last w-full min-w-0 overflow-x-auto overscroll-x-contain sm:order-none sm:w-auto sm:flex-1">
          {steps}
        </div>
      )}
      {actions && (
        <div className="ml-auto flex max-w-full min-w-0 flex-wrap items-center justify-end gap-2">
          {actions}
        </div>
      )}
      {status}
    </div>
  )
}
