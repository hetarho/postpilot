import { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useStartGeneration,
  useStartStoryline,
  type GenerationJob,
} from '@/entities/generation-job'
import { useStartWriteExperiment } from '@/entities/model-experiment'
import { isPublished, type PostDraft } from '@/entities/post'
import { useSelectionSavePending } from '@/entities/model-catalog'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  ActionMenu,
  AppFailureMessage,
  Button,
  FieldMessage,
  Notice,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { needsPicker } from '../model/reobserve'
import {
  comparisonGenerationPreconditions,
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
  startComparison: () => void
  /** 스토리라인 먼저, and the retry of a storyline job started from ①. */
  startStoryline: () => void
}

/** The three things ① starts. 스토리라인 먼저 and 바로 글 쓰기 share the ordinary checks (the write
 *  model and the shared ones); A/B 비교 has the pair's. */
type ActionMode = 'generation' | 'comparison' | 'storyline'

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
    /** Opens the writing brief — where the 관찰/작성 모델 and the A/B pair are chosen — for a press
     *  of `mode` that its setup refused, so the brief can mark what that run is missing. Supplied
     *  by `pages/editor`, which owns the brief's open state — a feature may not import the widget
     *  that composes its siblings (ARCH-13). */
    onOpenBrief: (mode: GenerationMode) => void
  }
