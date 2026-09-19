import { Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { ReactNode } from 'react'
import { Button, Typography, typographyStyles } from '@/shared/ui'

export const STEP_PANEL_ID = 'clip-step-panel'

/** The workspace's top row (CLIP-37): ONE line holding the way out, the step bar and the delete,
 *  in that order — the group's own 클립 · 영상 템플릿 row is not drawn on this page, so this is
 *  the page's only chrome and the 목록 link is its way back (owner decision 2026-09-19). The page's
 *  ONE status line rides the row too, but drops to a second line only while it has something to
 *  say; at rest it is out of the flow.
 *
 *  The steps are drawn as text — 생성 › 수정 › 완성, the current one told by colour — between the two
 *  controls at every width: three pills in the row read as three more buttons, and the short
 *  names share 328px with the two 44px targets. `flex-wrap` lets a delete refusal, which asks for
 *  the full width, drop to its own line. */
export function ClipTopRow({
  status,
  steps,
  actions,
}: {
  status: ReactNode
  steps?: ReactNode
  actions?: ReactNode
}) {
  const { t } = useTranslation('clips')
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      {/* The word is underlined: `link-fg` resolves to `content-secondary`, so at rest an
          un-underlined way out is pixel-identical to ordinary copy. On a phone the glyph stands
          for it and the name stays the word. */}
      <Link
        to="/clips"
        aria-label={t('project.back')}
        className={typographyStyles({
          variant: 'label',
          className:
            'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center gap-1 sm:justify-start',
        })}
      >
        <ArrowLeft aria-hidden="true" className="size-5" />
        <span className="hidden underline sm:inline">{t('project.back')}</span>
      </Link>
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
