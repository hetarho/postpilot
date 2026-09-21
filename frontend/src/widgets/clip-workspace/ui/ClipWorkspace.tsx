import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipFailureNotice, ClipRequestRecord, type ClipProject } from '@/entities/clip-project'
import { ClipCorrectionWorkspace } from '@/features/correct-clip'
import { ClipDraftPreviewPanel } from '@/features/preview-clip-draft'
import { FinalizeClipAction } from '@/features/finalize-clip'
import { CancelClipAction } from '@/features/cancel-clip'
import { ClipProjectForm } from '@/features/edit-clip-project'
import { DeleteClipProjectButton } from '@/features/delete-clip-project'
import { discardClipDraftQueue } from '@/features/edit-clip-project'
import {
  ClipApprovalAction,
  ClipCreditSettlement,
  ClipDownloadAction,
  ClipResult,
} from '@/features/generate-clip'
import { ClipBrowserRenderStatus } from '@/features/render-clip-browser'
import { ClipRevisionRequest } from '@/features/revise-clip'
import { StageModelSelect } from '@/features/select-model'
import { ClipSourcePicker } from '@/features/upload-clip-sources'
import { ClipObservationViewer, ClipAttemptInspection } from '@/features/inspect-clip-observations'
import { ActionBar, Button, Sheet, SegmentedControl, ProgressBar, Typography } from '@/shared/ui'
import { useRunFocus, useUnsavedNotice } from '../model/lifecycle'
import { useClipWorkspace } from '../model/useClipWorkspace'
import { clipStepLabel, clipSteps } from '../model/steps'
import { ClipStepWaiting, ClipTopRow, STEP_PANEL_ID } from './ClipTopRow'
import { ClipProgressBar, ClipStatusLine, type CorrectionStatus } from './ClipStatus'

/** The clip workspace: ONE mounted block holding the three steps of a project (CLIP-36). The
 *  page routes to it and composes it; every hook the steps run on lives in `useClipWorkspace`. */
