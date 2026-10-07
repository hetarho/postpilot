import { ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { ReactNode } from 'react'
import { Button, Typography, typographyStyles } from '@/shared/ui'

export const STEP_PANEL_ID = 'clip-step-panel'

/** Contextual return, persisted lifecycle steps and delete share the clip's own chrome.
 * The title identifies the record while its active task or running stage supplies the heading. */
export function ClipTopRow({
  status,
  steps,
  actions,
  returnTo,
  contextTitle,
}: {
  status: ReactNode
  steps?: ReactNode
  actions?: ReactNode
  returnTo: { href: string; label: string; onReturn?: () => void | Promise<void> }
  contextTitle?: string
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      {contextTitle && (
        <Typography variant="label" as="p" className="text-content-secondary w-full break-words">
          {contextTitle}
        </Typography>
      )}
      <a
        href={returnTo.href}
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
        className={typographyStyles({
          variant: 'label',
          className:
            'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center gap-1 sm:justify-start',
        })}
      >
        <ArrowLeft aria-hidden="true" className="size-5" />
        <span className="underline">{returnTo.label}</span>
      </a>
      {steps}
      {actions && <div className="ml-auto flex shrink-0 items-center gap-2">{actions}</div>}
      {status}
    </div>
  )
}

/** A step the project has not reached yet. Never a disabled tab: the point of showing all three
 *  from the first screen is that the shape of the flow is visible, so an empty step says what it
 *  is waiting for and offers the way to the step that produces it (THEME-39). */
export function ClipStepWaiting({
  message,
  onGo,
  refine = false,
}: {
  message: string
  onGo: () => void
  refine?: boolean
}) {
  const { t } = useTranslation('clips')
  return (
    <div className="mt-8">
      <Typography variant="body" as="p" className="text-content-secondary">
        {message}
      </Typography>
      <Button variant="ghost" onClick={onGo} className="mt-2 -ml-3">
        {t(refine ? 'finalization.goRefine' : 'steps.goGenerate')}
      </Button>
    </div>
  )
}

/** `/clips/new` — a project that does not exist yet. It has no lifecycle, so it shows no step
 *  bar and no delete: just the settings and the one committing action that mints it, which stays
 *  explicit because the ratio it carries can never be changed again (CLIP-9, CLIP-39). */
