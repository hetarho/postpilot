import { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useStartGeneration,
  useStartStoryline,
  type GenerationJob,
} from '@/entities/generation-job'
import { isPublished, type PostDraft } from '@/entities/post'
import { useSelectionSavePending } from '@/entities/model-catalog'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  Dialog,
  FieldMessage,
  Notice,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { needsPicker } from '../model/reobserve'
import {
  isSetupBlocker,
  ordinaryGenerationPreconditions,
  type GenerationMode,
  type GenerationPreconditions,
} from '../model/preconditions'
import { useGenerationSelections } from '../model/useBriefIssues'
import { ReobservePicker } from './ReobservePicker'

export interface GenerationActionsHandle {
  /** 바로 글 쓰기 — the name the retry path has always called. */
  startGeneration: () => void
  /** 스토리라인 먼저, and the retry of a storyline job started from ①. */
  startStoryline: () => void
}

/** Ordinary creation actions use the active observer and writer independently of tests. */
type ActionMode = 'generation' | 'storyline'

export const GenerationActions = forwardRef<
  GenerationActionsHandle,
  {
    post: Pick<
      PostDraft,
      'slug' | 'status' | 'images' | 'videos' | 'observations' | 'pendingExperimentId' | 'voice'
    >
    /** Owned by the editor, not by this action: the writing brief sets it from another layer
     *  (`widgets/generation-brief`) and the two must agree on what the next run is given. */
    targetLength?: number
    activeJob?: GenerationJob
    jobPending?: boolean
    onStarted: (jobId: string) => void
    beforeStart?: () => Promise<void>
    /** The page flushes both material/content queues before opening the retained paid result. */
    onReviewResult?: () => Promise<void> | void
    /** Returns false after pointing the author to missing required template answers. */
    checkRequiredAnswers?: () => boolean
    /** Opens the writing brief — where the active 관찰/작성 models are chosen — for a press
     *  of `mode` that its setup refused, so the brief can mark what that run is missing. Supplied
     *  by `pages/editor`, which owns the brief's open state — a feature may not import the widget
     *  that composes its siblings (ARCH-13). */
    onOpenBrief: (mode: GenerationMode) => void
  }
