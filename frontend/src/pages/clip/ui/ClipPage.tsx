import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useBlocker, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipProject, requiredClipSources, type ClipProject } from '@/entities/clip-project'
import { useClipCorrection, ClipCorrectionWorkspace } from '@/features/correct-clip'
import { useSession } from '@/entities/session'
import { ClipProjectForm, useClipDraftSave } from '@/features/edit-clip-project'
import { DeleteClipProjectButton } from '@/features/delete-clip-project'
import { discardClipDraftQueue } from '@/features/edit-clip-project'
import {
  ClipApprovalAction,
  ClipCreditSettlement,
  ClipDownloadAction,
  ClipGenerationFailure,
  ClipResult,
  useGenerateClip,
} from '@/features/generate-clip'
import { StageModelSelect } from '@/features/select-model'
import { ClipSourcePicker, useClipSourceUpload } from '@/features/upload-clip-sources'
import { ClipObservationViewer } from '@/features/inspect-clip-observations'
import { appFailureFromConnect } from '@/shared/api'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Dialog,
  SegmentedControl,
  Typography,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { clipStepLabel, clipSteps, stepForProject, type ClipStep } from '../model/steps'
import { ClipProgressBar, ClipStatusLine, type CorrectionStatus } from './ClipStatus'

const STEP_PANEL_ID = 'clip-step-panel'

/** The workspace's top row (CLIP-37): the way out, the page's ONE status line, and the delete.
 *  `flex-wrap` so a delete refusal, which asks for the full width, drops to its own line rather
 *  than crushing the way out beside it — the Korean refusal copy is longer than a 360px row can
 *  hold beside anything. */
function ClipTopRow({ status, actions }: { status: ReactNode; actions?: ReactNode }) {
  const { t } = useTranslation('clips')
  return (
    <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
      {/* Underlined: `link-fg` resolves to `content-secondary`, so at rest an un-underlined way
          out is pixel-identical to ordinary copy and only a `hover:` colour no touchscreen ever
          matches would mark it. */}
      <Link
        to="/clips"
        className={typographyStyles({
          variant: 'label',
          className:
            'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center underline',
        })}
      >
        {t('project.back')}
      </Link>
      <div className="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-2">
        {status}
        {actions}
      </div>
    </div>
  )
}

/** A step the project has not reached yet. Never a disabled tab: the point of showing all three
 *  from the first screen is that the shape of the flow is visible, so an empty step says what it
 *  is waiting for and offers the way to the step that produces it (THEME-39). */
function ClipStepWaiting({ message, onGo }: { message: string; onGo: () => void }) {
  const { t } = useTranslation('clips')
  return (
    <div className="mt-8">
      <Typography variant="body" as="p" className="text-content-secondary">
        {message}
      </Typography>
      <Button variant="ghost" onClick={onGo} className="mt-2 -ml-3">
        {t('steps.goGenerate')}
      </Button>
    </div>
  )
}

/** `/clips/new` — a project that does not exist yet. It has no lifecycle, so it shows no step
 *  bar and no delete: just the settings and the one committing action that mints it, which stays
 *  explicit because the ratio it carries can never be changed again (CLIP-9, CLIP-39). */
function NewClip({ ownerId }: { ownerId: string }) {
  return (
    <>
      {/* The status region is mounted before there is a project to have a status: a live region
          inserted already holding its message announces nothing. */}
      <ClipTopRow
        status={
          <ClipStatusLine
            project={undefined}
            job={undefined}
            upload={{ phase: 'idle' }}
            correction="clean"
            save={{ failing: false, label: '' }}
          />
        }
      />
      <ClipProjectForm ownerId={ownerId} />
    </>
  )
}