export function ClipWorkspace({
  ownerId,
  project,
  onUnsavedChange,
}: {
  ownerId: string
  project: ClipProject
  /** Told whenever the workspace starts or stops holding work the owner has not saved. */
  onUnsavedChange?: (unsaved: boolean) => void
}) {
  const { t } = useTranslation('clips')
  const workspace = useClipWorkspace(ownerId, project)
  useUnsavedNotice(workspace.unsaved, onUnsavedChange)
  // The run view takes the screen, so it takes the focus: the ref stays OUT of the handles,
  // because a ref reached through an object is a ref read during render.
  const runRoot = useRunFocus(workspace.run.focused)
  const { correction, generation, finalization, render, sources, revision, observations, run } =
    workspace
  const { pending, uploading, plan, save } = workspace
  const upload = sources.upload
  const step = workspace.step.value
  const setStep = workspace.step.set
  // A finalized project is READ in ① and ②: the same steps, the same places, values instead of
  // controls, and no original behind them (CLIP-160, CLIP-76).
  const reading = !!project.finalized
  const job = run.job
  const browserStatus = (
    <ClipBrowserRenderStatus state={render.browser.state} cancel={render.browser.cancel} />
  )
  const correctionStatus: CorrectionStatus = correction.dirty
    ? 'dirty'
    : project.editPlanRevision > project.renderedPlanRevision
      ? 'unrendered'
      : 'clean'
  // Shared reference content: ① keeps it in flow; ② mounts only the chosen tab.
  const [referenceOpen, setReferenceOpen] = useState(false)
  const [referenceTab, setReferenceTab] = useState<'observations' | 'sources' | 'requests'>(
    'observations',
  )
  const requestRecord = <ClipRequestRecord requests={project.requests} />

  const observationPanel = (
    <ClipObservationViewer
      project={project}
      onAddCut={
        reading
          ? undefined
          : observations.addCut &&
            (async (selection) => {
              await observations.addCut?.(selection, observations.soundOf(selection))
              setReferenceOpen(false)
            })
      }
      draftPlan={observations.draftPlan}
      resolvePlayback={reading ? undefined : observations.resolvePlayback}
      localSources={reading ? [] : observations.localSources}
    />
  )

  const generatePanel = (
    <ClipProjectForm
      ownerId={ownerId}
      stored={project}
      disabled={pending || reading}
      readOnly={reading}
      onUploadAllowed={sources.allow}
      refusal={reading ? undefined : <ClipFailureNotice failure={generation.failure} />}
      actions={
        reading
          ? undefined
          : (ready) => (
              <ClipApprovalAction
                ownerId={ownerId}
                project={project}
                batch={upload.readyBatch}
                observe={generation.observeRef}
                write={generation.writeRef}
                observeStatus={generation.observeStatus}
                ready={ready && generation.canQuote && !generation.busy && !render.browser.busy}
                pending={generation.starting}
                // The queue is flushed BEFORE the run starts, so an approval can never be committed
                // against settings the server has not taken (CLIP-39). A refusal stops the start; the
                // status line is already saying the save failed.
                onApprove={(quote) => {
                  void save
                    .flush()
                    .then(() => correction.flush())
                    .then(() =>
                      generation.start(upload.readyBatch, ready, quote, generation.ownership),
                    )
                    .catch(() => undefined)
                }}
              />
            )
      }
    >
      {!reading && (project.result || job?.status === 'failed' || job?.status === 'cancelled') && (
        <Typography variant="body" className="text-content-secondary mt-6">
          {t('generation.reselection')}
        </Typography>
      )}
      {/* Bind a whole source to an item before generating, where footage of one
          cut of meat carries nothing that tells it from another (CLIP-123). */}
      {!reading && (
        <ClipSourcePicker
          upload={upload}
          sound={sources.sound}
          order={sources.order}
          binding={sources.binding.items.length ? sources.binding : undefined}
          disabled={!sources.allowed || generation.busy || render.browser.busy || finalization.busy}
          processing={generation.busy}
        />
      )}
      {observationPanel}
      {requestRecord}
      {!reading && (
        <section aria-labelledby="clip-models-heading" className="mt-10 mb-8 space-y-4">
          <Typography id="clip-models-heading" variant="title">
            {t('generation.models')}
          </Typography>
          {/* The 영상 템플릿 and the two model selectors stay in ①'s PANEL rather than riding the
            dock's header the way the post editor's 말투 does: choosing a template rewrites the
            answer fields directly beneath it (CLIP-40). */}
          {/* T111's live eligibility is the observe picker's verdict for THIS workflow: the
            picker greys what it refuses with the reason, and the note below is the action's
            one readiness line. The saved choice is never cleared or swapped here; it may
            still serve photo-only posts. */}
          <StageModelSelect
            stage="observe"
            disabled={generation.busy || render.browser.busy}
            availability={generation.availability}
          />
          <StageModelSelect stage="write" disabled={generation.busy || render.browser.busy} />
          {!generation.modelsReady && (
            <Typography variant="body" role="status" className="text-content-secondary break-words">
              {generation.eligibility.kind === 'loading'
                ? t('generation.eligibility.loading')
                : generation.eligibility.kind === 'failed'
                  ? t('generation.eligibility.failed')
                  : !generation.observeRef || !generation.writeRef
                    ? t('generation.selectModels')
                    : generation.observeStatus && generation.observeStatus !== 'eligible'
                      ? t(`generation.eligibility.reason.${generation.observeStatus}`)
                      : t('generation.eligibility.unresolved')}
            </Typography>
          )}
          <Typography variant="body" className="text-content-secondary">
            {t('generation.creditPolicy')}
          </Typography>
        </section>
      )}
    </ClipProjectForm>
  )

  const referenceAction = (
    <>
      <Button
        variant="secondary"
        onClick={() => {
          setReferenceTab('observations')
          setReferenceOpen(true)
        }}
      >
        {t('reference.label')}
      </Button>
      <Sheet
        open={referenceOpen}
        labelledBy="clip-reference-title"
        onClose={() => setReferenceOpen(false)}
        header={
          <div className="space-y-3">
            <Typography variant="title" as="h2" id="clip-reference-title">
              {t('reference.label')}
            </Typography>
            <SegmentedControl
              ariaLabel={t('reference.label')}
              value={referenceTab}
              onChange={setReferenceTab}
              controls="clip-reference-panel"
              options={(['observations', 'sources', 'requests'] as const).map((value) => ({
                value,
                label: t(`reference.tabs.${value}`),
              }))}
            />
          </div>
        }
      >
        {referenceOpen && (
          <div
            role="tabpanel"
            id="clip-reference-panel"
            aria-label={t(`reference.tabs.${referenceTab}`)}
          >
            {referenceTab === 'observations' && observationPanel}
            {referenceTab === 'requests' && requestRecord}
            {referenceTab === 'sources' && (
              <>
                <section aria-labelledby="clip-required-sources" className="mt-10 space-y-3">
                  <Typography variant="title" id="clip-required-sources">
                    {t('correction.requiredSources')}
                  </Typography>
                  <ul className="space-y-2">
                    {sources.required?.map((source) => (
                      <li key={source.fingerprint}>
                        <Typography variant="body" className="break-words">
                          {source.filename}
                        </Typography>
                      </li>
                    ))}
                  </ul>
                </section>
                <ClipSourcePicker
                  upload={upload}
                  sound={sources.sound}
                  correction
                  disabled={
                    correction.dirty ||
                    correction.pending ||
                    generation.busy ||
                    render.browser.busy ||
                    finalization.busy ||
                    !correction.validation?.valid
                  }
                  processing={generation.busy}
                />
              </>
            )}
          </div>
        )}
      </Sheet>
    </>
  )

  const refinePanel = plan ? (
    <ClipCorrectionWorkspace
      project={{
        id: project.id,
        state: plan,
        captionStyles: project.allowedCaptionStyles,
        notices: project.notices,
        language: project.language,
      }}
      correction={correction}
      readOnly={reading}
      render={{
        ready: render.ready,
        pending: render.pending,
        failure: render.failure,
        progress: render.browser.state.phase !== 'idle' ? browserStatus : undefined,
        lastKind: render.lastKind,
        current: render.current,
        capability: render.capability,
        start: render.start,
      }}
      footage={{
        localSources: reading ? [] : sources.localSources,
        resolvePlayback: reading ? undefined : sources.resolvePlayback,
      }}
      disabled={pending || reading}
      slots={{
        preview: (controls) =>
          reading ? (
            <ClipResult ownerId={ownerId} project={project} />
          ) : (
            <ClipDraftPreviewPanel
              {...controls}
              projectId={project.id}
              revision={correction.revision}
              plan={correction.previewPlan}
              ratio={project.ratio}
              sources={plan.sources}
              resolvePlayback={sources.resolvePlayback}
            />
          ),
        comparison: !reading && project.result && (
          <details className="mt-4">
            <summary className="text-content-secondary cursor-pointer">
              {t('preview.renderedRevision', { revision: project.renderedPlanRevision })}
            </summary>
            <ClipResult ownerId={ownerId} project={project} />
          </details>
        ),
        revision: reading
          ? undefined
          : (actions) => (
              <ClipRevisionRequest
                ownerId={ownerId}
                project={project}
                observe={generation.observeRef}
                write={generation.writeRef}
                job={job}
                disabled={revision.disabled}
                flush={revision.flush}
                cancelAction={
                  <CancelClipAction
                    action={revision.cancellation}
                    job={job}
                    accounting={generation.accounting}
                  />
                }
                action={actions}
              />
            ),
        downloadAction:
          !reading && project.result?.downloadUrl ? (
            <ClipDownloadAction icon project={project} />
          ) : undefined,
        // 확정하기 stands only once the project has a render to confirm (CLIP-40, CLIP-152):
        // before that the dock's one action is the render, and a disabled 확정하기 beside it
        // only said so in smaller type.
        finalizeAction:
          !reading && project.result ? (
            <FinalizeClipAction
              action={finalization}
              project={project}
              disabled={
                uploading ||
                generation.busy ||
                render.browser.busy ||
                !correction.validation?.saveable
              }
              localRefusal={
                uploading || generation.busy || render.browser.busy
                  ? 'busy'
                  : !correction.validation?.saveable
                    ? 'invalid_plan'
                    : undefined
              }
            />
          ) : undefined,
        referenceAction,
      }}
    />
  ) : (
    <>
      <ClipFailureNotice failure={generation.failure} />
      <ClipStepWaiting message={t('steps.refineWaiting')} onGo={() => setStep('generate')} />
      {project.result && (
        <>
          <Typography variant="meta">
            {t('preview.renderedRevision', { revision: project.renderedPlanRevision })}
          </Typography>
          <ClipResult ownerId={ownerId} project={project} />
          {!reading && (
            <>
              <ClipDownloadAction project={project} />
              <ActionBar ariaLabel={t('correction.actions')}>
                <div className="flex flex-wrap items-center justify-end gap-2">
                  <FinalizeClipAction
                    action={finalization}
                    project={project}
                    disabled
                    localRefusal="invalid_plan"
                  />
                </div>
              </ActionBar>
            </>
          )}
        </>
      )}
      {referenceAction}
    </>
  )

  const finishPanel =
    project.finalized && project.result ? (
      <>
        <ClipResult key={project.result.createdAt} ownerId={ownerId} project={project} />
      </>
    ) : (
      <>
        <ClipStepWaiting
          message={t('finalization.waiting')}
          refine
          onGo={() => setStep('refine')}
        />
      </>
    )

  return (
    <>
      {/* First child of the flow on purpose: a sticky box can only be pinned by the box it sits
          in, and this one has to hold the page's top edge while the panel scrolls past it. */}
      {!run.focused && !project.finalized && <ClipProgressBar job={undefined} upload={upload} />}
      <ClipTopRow
        status={
          !run.focused && (
            <ClipStatusLine
              project={project}
              job={job}
              upload={upload}
              correction={correctionStatus}
              save={save}
              className="order-last w-full"
            />
          )
        }
        steps={
          !run.focused && (
            <SegmentedControl
              value={step}
              options={clipSteps()}
              onChange={(next) => {
                if (!finalization.busy) setStep(next)
              }}
              ariaLabel={t('steps.aria')}
              controls={STEP_PANEL_ID}
              variant="steps"
              className="min-w-0 flex-1"
            />
          )
        }
        actions={
          !run.focused && (
            <DeleteClipProjectButton
              ownerId={ownerId}
              project={project}
              disabled={pending}
              // A queue outlives its form, so a retry left running would keep saving an id the
              // server no longer has. Stopped before the navigation unmounts the page.
              onDeleted={() => discardClipDraftQueue(project.id)}
            />
          )
        }
      />
      <section
        ref={runRoot}
        hidden={!run.focused}
        tabIndex={-1}
        aria-label={t('cancellation.progressTitle')}
        className="my-auto min-w-0 space-y-4 py-6"
      >
        <Typography variant="title" role="status" aria-live="polite">
          {run.focused ? run.title : ''}
        </Typography>
        {run.focused && (
          <>
            <ProgressBar label={run.title} done={run.progress?.done} total={run.progress?.total} />
            <ClipSourcePicker upload={upload} processing readOnly />
            <CancelClipAction
              action={revision.cancellation}
              job={job}
              accounting={generation.accounting}
            />
          </>
        )}
      </section>
      {/* Both of these are about the ATTEMPT rather than about a step, and both are controls with
          something to press, so they stay outside the panel and outside the status line (which
          says what is true, not what to do — CLIP-38). */}
      {generation.pollFailed && (
        <div role="alert" className="mt-4">
          <Typography variant="body">{t('generation.pollingFailed')}</Typography>
          <Button variant="ghost" onClick={generation.checkAgain}>
            {t('project.retry')}
          </Button>
        </div>
      )}
      {generation.uncertain && (
        <div className="mt-4 space-y-2">
          <ClipFailureNotice failure={generation.failure} />
          <Typography variant="body">{t('credits.uncertain')}</Typography>
          <Button variant="ghost" onClick={generation.checkAgain}>
            {t('credits.checkAttempt')}
          </Button>
        </div>
      )}
      {!run.focused && !project.finalized && (
        <>
          <ClipCreditSettlement job={job} accounting={generation.accounting} />
          {job?.status === 'cancelled' && (
            <Typography variant="body" className="mt-4">
              {t('cancellation.stopped')}
            </Typography>
          )}
          {(job?.status === 'cancelled' || job?.status === 'failed') &&
            step === 'refine' &&
            job.kind === 'generate_clip' && (
              <Button variant="secondary" onClick={() => setStep('generate')}>
                {t('generation.retry')}
              </Button>
            )}
        </>
      )}
      {/* Attempt inspection precedes the editor so its preview/action docks cannot
          cover the evidence, including when enlarged text makes those docks tall. */}
      {!run.focused && !project.finalized && step !== 'finish' && (
        <ClipAttemptInspection
          key={job?.id}
          project={project}
          currentJobId={job?.id}
          resolvePlayback={upload.ensurePlayback}
          localSources={upload.entries.map((entry) => ({
            fingerprint: entry.metadata.fingerprint,
            url: entry.previewURL,
          }))}
        />
      )}
      {!run.focused && step !== 'refine' && browserStatus}
      {!run.focused && (
        <div id={STEP_PANEL_ID} role="tabpanel" aria-label={clipStepLabel(step)}>
          {step === 'generate' ? generatePanel : step === 'refine' ? refinePanel : finishPanel}
        </div>
      )}
      {/* ONE dock per step: ① and ② carry their own (the settings form's and the correction
          workspace's), so the page docks only ③'s 다운로드 (CLIP-40). */}
      {project.finalized && step === 'finish' && project.result?.downloadUrl && (
        <ActionBar className="mt-auto" ariaLabel={t('steps.finishDockAria')}>
          <div className="flex flex-wrap justify-end gap-3">
            <ClipDownloadAction project={project} />
          </div>
        </ActionBar>
      )}
    </>
  )
}