>(function GenerationActions(
  {
    post,
    targetLength,
    activeJob,
    jobPending = false,
    onStarted,
    beforeStart,
    onReviewResult,
    checkRequiredAnswers,
    onOpenBrief,
  },
  ref,
) {
  const { t } = useTranslation('posts')
  const selections = useGenerationSelections()
  const selectionSaving = useSelectionSavePending()
  const generation = useStartGeneration()
  const storyline = useStartStoryline()
  const [preparing, setPreparing] = useState<ActionMode | ''>('')
  const [prepareFailure, setPrepareFailure] = useState<AppFailure>()
  const [reviewingResult, setReviewingResult] = useState(false)
  // The mode a confirmed picker will start. Non-empty IS the picker's open state: there is one
  // picker for both actions, and the answer only means something together with the action.
  const [picking, setPicking] = useState<ActionMode | ''>('')
  // The mode waiting on the no-photo confirmation; non-empty is that dialog's open state.
  const [confirmingEmpty, setConfirmingEmpty] = useState<ActionMode | ''>('')

  const { observe: observeSelection, write: writeSelection } = selections
  const published = isPublished(post)
  const ordinary = ordinaryGenerationPreconditions({
    images: post.images,
    videos: post.videos,
    published,
    activeJob,
    voice: post.voice,
    observe: observeSelection,
    write: writeSelection,
  })
  // `modelPending` is load-bearing: `useStageSelection` reports `selected: null` for the whole
  // fetch, so without it every visit to 글 생성 would treat an unanswered catalog as a missing
  // model and send the press to the brief.
  const modelPending = selections.isPending || selectionSaving
  const busy =
    jobPending ||
    Boolean(preparing) ||
    generation.isPending ||
    storyline.isPending ||
    reviewingResult
  const sharedDisabled = modelPending || busy

  // `reobserveFiles` undefined is a start with no re-observation decision (no picker was
  // shown), which observes every attached photo. An empty array is the picker's answer to reuse
  // everything, and the two must not collapse into one.
  const enqueue = useCallback(
    async (mode: ActionMode, reobserveFiles?: readonly string[]) => {
      // The WHOLE guard, re-checked here and not only in `start`: the picker can sit open long
      // enough for a catalog refetch to disable the observe model, for an ordinary job to
      // appear, or for the voice to be deleted. Confirming a dialog that went stale must not
      // force a draft save and fire an RPC the server is going to refuse.
      if (sharedDisabled || !ordinary.ok || !writeSelection) return
      if (checkRequiredAnswers?.() === false) return
      setPreparing(mode)
      setPrepareFailure(undefined)
      // Deliberately here and not before the picker: a cancelled picker must not have forced a
      // draft save, so the save sits immediately before the RPC that consumes it.
      try {
        await beforeStart?.()
      } catch (cause) {
        setPrepareFailure(appFailureFromConnect(cause))
        setPreparing('')
        return
      }
      const observeRef =
        post.images.length || post.videos.length ? observeSelection?.ref : undefined
      try {
        const response =
          mode === 'generation'
            ? await generation.start(
                post.slug,
                observeRef,
                writeSelection.ref,
                targetLength,
                reobserveFiles,
              )
            : await storyline.start(post.slug, observeRef, writeSelection.ref, reobserveFiles)
        onStarted(response.jobId)
      } catch {
        // The mode-specific mutation renders its transport error below the actions.
      } finally {
        setPreparing('')
      }
    },
    [
      beforeStart,
      checkRequiredAnswers,
      generation,
      observeSelection,
      onStarted,
      ordinary,
      post.images.length,
      post.videos.length,
      post.slug,
      sharedDisabled,
      storyline,
      targetLength,
      writeSelection,
    ],
  )

  const start = useCallback(
    async (mode: ActionMode) => {
      if (sharedDisabled) return
      // A model the run needs is not chosen, or cannot watch this post's media. The press is not
      // refused in place: it opens the brief with that field marked, which is where the fix is
      // (owner decision 2026-09-25). 스토리라인 먼저 needs what 바로 글 쓰기 needs.
      if (refusedForSetup(ordinary)) {
        onOpenBrief('generation')
        return
      }
      if (!ordinary.ok || !writeSelection) return
      if (checkRequiredAnswers?.() === false) return
      // A run with nothing attached writes from the text alone, and a forgotten upload is the
      // likelier story, so it asks once before spending the run (POST-108).
      if (!post.images.length && !post.videos.length) {
        setConfirmingEmpty(mode)
        return
      }
      // A post with observations worth reusing decides what to re-observe first; one with
      // nothing to reuse would observe everything either way, so it starts directly.
      if (needsPicker(post.images, post.observations, post.videos)) {
        setPicking(mode)
        return
      }
      await enqueue(mode)
    },
    [
      checkRequiredAnswers,
      enqueue,
      onOpenBrief,
      ordinary,
      post.images,
      post.observations,
      post.videos,
      sharedDisabled,
      writeSelection,
    ],
  )

  useImperativeHandle(
    ref,
    () => ({
      startGeneration: () => void start('generation'),
      startStoryline: () => void start('storyline'),
    }),
    [start],
  )

  return (
    <div>
      {/* The one place ① says why it is locked (POST-86): both actions are refused for it, and no
          control under the lock names a reason of its own. */}
      {ordinary.blocker === 'published' && (
        <Typography variant="label" as="p" role="status" className="mb-2">
          {ordinary.reason}
        </Typography>
      )}
      {/* Storyline first is the secondary path; writing now remains the committing action. */}
      <div className="grid grid-cols-[3fr_7fr] gap-3 sm:flex sm:flex-wrap sm:items-center sm:justify-end">
        {/* A refusal for the SETUP leaves an action live: pressing it is how the user is taken to
            the brief with the missing field marked, and nothing is written under the row. Every
            other refusal — a job running, a deleted or not-yet-made voice,
            a published post — keeps it disabled, because no field fixes those. */}
        <Button
          variant="secondary"
          disabled={sharedDisabled || (!ordinary.ok && !refusedForSetup(ordinary))}
          pending={preparing === 'storyline' || storyline.isPending}
          onClick={() => void start('storyline')}
        >
          {t('generation.storylineFirst')}
        </Button>
        <Button
          variant="cta"
          className="min-w-0"
          disabled={sharedDisabled || (!ordinary.ok && !refusedForSetup(ordinary))}
          pending={preparing === 'generation' || generation.isPending}
          onClick={() => void start('generation')}
        >
          {t('generation.writeNow')}
        </Button>
      </div>
      {post.pendingExperimentId && (
        <a
          href={`/posts/experiments/${encodeURIComponent(post.pendingExperimentId)}`}
          className={buttonStyles({ variant: 'ghost', className: 'mt-2 w-full sm:w-auto' })}
          aria-disabled={reviewingResult || undefined}
          aria-busy={reviewingResult || undefined}
          onClick={(event) => {
            if (
              !onReviewResult ||
              event.button !== 0 ||
              event.metaKey ||
              event.ctrlKey ||
              event.altKey ||
              event.shiftKey
            )
              return
            event.preventDefault()
            if (reviewingResult) return
            setReviewingResult(true)
            void Promise.resolve()
              .then(onReviewResult)
              .catch(() => {
                // The page's autosave status exposes a refused flush; the retained result
                // remains available and no default document navigation may bypass that refusal.
              })
              .finally(() => setReviewingResult(false))
          }}
        >
          {t('generation.reviewResult')}
        </a>
      )}
      {(generation.isError || storyline.isError) && (
        <FieldMessage className="mt-2">
          {generation.errorMessage || storyline.errorMessage}
        </FieldMessage>
      )}
      {prepareFailure && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={prepareFailure} />
        </Notice>
      )}
      <ReobservePicker
        open={Boolean(picking)}
        images={post.images}
        videos={post.videos}
        observations={post.observations}
        observeModel={observeSelection?.ref}
        pending={Boolean(preparing)}
        onConfirm={(files) => {
          const mode = picking
          setPicking('')
          if (mode) void enqueue(mode, files)
        }}
        // Cancel enqueues nothing and saves nothing — the draft save lives on the confirm path.
        onCancel={() => setPicking('')}
      />
      <Dialog
        open={Boolean(confirmingEmpty)}
        title={t('noPhotos.title')}
        confirmLabel={t('noPhotos.confirm')}
        onClose={() => setConfirmingEmpty('')}
        onConfirm={() => {
          const mode = confirmingEmpty
          setConfirmingEmpty('')
          if (mode) void enqueue(mode)
        }}
      >
        {t('noPhotos.body')}
      </Dialog>
    </div>
  )
})

/** Refused only because a brief field is missing or cannot serve this post. */
function refusedForSetup(precondition: GenerationPreconditions): boolean {
  return !precondition.ok && isSetupBlocker(precondition.blocker)
}
