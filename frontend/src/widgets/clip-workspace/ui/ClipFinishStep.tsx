import { useEffect, useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipBrowserRenderCapability } from '@/entities/clip-preview'
import {
  ClipFailureNotice,
  clipConfirmed,
  type ClipProject,
  type ClipRenderKind,
} from '@/entities/clip-project'
import type { MyPlan } from '@/entities/plan'
import { ClipPublishedUrlField, type useFinalizeClip } from '@/features/finalize-clip'
import { ClipDownloadAction, ClipResult } from '@/features/generate-clip'
import {
  ClipBrowserLocalResult,
  ClipBrowserRenderProgress,
  ClipRenderAction,
  type BrowserRenderState,
} from '@/features/render-clip-browser'
import { StepBar, StepBarActions, StepBarBack, Typography, cardStyles } from '@/shared/ui'
import type { AppFailure } from '@/shared/api'
import { ClipStepWaiting } from './ClipTopRow'

/** What ③ needs of the workspace's render handle. */
export interface ClipFinishRender {
  browser: {
    state: BrowserRenderState
    cancel: () => void
    retry?: () => void
  }
  /** Undefined while the device is still being checked. */
  capability?: ClipBrowserRenderCapability
  ready: boolean
  pending: boolean
  lastKind?: ClipRenderKind
  serverWindow?: MyPlan['serverExportWindow']
  serverPlan?: MyPlan['plan']
  serverEntitled?: boolean
  /** A render start the server refused before it became a job. */
  failure?: AppFailure
  /** A server render that stopped: its failure, or a cancellation (CLIP-79). */
  server?: { outcome: 'failed' | 'cancelled'; failure?: AppFailure }
  start: (kind: ClipRenderKind) => void
}

/** ③ 영상 완성 (CLIP-230): a short heading, the delivered render — its progress while it runs, then
 *  the file it hands over with its download under it — then the checklist, the 업로드 문구 and
 *  더 해 보기, and the 게시 URL with `완료로 표시` at its foot. ③ docks a step bar only while a
 *  render runs or failed (CLIP-40). */
export function ClipFinishStep({
  ownerId,
  project,
  render,
  finalization,
  disabled,
  startRender,
  onStartRenderHandled,
  onGoRefine,
  slots = {},
}: {
  ownerId: string
  project: ClipProject
  render: ClipFinishRender
  finalization: ReturnType<typeof useFinalizeClip>
  /** Holds ③'s actions while other work holds the project. */
  disabled: boolean
  /** One-shot: 확정하기 just succeeded, so the browser render starts at once (CLIP-153). */
  startRender: boolean
  onStartRenderHandled: () => void
  onGoRefine: () => void
  slots?: {
    /** The pre-submission checklist while 체험단 글 is on (CLIP-231). */
    checklist?: ReactNode
    /** The 업로드 문구 for the clip's channel (CLIP-211). */
    uploadText?: ReactNode
    /** The one 더 해 보기 card (CLIP-233). */
    more?: ReactNode
    /** The clip channel the finalized project may still move to (CLIP-209). */
    channel?: ReactNode
  }
}) {
  const { t } = useTranslation('clips')
  const headingId = useId()
  const finalized = !!project.finalized
  const confirmed = clipConfirmed(project)
  // A durable render of the plan as it stands — the confirmed one while it is confirmed.
  const matching = !!project.result?.id && project.renderedPlanRevision === project.editPlanRevision
  const phase = render.browser.state.phase
  const running = phase === 'running' || phase === 'cancelling'
  const browserFailed = phase === 'failed' || phase === 'upload_pending'
  const failed = browserFailed || render.server?.outcome === 'failed'
  const again = failed || phase === 'cancelled' || render.server?.outcome === 'cancelled'
  const { capability, ready, start } = render

  // 확정하기 starts the browser render of the confirmed plan at once on a supported device,
  // unless a durable render already matches it (CLIP-153). The request is consumed once: a
  // device that cannot render, or a render that already matches, leaves ③'s offer instead.
  useEffect(() => {
    if (!startRender) return
    if (finalized || !confirmed || matching || capability?.available === false) {
      onStartRenderHandled()
      return
    }
    // Still checking the device, or still reaching the originals: decide once they answer.
    if (!capability || !ready) return
    onStartRenderHandled()
    start('browser')
  }, [startRender, finalized, confirmed, matching, capability, ready, start, onStartRenderHandled])

  const renderAction = (
    <ClipRenderAction
      lastKind={render.lastKind}
      again={again}
      browserAvailable={capability?.available ?? false}
      browserRefusal={capability && !capability.available ? capability.reason : undefined}
      serverWindow={render.serverWindow}
      serverPlan={render.serverPlan}
      serverEntitled={render.serverEntitled}
      pending={render.pending}
      disabled={disabled || !ready}
      variant="cta"
      onRender={start}
    />
  )
  const result = project.result && (
    <div className="space-y-2">
      <Typography variant="meta" as="p">
        {t(matching ? 'finish.revision' : 'finish.earlierRevision', {
          revision: project.renderedPlanRevision,
        })}
      </Typography>
      <ClipResult key={project.result.createdAt} ownerId={ownerId} project={project} />
      {/* The download stands directly under the video it downloads, naming its revision
          (CLIP-149). */}
      <ClipDownloadAction icon project={project} />
    </div>
  )

  return (
    <>
      <Typography variant="stepTitle" as="h2" id={headingId} className="mt-6">
        {t(finalized || confirmed ? 'finish.heading' : 'finish.waitingHeading')}
      </Typography>
      <section
        aria-labelledby={headingId}
        className={cardStyles({ className: 'mt-6 min-w-0 space-y-4' })}
      >
        {!finalized && !confirmed && (
          <ClipStepWaiting
            message={t('finalization.waiting')}
            goLabel={t('finalization.goRefine')}
            onGo={onGoRefine}
          />
        )}
        {!finalized && confirmed && (
          <ClipBrowserLocalResult state={render.browser.state} retry={render.browser.retry} />
        )}
        {/* A finished or cancelled browser render says so where its result stands; while it runs
            or after it failed, the step bar carries it instead. */}
        {!finalized && !running && !failed && (
          <ClipBrowserRenderProgress state={render.browser.state} cancel={render.browser.cancel} />
        )}
        {result}
        {!finalized && confirmed && !render.server && (
          <ClipFailureNotice failure={render.failure} />
        )}
        {!finalized && confirmed && !matching && !running && !failed && (
          <div className="mt-6 flex flex-wrap items-center gap-2">{renderAction}</div>
        )}
      </section>
      {slots.checklist}
      {slots.uploadText}
      {slots.more}
      {slots.channel}
      <ClipPublishedUrlField action={finalization} project={project} disabled={disabled} />
      {!finalized && (running || failed) && (
        <StepBar ariaLabel={t('finish.barAria')}>
          {render.server?.outcome === 'failed' && (
            <ClipFailureNotice failure={render.server.failure} />
          )}
          <ClipBrowserRenderProgress state={render.browser.state} cancel={render.browser.cancel} />
          {!running && (
            <StepBarActions back={<StepBarBack label={t('finish.back')} onClick={onGoRefine} />}>
              {renderAction}
            </StepBarActions>
          )}
        </StepBar>
      )}
    </>
  )
}
