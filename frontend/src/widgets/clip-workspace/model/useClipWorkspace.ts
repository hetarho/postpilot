import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipRenderNeedsAudio,
  useClipBrowserRenderCapability,
  type ClipBrowserRenderCapability,
} from '@/entities/clip-preview'
import {
  useReorderClipSources,
  type ClipProject,
  type ClipRenderKind,
} from '@/entities/clip-project'
import { isTerminal, progressLabel, progressRatio } from '@/entities/generation-job'
import { useClipCorrection } from '@/features/correct-clip'
import { useCancelClip } from '@/features/cancel-clip'
import { discardClipDraftQueue, useClipDraftSave } from '@/features/edit-clip-project'
import { useFinalizeClip } from '@/features/finalize-clip'
import { useGenerateClip } from '@/features/generate-clip'
import { useBrowserRender } from '@/features/render-clip-browser'
import { useClipSourceBinding } from '@/features/bind-clip-source-item'
import { useClipSourceUpload } from '@/features/upload-clip-sources'
import {
  useDiscardQueueWhenFinalized,
  useSoundBatchHandoff,
  useUploadAttemptLifecycle,
} from './lifecycle'
import { requiredSourcesForStep, soundRetryAction, unsavedCorrection } from './rules'
import { stepForProject, type ClipStep } from './steps'

/** Every hook the workspace runs on lives HERE, above the panels: the steps are panels of ONE
 *  mounted widget (CLIP-36), so changing step cannot remount the upload session, restart the job
 *  poll or throw away a correction draft the owner has not saved yet.
 *
 *  The return is handles — one object per concern — rather than a flat list of scalars: the
 *  workspace's ui and the correction feature both read what they need from the same shape, and
 *  a new field on one concern does not widen every signature between here and there. */