>(function GenerationActions(
  { post, targetLength, activeJob, jobPending = false, onStarted, beforeStart, onOpenBrief },
  ref,
) {
  const { t } = useTranslation('posts')
  const selections = useGenerationSelections()
  const selectionSaving = useSelectionSavePending()
  const generation = useStartGeneration()
  const storyline = useStartStoryline()
  const comparison = useStartWriteExperiment()
  const [preparing, setPreparing] = useState<ActionMode | ''>('')
  const [prepareFailure, setPrepareFailure] = useState<AppFailure>()
  // The mode a confirmed picker will start. Non-empty IS the picker's open state: there is one
  // picker for both actions, and the answer only means something together with the action.
  const [picking, setPicking] = useState<ActionMode | ''>('')

  const { observe: observeSelection, write: writeSelection, writeA, writeB } = selections
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
  const ab = comparisonGenerationPreconditions({
    images: post.images,
    videos: post.videos,
    published,
    activeJob,
    voice: post.voice,
    observe: observeSelection,
    writeA,
    writeB,
  })
  const pendingExperiment = Boolean(post.pendingExperimentId)
  // `modelPending` is load-bearing: `useStageSelection` reports `selected: null` for the whole
  // fetch, so without it every visit to 글 생성 would treat an unanswered catalog as a missing
  // model and send the press to the brief.
  const modelPending = selections.isPending || selectionSaving
  const busy =
    jobPending ||
    Boolean(preparing) ||
    generation.isPending ||
    storyline.isPending ||
    comparison.isPending
  const sharedDisabled = modelPending || busy || pendingExperiment

  // `reobserveFiles` undefined is a start with no re-observation decision (no picker was
  // shown), which observes every attached photo. An empty array is the picker's answer to reuse
  // everything, and the two must not collapse into one.
  const enqueue = useCallback(
    async (mode: ActionMode, reobserveFiles?: readonly string[]) => {
      // The WHOLE guard, re-checked here and not only in `start`: the picker can sit open long
      // enough for a catalog refetch to disable the observe model, for a job or an A/B result to
      // appear, or for the voice to be deleted. Confirming a dialog that went stale must not
      // force a draft save and fire an RPC the server is going to refuse.
      const precondition = mode === 'comparison' ? ab : ordinary
      if (sharedDisabled || !precondition.ok) return
      if (mode !== 'comparison' && !writeSelection) return
      if (mode === 'comparison' && (!writeA || !writeB)) return
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
                writeSelection!.ref,
                targetLength,
                reobserveFiles,
              )
            : mode === 'storyline'
              ? await storyline.start(post.slug, observeRef, writeSelection!.ref, reobserveFiles)
              : await comparison.start(
                  post.slug,
                  'editor',
                  observeRef,
                  writeA!.ref,
                  writeB!.ref,
                  targetLength,
                  reobserveFiles,
                )
        onStarted(response.jobId)
      } catch {
        // The mode-specific mutation renders its transport error below the actions.
      } finally {
        setPreparing('')
      }
    },
    [
      ab,
      beforeStart,
      comparison,
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
      writeA,
      writeB,
      writeSelection,
    ],
  )

  const start = useCallback(
    async (mode: ActionMode) => {
      const precondition = mode === 'comparison' ? ab : ordinary
      if (sharedDisabled) return
      // A model the run needs is not chosen, or cannot watch this post's media. The press is not
      // refused in place: it opens the brief with that field marked, which is where the fix is
      // (owner decision 2026-09-25). 스토리라인 먼저 needs what 바로 글 쓰기 needs.
      if (refusedForSetup(precondition)) {
        onOpenBrief(briefMode(mode))
        return
      }
      if (!precondition.ok) return
      if (mode !== 'comparison' && !writeSelection) return
      if (mode === 'comparison' && (!writeA || !writeB)) return
      // A post with observations worth reusing decides what to re-observe first; one with
      // nothing to reuse would observe everything either way, so it starts directly.
      if (needsPicker(post.images, post.observations, post.videos)) {
        setPicking(mode)
        return
      }
      await enqueue(mode)
    },
    [
      ab,
      enqueue,
      onOpenBrief,
      ordinary,
      post.images,
      post.observations,
      post.videos,
      sharedDisabled,
      writeA,
      writeB,
      writeSelection,
    ],
  )

  useImperativeHandle(
    ref,
    () => ({
      startGeneration: () => void start('generation'),
      startComparison: () => void start('comparison'),
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
      {/* ONE row on a phone, 3 : 7: 스토리라인 먼저 left, 바로 글 쓰기 — the committing action — right
          with the ▾ that holds A/B 비교 beside it, which is both the THEME-22 emphasis order and the
          side the thumb of a right-handed one-handed grip reaches first. Not halves: writing the
          post is what this step is FOR, a storyline first is the careful path, and an A/B
          comparison is the occasional second opinion kept one press further in. From `sm:` up the
          row right-aligns at its natural width. */}
      <div className="grid grid-cols-[3fr_7fr] gap-3 sm:flex sm:flex-wrap sm:items-center sm:justify-end">
        {/* A refusal for the SETUP leaves an action live: pressing it is how the user is taken to
            the brief with the missing field marked, and nothing is written under the row. Every
            other refusal — a job running, a deleted or not-yet-made voice, a pending A/B result,
            a published post — keeps it disabled, because no field fixes those. */}
        <Button
          variant="secondary"
          disabled={sharedDisabled || (!ordinary.ok && !refusedForSetup(ordinary))}
          pending={preparing === 'storyline' || storyline.isPending}
          onClick={() => void start('storyline')}
        >
          {t('generation.storylineFirst')}
        </Button>
        <div className="flex min-w-0 gap-2">
          <Button
            variant="cta"
            className="min-w-0 flex-1 sm:flex-none"
            disabled={sharedDisabled || (!ordinary.ok && !refusedForSetup(ordinary))}
            pending={preparing === 'generation' || generation.isPending}
            onClick={() => void start('generation')}
          >
            {t('generation.writeNow')}
          </Button>
          <ActionMenu
            label={t('generation.otherWays')}
            // The menu holds A/B 비교 alone, so it is held exactly as that button was: by a job, a
            // pending A/B result, a save in flight, or any refusal no brief field fixes. A setup
            // refusal keeps it live — choosing it opens the brief on the pair.
            disabled={sharedDisabled || (!ab.ok && !refusedForSetup(ab))}
            items={[
              {
                id: 'compare',
                label: t('generation.compare'),
                onSelect: () => void start('comparison'),
              },
            ]}
          />
        </div>
      </div>
      {pendingExperiment && (
        <a
          href={`/posts/experiments/${encodeURIComponent(post.pendingExperimentId)}`}
          className={buttonStyles({ variant: 'secondary', className: 'mt-2 w-full sm:w-auto' })}
        >
          {t('generation.reviewResult')}
        </a>
      )}
      {(generation.isError || storyline.isError || comparison.isError) && (
        <FieldMessage className="mt-2">
          {generation.errorMessage || storyline.errorMessage || comparison.errorMessage}
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
    </div>
  )
})

/** The brief marks the fields of the run a press was refused for; 스토리라인 먼저 waits on
 *  바로 글 쓰기's. */
function briefMode(mode: ActionMode): GenerationMode {
  return mode === 'comparison' ? 'comparison' : 'generation'
}

/** Refused only because a brief field is missing or cannot serve this post. */
function refusedForSetup(precondition: GenerationPreconditions): boolean {
  return !precondition.ok && isSetupBlocker(precondition.blocker)
}
