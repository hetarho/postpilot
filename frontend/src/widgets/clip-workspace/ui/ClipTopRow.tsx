import { useTranslation } from 'react-i18next'
import type { ReactNode } from 'react'
import { Button, Typography, WorkspaceHeader } from '@/shared/ui'

export const STEP_PANEL_ID = 'clip-step-panel'

/** Contextual return, persisted lifecycle steps and delete share the clip's own chrome.
 * The title identifies the record while its active task or running stage supplies the heading. */
export function ClipTopRow(props: {
  status: ReactNode
  steps?: ReactNode
  actions?: ReactNode
  returnTo: { href: string; label: string; onReturn?: () => void | Promise<void> }
  contextTitle?: string
}) {
  return <WorkspaceHeader {...props} />
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