function ExistingClip({ ownerId, project }: { ownerId: string; project: ClipProject }) {
  const { t } = useTranslation('clips')
  // The bar FOLLOWS the project's own state: a manual tab choice stands until the project itself
  // moves on — a finished generation, a saved correction, a completed render — and then the bar
  // goes to the step that now describes it. Without that, a generation that finished while the
  // owner was on ① would leave the only copy of the video behind a tab nothing pointed at.
  // Adjusted DURING render rather than from an effect, so no frame paints the stale step.
  const derived = stepForProject(project)
  const [step, setStep] = useState<ClipStep>(derived)
  const [followed, setFollowed] = useState(derived)
  if (followed !== derived) {
    setFollowed(derived)
    setStep(derived)
  }
  const [uploadAllowed, setUploadAllowed] = useState(false)
  // The settings' autosave is watched HERE, not inside ①: the queue outlives the panel, so a save
  // in flight or failing has to stay on the line while the owner is on ② or ③ (CLIP-39).
  const save = useClipDraftSave(project.id)

  // Every hook the workspace runs on lives HERE, above the panels: the steps are panels of ONE
  // mounted page (CLIP-36), so changing step cannot remount the upload session, restart the job
  // poll or throw away a correction draft the owner has not saved yet.
  const correction = useClipCorrection(ownerId, project)
  const plan = project.editing
  // The required set follows the STEP because ① needs every source for a fresh AI run while ②
  // needs only the cuts' sources (CLIP-23). Changing it is what releases the picked originals —
  // the same rule the retired 보정 진입 toggle had, not something the step bar adds.
  const required =
    step === 'refine' && plan ? requiredClipSources(correction.draft, plan.sources) : undefined
  const upload = useClipSourceUpload(project.id, required)
  const generation = useGenerateClip(ownerId, project, upload.attempt?.jobId)
  const ownership = {
    begin: upload.beginAttempt,
    owned: upload.markOwned,
    rejected: upload.rejectAttempt,
  }
  const job = generation.job
  useEffect(() => {
    if (job && (job.status === 'done' || job.status === 'failed'))
      upload.finishAttempt(job.id, job.status)
  }, [job, upload])

  // The unsaved-correction guard belongs to the PAGE, not to ②: the draft outlives a step change
  // (it is held here), so a guard mounted with the panel would stop warning the moment the owner
  // looked at ③ and let a real navigation throw the edits away silently.
  const leaving = useRef(false)
  const guard = () => correction.dirty && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })

  const uploading = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  const pending = generation.busy || uploading
  const correctionStatus: CorrectionStatus = correction.dirty
    ? 'dirty'
    : project.editPlanRevision > project.renderedPlanRevision
      ? 'unrendered'
      : 'clean'

  // Inside each step's content, above its dock, so inspecting a long observation
  // cannot scroll the step's committing action out of its sticky container.
  const observationPanel = (
    <ClipObservationViewer
      project={project}
      localSources={upload.entries.map((entry) => ({
        fingerprint: entry.metadata.fingerprint,
        url: entry.previewURL,
      }))}
    />
  )

  const generatePanel = (
    <ClipProjectForm
      ownerId={ownerId}
      stored={project}
      disabled={pending}
      onUploadAllowed={setUploadAllowed}
      refusal={<ClipGenerationFailure failure={generation.failure} />}
      actions={(ready) => (
        <ClipApprovalAction
          ownerId={ownerId}
          project={project}
          batch={upload.readyBatch}
          observe={generation.observeRef}
          write={generation.writeRef}
          observeStatus={generation.observeStatus}
          ready={ready && generation.modelsReady && !generation.busy}
          pending={generation.starting}
          // The queue is flushed BEFORE the run starts, so an approval can never be committed
          // against settings the server has not taken (CLIP-39). A refusal stops the start; the
          // status line is already saying the save failed.
          onApprove={(quote) => {
            void save
              .flush()
              .then(() => generation.start(upload.readyBatch, ready, quote, ownership))
              .catch(() => undefined)
          }}
        />
      )}
    >
      {(project.result || job?.status === 'failed') && (
        <Typography variant="body" className="text-content-secondary mt-6">
          {t('generation.reselection')}
        </Typography>
      )}
      <ClipSourcePicker
        upload={upload}
        disabled={!uploadAllowed || generation.busy}
        processing={generation.busy}
      />
      {observationPanel}
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
          disabled={generation.busy}
          availability={generation.availability}
        />
        <StageModelSelect stage="write" disabled={generation.busy} />
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
    </ClipProjectForm>
  )

  const refinePanel = plan ? (
    <ClipCorrectionWorkspace
      correction={correction}
      state={plan}
      answers={project.answers}
      disabled={pending}
      renderReady={!!upload.readyBatch && correction.revision === project.editPlanRevision}
      renderPending={generation.starting}
      renderFailure={generation.failure}
      onRender={() => {
        if (!correction.dirty && !correction.pending)
          void generation.render(upload.readyBatch, correction.revision, ownership)
      }}
      localSources={upload.entries.map((entry) => ({
        fingerprint: entry.metadata.fingerprint,
        url: entry.previewURL,
      }))}
      sourcePicker={
        <>
          {observationPanel}
          <section aria-labelledby="clip-required-sources" className="mt-10 space-y-3">
            <Typography variant="title" id="clip-required-sources">
              {t('correction.requiredSources')}
            </Typography>
            <ul className="space-y-2">
              {required?.map((source) => (
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
            correction
            disabled={
              correction.dirty ||
              correction.pending ||
              generation.busy ||
              !correction.validation?.valid
            }
            processing={generation.busy}
          />
        </>
      }
    />
  ) : (
    <>
      <ClipStepWaiting message={t('steps.refineWaiting')} onGo={() => setStep('generate')} />
      {observationPanel}
    </>
  )

  const finishPanel = project.result ? (
    <>
      <ClipResult key={project.result.createdAt} ownerId={ownerId} project={project} />
      {observationPanel}
    </>
  ) : (
    <>
      <ClipStepWaiting message={t('steps.finishWaiting')} onGo={() => setStep('generate')} />
      {observationPanel}
    </>
  )

  return (
    <>
      {/* First child of the flow on purpose: a sticky box can only be pinned by the box it sits
          in, and this one has to hold the page's top edge while the panel scrolls past it. */}
      <ClipProgressBar job={job} upload={upload} />
      <ClipTopRow
        status={
          <ClipStatusLine
            project={project}
            job={job}
            upload={upload}
            correction={correctionStatus}
            save={save}
          />
        }
        actions={
          <DeleteClipProjectButton
            ownerId={ownerId}
            project={project}
            disabled={pending}
            // A queue outlives its form, so a retry left running would keep saving an id the
            // server no longer has. Stopped before the navigation unmounts the page.
            onDeleted={() => discardClipDraftQueue(project.id)}
          />
        }
      />
      <SegmentedControl
        value={step}
        options={clipSteps()}
        onChange={setStep}
        ariaLabel={t('steps.aria')}
        controls={STEP_PANEL_ID}
        className="mt-4"
      />
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
          <Typography variant="body">{t('credits.uncertain')}</Typography>
          <Button variant="ghost" onClick={generation.checkAgain}>
            {t('credits.checkAttempt')}
          </Button>
        </div>
      )}
      <ClipCreditSettlement job={job} accounting={generation.accounting} />
      <div id={STEP_PANEL_ID} role="tabpanel" aria-label={clipStepLabel(step)}>
        {step === 'generate' ? generatePanel : step === 'refine' ? refinePanel : finishPanel}
      </div>
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('correction.leaveTitle')}
        confirmLabel={t('project.leave')}
        onClose={() => blocker.reset?.()}
        onConfirm={() => {
          leaving.current = true
          blocker.proceed?.()
        }}
      >
        {t('correction.leaveBody')}
      </Dialog>
      {/* ONE dock per step: ① and ② carry their own (the settings form's and the correction
          workspace's), so the page docks only ③'s 다운로드 (CLIP-40). */}
      {step === 'finish' && project.result?.downloadUrl && (
        <ActionBar className="mt-auto" ariaLabel={t('steps.finishDockAria')}>
          <div className="flex flex-wrap justify-end gap-3">
            <ClipDownloadAction project={project} />
          </div>
        </ActionBar>
      )}
    </>
  )
}