export function useClipWorkspace(ownerId: string, project: ClipProject) {
  const { t } = useTranslation('clips')
  // Follow a lifecycle change immediately; a successful render stays in correction.
  // Manual tab selection otherwise stands until the project's derived step changes.
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
  const correction = useClipCorrection(ownerId, project)
  const plan = project.editing
  const required = requiredSourcesForStep(step, correction.draft, plan)
  const upload = useClipSourceUpload(project.id, required, !project.finalized)
  const binding = useClipSourceBinding(
    ownerId,
    project,
    upload.entries.map((entry) => ({
      fingerprint: entry.metadata.fingerprint,
      sourceId: entry.sourceId,
    })),
  )
  useSoundBatchHandoff(correction.soundBatch, upload.acceptSoundBatch)
  const generation = useGenerateClip(ownerId, project, upload.attempt?.jobId)
  const ownership = {
    begin: upload.beginAttempt,
    owned: upload.markOwned,
    rejected: upload.rejectAttempt,
  }
  const browser = useBrowserRender(ownerId, project.id)
  const job = generation.job
  const browserCapability = useClipBrowserRenderCapability(
    project.ratio,
    clipRenderNeedsAudio(correction.draft),
  )
  useUploadAttemptLifecycle(job, upload)
  const uploading = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  const finalization = useFinalizeClip(ownerId, project, async () => {
    await save.flush(true)
    return correction.flush()
  })
  const cancellation = useCancelClip(ownerId, project.id, job)
  // A revision runs WITHOUT the focused job view: it rewrites the plan the owner
  // is looking at, and ② is where that change shows up (CLIP-131). Every other
  // clip job still takes the whole screen (CLIP-78).
  const revising = job?.kind === 'revise_clip' && !isTerminal(job)
  const focused = !project.finalized && generation.busy && !revising
  const pending = generation.busy || uploading || finalization.busy || browser.busy
  useDiscardQueueWhenFinalized(project.id, project.finalized, discardClipDraftQueue)
  const reorderSources = useReorderClipSources()
  const localSources = upload.entries.map((entry) => ({
    fingerprint: entry.metadata.fingerprint,
    url: entry.previewURL,
  }))
  /** The plan the owner may still leave behind, watched by the page's navigation guard. */
  const unsaved = unsavedCorrection(correction.dirty, project.finalized)
  /** Everything a run has to say while it owns the screen. */
  const run = {
    focused,
    job,
    cancellation,
    progress: job ? progressRatio(job) : undefined,
    title: cancellation.cancelling
      ? t('cancellation.cancelling')
      : job
        ? progressLabel(job)
        : t('generation.running'),
  }
  const sources = {
    upload,
    binding,
    required,
    localSources,
    resolvePlayback: upload.ensurePlayback,
    allowed: uploadAllowed,
    allow: setUploadAllowed,
    sound: {
      value: correction.soundValue,
      change: correction.setSourceSound,
      disabled: pending || !!project.finalized,
      pending: correction.pending,
      failed: correction.hasUnsavedSound && !!correction.failure,
      retry: soundRetryAction(correction),
    },
    // The order the footage plays in when no instruction directs otherwise (CLIP-136). It is
    // stored on the batch, so the refreshed batch is what the strip then shows.
    order: {
      // Arranging is refused while a generation runs or after finalization; an upload in flight
      // is not a reason, since the strip itself hides the control until the footage is there.
      disabled: generation.busy || browser.busy || finalization.busy || !!project.finalized,
      change: (sourceIds: string[]) => {
        const batchId = upload.readyBatch?.id
        if (!batchId) return
        void reorderSources({ projectId: project.id, batchId, sourceIds }).then(
          upload.acceptSourceOrder,
        )
      },
    },
  }
  const render = {
    browser,
    capability: browserCapability.data as ClipBrowserRenderCapability | undefined,
    ready: !!upload.readyBatch && correction.revision === project.editPlanRevision,
    pending: generation.starting || browser.busy,
    lastKind: project.lastRenderKind,
    current:
      !!project.result && !correction.dirty && project.renderedPlanRevision === correction.revision,
    // A revision states its own refusal beside its composer: the dock's alert is about the
    // render it commits.
    failure: job?.kind === 'revise_clip' ? undefined : generation.failure,
    // The draft's queue is flushed BEFORE the render starts, so the render always runs against
    // the revision the server took rather than against one the owner has since typed past
    // (CLIP-39). A failed save stops the start; the status line already says so.
    start: (kind: ClipRenderKind) => {
      if (correction.pending || browser.busy) return
      if (kind === 'browser') {
        if (!browserCapability.data?.available || !upload.readyBatch) return
        void browser.start({
          batchId: upload.readyBatch.id,
          localSources,
          resolvePlayback: upload.ensurePlayback,
          flush: async () => {
            await save.flush(true)
            return correction.flush()
          },
        })
        return
      }
      void correction
        .flush()
        .then((revision) => generation.render(upload.readyBatch, revision, ownership))
        .catch(() => undefined)
    },
  }
  const revision = {
    job,
    revising,
    // A revision is refused while any other clip job holds the project, and after finalization;
    // its own run is not a reason, since the composer then shows that run instead of the field.
    disabled: uploading || finalization.busy || browser.busy || (generation.busy && !revising),
    // The same chain a generation runs before it starts (CLIP-39): the writer answers from the
    // SAVED plan, so an unflushed edit would be silently dropped from what it rewrites.
    flush: async () => {
      await save.flush()
      return correction.flush()
    },
    cancellation,
  }
  const observations = {
    localSources,
    resolvePlayback: upload.ensurePlayback,
    /** Adding a cut from recorded evidence is ②'s, and only while nothing else holds the plan. */
    addCut:
      step === 'refine' && !pending && plan?.plan.nativeComposition ? correction.addCut : undefined,
    draftPlan: step === 'refine' ? correction.draft : undefined,
    soundOf: (selection: { source: { id: string; fingerprint: string } }) =>
      upload.readyBatch?.sources.find(
        (source) =>
          source.id === selection.source.id &&
          source.metadata.fingerprint === selection.source.fingerprint,
      )?.retainOriginalAudio,
  }
  return {
    project,
    ownerId,
    plan,
    step: { value: step, set: setStep },
    save,
    correction,
    generation: { ...generation, ownership },
    sources,
    render,
    revision,
    observations,
    finalization,
    run,
    pending,
    uploading,
    unsaved,
  }
}

export type ClipWorkspace = ReturnType<typeof useClipWorkspace>
