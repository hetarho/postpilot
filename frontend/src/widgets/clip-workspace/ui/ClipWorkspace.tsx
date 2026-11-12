import { ClipBrowserPreparationStatus } from '@/features/prepare-clip-browser'
import { useId, useRef, useState, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { isTerminal } from '@/entities/generation-job'
import { useTranslation } from 'react-i18next'
import {
  ClipFailureNotice,
  ClipRequestRecord,
  clipChannel,
  clipConfirmed,
  clipDisplayTitle,
  clipReturnDestination,
  markClipHistoryReturn,
  type ClipChannel,
  type ClipProject,
} from '@/entities/clip-project'
import { ClipCorrectionWorkspace } from '@/features/correct-clip'
import { ClipStorylineSpace, discardClipStorylineQueue } from '@/features/edit-clip-storyline'
import { ClipStorylineRequest } from '@/features/request-clip-storyline'
import { ClipDraftPreviewPanel } from '@/features/preview-clip-draft'
import { CancelClipAction } from '@/features/cancel-clip'
import {
  ClipProjectForm,
  MoveClipChannel,
  peekClipMintHandoff,
  type ClipPreparationScreen,
} from '@/features/edit-clip-project'
import {
  ClipVoiceChoice,
  ClipDubbingEditor,
  ClipSpokenProperties,
  ClipSpokenRecoveryEditor,
} from '@/features/regenerate-clip-speech'
import { DeleteClipProjectButton } from '@/features/delete-clip-project'
import { discardClipDraftQueue } from '@/features/edit-clip-project'
import { ClipRegionEditor, discardClipRegionQueue } from '@/features/edit-clip-regions'
import {
  ClipGenerationActions,
  ClipStorylineBuildActions,
  ClipCreditSettlement,
  ClipResult,
  readyClipBatch,
} from '@/features/generate-clip'
import { ClipBrowserRenderStatus } from '@/features/render-clip-browser'
import { ClipUploadText } from '@/features/clip-upload-text'
import { StageModelSelect } from '@/features/select-model'
import { ClipSourcePicker } from '@/features/upload-clip-sources'
import { ClipObservationViewer, ClipAttemptInspection } from '@/features/inspect-clip-observations'
import { Button, Sheet, SegmentedControl, ProgressBar, Typography } from '@/shared/ui'

import { useRunFocus } from '../model/lifecycle'
import { useClipWorkspace } from '../model/useClipWorkspace'
import { clipStepLabel, clipSteps, preparationOpening } from '../model/steps'
import { ClipPreparationGuide } from './ClipPreparationGuide'
import { ClipPreparationSources } from './ClipPreparationSources'
import { RetainedPreparationMedia } from './RetainedPreparationMedia'
import { usePreparationClock } from '../model/usePreparationClock'
import { ClipStepWaiting, ClipTopRow, STEP_PANEL_ID } from './ClipTopRow'
import { ClipProgressBar, ClipStatusLine, type CorrectionStatus } from './ClipStatus'
import { ClipRefineDock } from './ClipRefineDock'
import { ClipFinishStep } from './ClipFinishStep'
import { ClipFinishChecklist } from './ClipFinishChecklist'
import { ClipMoreCard } from './ClipMoreCard'

/** The clip workspace: ONE mounted block holding the three steps of a project (CLIP-36). The
 *  page routes to it and composes it; every hook the steps run on lives in `useClipWorkspace`. */
export function ClipWorkspace({
  ownerId,
  project,
  sourceFeedback,
  initialGuide,
}: {
  ownerId: string
  project: ClipProject
  sourceFeedback?: ReactNode
  /** The screen ① opens on instead of CLIP-225's opening rule: 템플릿, back from the wizard. */
  initialGuide?: ClipPreparationScreen
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const destination = clipReturnDestination(ownerId, project.id, undefined, project.channel)
  const returnTo = {
    href: destination.href,
    label:
      destination.path === '/my'
        ? t('navigation.returnMine')
        : t('navigation.returnCreation', { channel: t(`channel.${destination.channel}`) }),
    onReturn: async (replace = false) => {
      markClipHistoryReturn(ownerId, project.id)
      await navigate({ href: destination.href, replace })
    },
  }
  const workspace = useClipWorkspace(ownerId, project)
  // The run view takes the screen, so it takes the focus: the ref stays OUT of the handles,
  // because a ref reached through an object is a ref read during render.
  const runRoot = useRunFocus(workspace.run.focused)
  const {
    correction,
    generation,
    finalization,
    confirmation,
    render,
    sources,
    revision,
    observations,
    run,
  } = workspace
  const { pending, uploading, plan, save, regions, storyline, flushAll } = workspace
  const upload = sources.upload
  const step = workspace.step.value
  const setStep = workspace.step.set
  // A finalized project is READ in ① and ②: the same steps, the same places, values instead of
  // controls, and no original behind them (CLIP-160, CLIP-76).
  const reading = !!project.finalized
  const reasonId = useId()
  const now = usePreparationClock([
    upload.readyBatch?.expiresAt,
    ...upload.entries.map((entry) => entry.retentionExpiresAt),
  ])
  const sourceReady = readyClipBatch(upload.readyBatch, project.id, now)
  const job = run.job
  // The 업로드 문구 on screen in ③, which 더 해 보기 can turn to the other channel (CLIP-233).
  const [uploadChannel, setUploadChannel] = useState<ClipChannel>(() =>
    clipChannel(project.channel),
  )
  const uploadTextRef = useRef<HTMLDivElement>(null)
  // A project holding an instruction, a source or a running job opens on 확인, any other on 영상
  // 종류 (CLIP-225). Derived until the owner moves, so sources that load late still count.
  // The first save's address change keeps the screen the unminted guide was on (CLIP-225).
  const [chosenGuide, setGuide] = useState<ClipPreparationScreen | undefined>(
    () => initialGuide ?? peekClipMintHandoff(project.id)?.screen,
  )
  const guide: ClipPreparationScreen =
    chosenGuide ??
    preparationOpening({
      instruction: project.instruction,
      sources: upload.entries.length,
      running: !!job && !isTerminal(job),
    })
  const browserStatus = (
    <ClipBrowserRenderStatus
      state={render.browser.state}
      cancel={render.browser.cancel}
      retry={render.browser.retry}
    />
  )
  const correctionStatus: CorrectionStatus = correction.dirty
    ? 'dirty'
    : project.editPlanRevision > project.renderedPlanRevision
      ? 'unrendered'
      : 'clean'
  // Shared reference content: ① keeps it in flow; ② mounts only the chosen tab.
  const [editorPanel, setEditorPanel] = useState<'script'>()
  // Tools and lifecycle panels can unmount the composer; the keyed owner/project workspace
  // retains its unsent words while quote and approval state stay inside the feature.
  const [storylineRequest, setStorylineRequest] = useState('')
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

  // Reselection is asked for only once the retained originals are gone — expired, missing or
  // being cleaned up — never inside the retention window the picker discloses (CLIP-21, CLIP-23).
  const originalsGone =
    !upload.readyBatch &&
    upload.entries.some(
      (entry) =>
        !entry.file && ['expired', 'missing', 'cleanup_pending'].includes(entry.availability ?? ''),
    )
  const sourcePanel = (
    <>
      {!reading &&
        originalsGone &&
        (project.result || job?.status === 'failed' || job?.status === 'cancelled') && (
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
    </>
  )
  const modelsPanel = (
    <>
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
    </>
  )
  const generatePanel = (
    <ClipProjectForm
      ownerId={ownerId}
      stored={project}
      disabled={pending || reading}
      readOnly={reading}
      onUploadAllowed={sources.allow}
      dubbing={(value, change) => (
        <RetainedPreparationMedia
          active={!run.focused && step === 'generate' && (reading || guide === 'style')}
        >
          <ClipVoiceChoice
            ownerId={ownerId}
            enabled={value.enabled}
            voiceId={value.voiceId}
            disabled={pending || reading}
            onChange={change}
          />
        </RetainedPreparationMedia>
      )}
      regions={
        regions.regions && {
          enabled: { intro: regions.regions.intro.enabled, outro: regions.regions.outro.enabled },
          setEnabled: regions.setEnabled,
        }
      }
      refusal={reading ? undefined : <ClipFailureNotice failure={generation.failure} />}
      layout={
        reading
          ? undefined
          : (content) => (
              <ClipPreparationGuide
                screen={guide}
                onSelect={setGuide}
                reasonId={reasonId}
                campaign={content.campaign}
                requirements={content.requirements}
                noteEmpty={content.noteEmpty}
                screens={{
                  ...content.screens,
                  sources: (
                    <>
                      {content.screens.sources}
                      {sourceFeedback}
                      <RetainedPreparationMedia
                        active={!run.focused && step === 'generate' && guide === 'sources'}
                      >
                        {sourcePanel}
                      </RetainedPreparationMedia>
                    </>
                  ),
                }}
                rows={content.rows.flatMap((row) =>
                  // The source row reads right after the linked post (CLIP-227).
                  row.id !== 'post'
                    ? [row]
                    : [
                        row,
                        {
                          id: 'sources',
                          label: t('guide.sourceSummary'),
                          screen: 'sources',
                          value: (
                            <ClipPreparationSources
                              sources={sources}
                              ready={sourceReady}
                              now={now}
                            />
                          ),
                        },
                      ],
                )}
                models={modelsPanel}
                actions={content.actions}
                refusal={content.refusal}
                correction={
                  content.inputRefusal
                    ? {
                        screen: content.inputRefusal,
                        label: t('guide.inputRefusal', {
                          screen: t(`guide.${content.inputRefusal}`),
                        }),
                      }
                    : regions.invalid
                      ? {
                          // ② shows the regions in its storyline space and as the scene list's
                          // first and last blocks, each slot carrying its own error (CLIP-17).
                          retry: () => {
                            setStep('refine')
                            // Land on the first slot holding words it cannot draw.
                            requestAnimationFrame(() =>
                              document
                                .querySelector<HTMLElement>(
                                  '[data-clip-regions] [aria-invalid="true"]',
                                )
                                ?.focus(),
                            )
                          },
                          label: t('guide.regionRefusal'),
                        }
                      : save.failing || regions.status.failing
                        ? {
                            retry: () => {
                              void flushAll(true).catch(() => undefined)
                            },
                            label: t('guide.saveRefusal'),
                          }
                        : !sourceReady
                          ? { screen: 'sources', label: t('guide.sourceRefusal') }
                          : !generation.canQuote
                            ? { retry: generation.retryEligibility, label: t('guide.modelRefusal') }
                            : pending || generation.busy || render.browser.busy
                              ? { label: t('guide.busyRefusal') }
                              : undefined
                }
              />
            )
      }
      actions={
        reading
          ? undefined
          : (ready) => (
              <ClipGenerationActions
                stepBar
                describedBy={reasonId}
                preparationRoute={generation.preparationRoute}
                ownerId={ownerId}
                project={project}
                batch={upload.readyBatch}
                observe={generation.observeRef}
                write={generation.writeRef}
                observeStatus={generation.observeStatus}
                ready={
                  ready &&
                  sourceReady &&
                  generation.canQuote &&
                  !generation.busy &&
                  !render.browser.busy &&
                  !regions.invalid
                }
                pending={generation.starting}
                // The queue is flushed BEFORE the run starts, so an approval can never be committed
                // against settings the server has not taken (CLIP-39). A refusal stops the start; the
                // status line is already saying the save failed.
                onApprove={(mode, quote) => {
                  void flushAll(true)
                    .then(() =>
                      generation.start(upload.readyBatch, ready, quote, generation.ownership, mode),
                    )
                    .catch(() => undefined)
                }}
              />
            )
      }
    >
      {reading && sourcePanel}
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

  // The storyline space leads ② whenever the clip has a storyline or intro/outro slots to write
  // (CLIP-178, CLIP-179): the whole step while there is no plan yet, and above the plan once there
  // is one. The region blocks stand in it before any storyline or plan exists.
  const regionBlock = (kind: 'intro' | 'outro') => (
    <ClipRegionEditor editor={regions} kind={kind} projectId={project.id} ratio={project.ratio} />
  )
  const storylineSpace =
    project.storyline || regions.regions ? (
      <ClipStorylineSpace
        ownerId={ownerId}
        project={project}
        hasPlan={!!plan}
        readOnly={reading || generation.busy}
        sourceFiles={upload.readyBatch?.sources.map((s) => ({
          id: s.id,
          filename: s.metadata.filename,
        }))}
        localSources={reading ? [] : sources.localSources}
        resolvePlayback={reading ? undefined : sources.resolvePlayback}
        intro={regionBlock('intro')}
        outro={regionBlock('outro')}
        // Its actions and its request are absent on a finalized clip and held while a job runs
        // (CLIP-160, CLIP-181); both build on a storyline, so neither stands without one.
        aside={
          reading || !project.storyline ? undefined : (
            <ClipStorylineBuildActions
              preparationRoute={generation.preparationRoute}
              ownerId={ownerId}
              project={project}
              batch={upload.readyBatch}
              observe={generation.observeRef}
              write={generation.writeRef}
              observeStatus={generation.observeStatus}
              ready={generation.canQuote && !render.browser.busy && !regions.invalid}
              pending={generation.starting}
              hasPlan={!!plan}
              disabled={generation.busy || uploading}
              onApprove={(mode, quote) => {
                void flushAll()
                  .then(() =>
                    generation.start(upload.readyBatch, true, quote, generation.ownership, mode),
                  )
                  .catch(() => undefined)
              }}
            />
          )
        }
        lead={
          reading || !project.storyline ? undefined : (
            <ClipStorylineRequest
              ownerId={ownerId}
              project={project}
              request={storylineRequest}
              onRequestChange={setStorylineRequest}
              observe={generation.observeRef}
              write={generation.writeRef}
              job={job}
              disabled={generation.busy && job?.kind !== 'revise_storyline_clip'}
              // A storyline request writes the slots too, from their instructions and exact words
              // (CLIP-178), over the storyline the owner is looking at: all three land before it
              // starts.
              flush={() =>
                save
                  .flush()
                  .then(() => regions.flush())
                  .then(() => storyline.flush())
              }
            />
          )
        }
      />
    ) : null

  // ③ alone offers the 업로드 문구, once the plan is confirmed and on a finalized clip (CLIP-211,
  // CLIP-160); a write still running keeps its cancel there whatever the plan became since.
  const finishing = clipConfirmed(project) || reading
  const writingUploadText =
    project.latestJob?.kind === 'clip_upload_text' && !isTerminal(project.latestJob)
  const uploadText =
    project.editPlanRevision > 0 && (finishing || writingUploadText) ? (
      <div ref={uploadTextRef} tabIndex={-1} className="mt-10 outline-none">
        <ClipUploadText
          ownerId={ownerId}
          project={project}
          writeModel={generation.writeRef}
          modelSelect={<StageModelSelect stage="write" disabled={pending} />}
          disabled={pending}
          beforeWrite={() => flushAll(true)}
          channel={uploadChannel}
          onChannelChange={setUploadChannel}
        />
      </div>
    ) : null

  const refinePanel = plan ? (
    <>
      {/* ② stands its storyline space first, closed once there is a plan (CLIP-53, CLIP-179). */}
      {storylineSpace}
      <ClipCorrectionWorkspace
        project={{
          id: project.id,
          ownerId,
          ratio: project.ratio,
          design: {
            captionStyles: project.allowedCaptionStyles,
            captionPace: project.captionPace,
            accent: project.accent,
            introPreset: project.introPreset,
            outroPreset: project.outroPreset,
            disclosure: project.disclosure,
            hideDisclosure: project.hideDisclosure,
          },
          state: plan,
          captionStyles: project.allowedCaptionStyles,
          // A slot's notice stands in its storyline block, beside the words it is about.
          notices: project.notices?.filter((notice) => !regions.slotIds.has(notice.elementId)),
          language: project.language,
        }}
        correction={correction}
        readOnly={reading}
        render={{ failure: render.failure }}
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
                ownerId={ownerId}
                projectId={project.id}
                projectRevision={project.editPlanRevision}
                revision={correction.revision}
                design={{
                  captionStyles: project.allowedCaptionStyles,
                  captionPace: project.captionPace,
                  accent: project.accent,
                  introPreset: project.introPreset,
                  outroPreset: project.outroPreset,
                  disclosure: project.disclosure,
                  hideDisclosure: project.hideDisclosure,
                }}
                layoutObservations={plan.layoutObservations}
                localSources={sources.localSources}
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
          // ②'s step bar: the revision request, then 이전 and 확정하기 (CLIP-40 →POST-56).
          dock: reading ? undefined : (
            <ClipRefineDock
              request={{
                ownerId,
                project,
                observe: generation.observeRef,
                write: generation.writeRef,
                job,
                disabled: revision.disabled,
                flush: revision.flush,
                cancelAction: (
                  <CancelClipAction
                    action={revision.cancellation}
                    job={job}
                    accounting={generation.accounting}
                  />
                ),
                scope: correction.selectedBlock
                  ? {
                      cut: correction.selectedBlock,
                      clear: () => correction.dispatch({ type: 'select', selection: undefined }),
                    }
                  : undefined,
              }}
              confirm={confirmation.action}
              refusal={confirmation.refusal}
              fix={
                confirmation.refusal === 'narration' ? (
                  <Button variant="ghost" onClick={() => setEditorPanel('script')}>
                    {t('refineDock.openScript')}
                  </Button>
                ) : undefined
              }
              onBack={() => {
                setStep('generate')
                window.scrollTo(0, 0)
              }}
            />
          ),
          tools: (
            <>
              <Button variant="ghost" onClick={() => setEditorPanel('script')}>
                {t('editorEntries.script')}
              </Button>
            </>
          ),
          addDubbing: () => setEditorPanel('script'),
          spokenProperties: (segment) =>
            correction.draft.narration && (
              <ClipSpokenProperties
                ownerId={ownerId}
                projectId={project.id}
                narration={correction.draft.narration}
                segment={segment}
                durationMs={correction.draft.durationMs}
                disabled={pending || reading}
                change={correction.change}
              />
            ),
          referenceAction,
          // 장면 추가 opens the observed-scene chooser the reference sheet holds; the cut it adds
          // lands after the selected block (CLIP-55, CLIP-67).
          addScene:
            !reading && observations.addCut
              ? () => {
                  setReferenceTab('observations')
                  setReferenceOpen(true)
                }
              : undefined,
          sourceSound: (cut) => {
            const entry = upload.entries.find(
              (entry) =>
                entry.sourceId === cut.sourceId && entry.metadata.fingerprint === cut.fingerprint,
            )
            if (!entry) return undefined
            const source = {
              batchId: entry.batchId ?? '',
              sourceId: cut.sourceId,
              fingerprint: cut.fingerprint,
              retainOriginalAudio: entry.retainOriginalAudio ?? false,
            }
            return {
              value: sources.sound.value(source),
              change:
                reading || sources.sound.disabled
                  ? undefined
                  : (enabled: boolean) => sources.sound.change(source, enabled),
            }
          },
          intro: regions.regions ? regionBlock('intro') : undefined,
          outro: regions.regions ? regionBlock('outro') : undefined,
        }}
      />
      <Sheet
        open={editorPanel !== undefined}
        labelledBy="clip-workspace-tool-title"
        onClose={() => setEditorPanel(undefined)}
        header={
          <Typography id="clip-workspace-tool-title" variant="fieldTitle">
            {t('editorEntries.script')}
          </Typography>
        }
      >
        <ClipDubbingEditor
          ownerId={ownerId}
          project={project}
          plan={correction.draft}
          state={plan}
          revision={correction.revision}
          disabled={pending || reading}
          change={correction.change}
          flush={() => flushAll(true)}
          onSelect={(segment) => {
            correction.dispatch({ type: 'select', selection: { kind: 'spoken', id: segment.id } })
            setEditorPanel(undefined)
          }}
        />
      </Sheet>
    </>
  ) : (
    <>
      <ClipFailureNotice failure={generation.failure} />
      {!reading && (
        <ClipSpokenRecoveryEditor
          ownerId={ownerId}
          projectId={project.id}
          durationMs={project.targetDurationMs}
          disabled={pending}
        />
      )}
      {storylineSpace}
      {!project.storyline && (
        <ClipStepWaiting message={t('steps.refineWaiting')} onGo={() => setStep('generate')} />
      )}
      {project.result && (
        <>
          <Typography variant="meta">
            {t('preview.renderedRevision', { revision: project.renderedPlanRevision })}
          </Typography>
          <ClipResult ownerId={ownerId} project={project} />
        </>
      )}
      {referenceAction}
    </>
  )

  const finishPanel = (
    <ClipFinishStep
      ownerId={ownerId}
      project={project}
      render={render}
      finalization={finalization}
      disabled={uploading || generation.busy || render.browser.busy || finalization.busy}
      startRender={workspace.finish.startRender}
      onStartRenderHandled={workspace.finish.handled}
      onGoRefine={() => {
        setStep('refine')
        window.scrollTo(0, 0)
      }}
      slots={{
        checklist:
          finishing && project.campaign ? (
            <ClipFinishChecklist key={project.id} ownerId={ownerId} project={project} />
          ) : undefined,
        uploadText,
        more: (
          <ClipMoreCard
            ownerId={ownerId}
            project={project}
            onOtherChannel={(other) => {
              setUploadChannel(other)
              const panel = uploadTextRef.current
              panel?.scrollIntoView?.({ block: 'start' })
              panel?.focus({ preventScroll: true })
            }}
          />
        ),
        channel: reading ? (
          <MoveClipChannel ownerId={ownerId} project={project} disabled={pending} />
        ) : undefined,
      }}
    />
  )

  return (
    <>
      {/* First child of the flow on purpose: a sticky box can only be pinned by the box it sits
          in, and this one has to hold the page's top edge while the panel scrolls past it. */}
      {!run.focused && !project.finalized && (
        <ClipProgressBar job={job?.kind === 'speech_clip' ? job : undefined} upload={upload} />
      )}
      <ClipTopRow
        returnTo={returnTo}
        contextTitle={clipDisplayTitle(project)}
        status={
          !run.focused && (
            <ClipStatusLine
              project={project}
              job={job}
              upload={upload}
              correction={correctionStatus}
              save={workspace.saveStatus}
              className="order-last w-full overflow-visible! break-words text-clip! whitespace-normal!"
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
              onReturn={() => returnTo.onReturn(true)}
              // A queue outlives its form, so a retry left running would keep saving an id the
              // server no longer has. Stopped before the navigation unmounts the page.
              onDeleted={() => {
                discardClipDraftQueue(project.id)
                discardClipRegionQueue(project.id)
                discardClipStorylineQueue(project.id)
              }}
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
            <ClipBrowserPreparationStatus {...generation.analysis} />
            {!generation.analysis.busy && (
              <ProgressBar
                label={run.title}
                done={run.progress?.done}
                total={run.progress?.total}
              />
            )}
            <ClipSourcePicker upload={upload} processing readOnly />
            {!generation.analysis.busy && (
              <CancelClipAction
                action={revision.cancellation}
                job={job}
                accounting={generation.accounting}
              />
            )}
          </>
        )}
      </section>
      {!run.focused && <ClipBrowserPreparationStatus {...generation.analysis} />}
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
          {job?.kind === 'speech_clip' && !isTerminal(job) && !generation.analysis.busy && (
            <CancelClipAction
              action={revision.cancellation}
              job={job}
              accounting={generation.accounting}
            />
          )}
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
      {/* ③ shows its own render progress; ① keeps a render the owner left running in view. */}
      {!run.focused && step === 'generate' && browserStatus}
      {/* A focused run owns the screen: the panel stays mounted for ①'s retained inputs, but
          leaves the accessibility tree and the column's free height to the run. */}
      <div
        id={STEP_PANEL_ID}
        role="tabpanel"
        aria-label={clipStepLabel(step)}
        hidden={run.focused}
        className="flex flex-1 flex-col"
      >
        <div hidden={step !== 'generate'} className="flex flex-1 flex-col">
          {generatePanel}
        </div>
        {!run.focused && step === 'refine' && refinePanel}
        {!run.focused && step === 'finish' && finishPanel}
      </div>
    </>
  )
}