export function ClipPage() {
  const { t } = useTranslation('clips')
  const { user } = useSession()
  const { clipId } = useParams({ strict: false })
  const query = useClipProject(user?.id ?? '', clipId)
  return (
    // `flex-1 flex-col` here plus `mt-auto` on a dock is what puts the bar at the BOTTOM of a
    // short panel: `sticky` can only pull an element up toward the scrollport edge, never push
    // one down.
    <main className={pageStyles({ width: 'prose', className: 'flex flex-1 flex-col pb-8' })}>
      {!clipId ? (
        <NewClip ownerId={user?.id ?? ''} />
      ) : query.data ? (
        <ExistingClip key={`${user?.id}-${clipId}`} ownerId={user?.id ?? ''} project={query.data} />
      ) : query.isPending ? (
        <>
          <ClipTopRow status={null} />
          <Typography variant="body" role="status" className="mt-6">
            {t('project.loading')}
          </Typography>
        </>
      ) : (
        <>
          <ClipTopRow status={null} />
          <div role="alert" className="mt-6">
            <AppFailureMessage failure={appFailureFromConnect(query.error)} />
            <Button variant="ghost" onClick={() => void query.refetch()}>
              {t('project.retry')}
            </Button>
          </div>
        </>
      )}
    </main>
  )
}
