import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import {
  type ComparisonPair,
  type ModelRef,
  refKey,
  sameRef,
  useComparisonPairSavePending,
  useModelSetup,
  useStageSelection,
} from '@/entities/model-catalog'
import { useStartModelExperiment, useStartWriteExperiment } from '@/entities/model-experiment'
import { displayTitle, usePost, usePosts } from '@/entities/post'
import { ModelPairForm, type ShownPair } from '@/features/configure-model-pair'
import {
  comparisonGenerationPreconditions,
  needsPicker,
  ReobservePicker,
  type GenerationModelSelection,
} from '@/features/generate-post'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  pageStyles,
} from '@/shared/ui'
import { stageOfTab, useModelStage } from '../model/useModelStage'
import { ModelPageHeader } from './ModelPageHeader'
import { ModelStageTabs } from './ModelStageTabs'
import { VoiceReflectionStart } from './VoiceReflectionStart'

export function ModelComparisonPage() {
  const { t } = useTranslation(['models', 'common'])
  const { stage: tab } = useModelStage()
  // 말투 반영 compares the write pair: its comparisons are write comparisons (MODEL-67).
  const stage = stageOfTab(tab)
  const [postSlug, setPostSlug] = useState('')
  const startHintId = useId()
  const setup = useModelSetup()
  const pairSaving = useComparisonPairSavePending()
  const { posts } = usePosts()
  const start = useStartModelExperiment()
  const navigate = useNavigate()
  const stored = setup.pairs.find((item) => item.stage === stage)
  // The gate reads the pair the screen shows (MODEL-65): while the fields hold a change that
  // wrote nothing — incomplete, or one model twice — the stored pair is not what the owner
  // sees, so nothing may start against it.
  const [shown, setShown] = useState<ShownPair>()
  const pair = shownIsStored(shown, stage, stored) ? stored : undefined
  // What the CTA is still waiting for, in the user's words. `pair` comes from the server, so
  // choosing A and B in the form above is not enough — the combination has to have been SAVED,
  // and a greyed button two screens down cannot say that on its own (THEME-24).
  const unmet = [
    !pair?.candidateA || !pair.candidateB ? t('page.requirement.pair', { ns: 'models' }) : '',
    stage === 'observe' && !postSlug ? t('page.requirement.photoPost', { ns: 'models' }) : '',
  ].filter(Boolean)
  const canStart = !pairSaving && unmet.length === 0
  const startHint = pairSaving
    ? t('page.pairSaving', { ns: 'models' })
    : canStart
      ? ''
      : t('page.canStart', {
          ns: 'models',
          requirements: unmet.join(t('page.requirementSeparator', { ns: 'models' })),
        })
  const startComparison = async () => {
    // The CTA is `aria-disabled`, not `disabled`, so it keeps its place in the focus order and can
    // still be activated from a keyboard — the preconditions are enforced here, not by the browser.
    if (stage === 'write' || !canStart || start.isPending) return
    if (!pair?.candidateA || !pair.candidateB) return
    const response = await start.startObserve(postSlug, pair.candidateA.ref, pair.candidateB.ref)
    void navigate({
      to: '/ai-models/experiments/$id',
      params: { id: response.experimentId },
      search: { stage, from: 'compare' },
    })
  }
  return (
    <main className={pageStyles({ width: 'board', className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="comparison" description="comparisonDescription" />
      <ModelStageTabs to="/ai-models/compare" withVoice />
      <section className="mt-6" aria-label={t('page.pairSettings', { ns: 'models' })}>
        <div className="mt-6">
          {/* Keyed by stage: the form's save mutations live inside the feature, and a '저장했어요'
              or a save error belongs to the tab it was fired from, not to the next one. */}
          <ModelPairForm key={tab} stage={stage} onShownChange={setShown} />
        </div>
        {tab === 'voice' ? (
          <VoiceReflectionStart pair={pair} pairPending={setup.isPending} pairSaving={pairSaving} />
        ) : (
          <PostComparisonStart
            stage={stage}
            postSlug={postSlug}
            onPostChange={setPostSlug}
            posts={posts}
            pair={pair}
            pairPending={setup.isPending}
            pairSaving={pairSaving}
            canStart={canStart}
            startHint={startHint}
            startHintId={startHintId}
            start={start}
            onStart={() => void startComparison()}
          />
        )}
      </section>
    </main>
  )
}

function PostComparisonStart({
  stage,
  postSlug,
  onPostChange,
  posts,
  pair,
  pairPending,
  pairSaving,
  canStart,
  startHint,
  startHintId,
  start,
  onStart,
}: {
  stage: 'observe' | 'write'
  postSlug: string
  onPostChange: (slug: string) => void
  posts: ReturnType<typeof usePosts>['posts']
  pair: ComparisonPair | undefined
  pairPending: boolean
  pairSaving: boolean
  canStart: boolean
  startHint: string
  startHintId: string
  start: ReturnType<typeof useStartModelExperiment>
  onStart: () => void
}) {
  const { t } = useTranslation(['models', 'common'])
  return (
    <>
      <div className="mt-6">
        <FieldLabel id="experiment-post-label" htmlFor="experiment-post">
          {stage === 'observe'
            ? t('page.photoPost', { ns: 'models' })
            : t('page.comparePost', { ns: 'models' })}
        </FieldLabel>
        <Listbox
          id="experiment-post"
          aria-labelledby="experiment-post-label"
          className="mt-1"
          value={postSlug}
          options={[
            {
              value: '',
              label:
                stage === 'observe'
                  ? t('page.selectPhotoPost', { ns: 'models' })
                  : t('page.selectPost', { ns: 'models' }),
            },
            ...posts.map((post) => ({ value: post.slug, label: displayTitle(post) })),
          ]}
          onChange={onPostChange}
        />
      </div>
      {stage === 'write' ? (
        <WriteComparisonStart
          postSlug={postSlug}
          pair={pair}
          pairPending={pairPending}
          pairSaving={pairSaving}
        />
      ) : (
        <div className="mt-6">
          <Button
            variant="cta"
            className="w-full sm:w-auto"
            pending={start.isPending}
            // `aria-disabled` rather than `disabled`: a disabled button is removed from the focus
            // order, so the reason below it would never reach a screen reader. `buttonStyles`
            // dims it and blocks the pointer either way.
            aria-disabled={!canStart || undefined}
            aria-describedby={startHint ? startHintId : undefined}
            onClick={onStart}
          >
            {t('page.start', { ns: 'models' })}
          </Button>
          {startHint && (
            <Typography variant="label" as="p" id={startHintId} className="mt-2">
              {startHint}
            </Typography>
          )}
          {start.failure && (
            <Typography variant="body" as="div" role="alert" className="text-field-error mt-2">
              <AppFailureMessage failure={start.failure} />
            </Typography>
          )}
        </div>
      )}
    </>
  )
}

function WriteComparisonStart({
  postSlug,
  pair,
  pairPending,
  pairSaving,
}: {
  postSlug: string
  pair: ComparisonPair | undefined
  pairPending: boolean
  pairSaving: boolean
}) {
  const { t } = useTranslation('models')
  const hintId = useId()
  if (!postSlug) {
    return (
      <div className="mt-6">
        <Button variant="cta" className="w-full sm:w-auto" aria-disabled aria-describedby={hintId}>
          {t('page.start')}
        </Button>
        <Typography variant="label" as="p" id={hintId} className="mt-2">
          {t('page.choosePostHelp')}
        </Typography>
      </div>
    )
  }
  return (
    <SelectedPostWriteComparison
      key={postSlug}
      postSlug={postSlug}
      pair={pair}
      pairPending={pairPending}
      pairSaving={pairSaving}
      hintId={hintId}
    />
  )
}

function SelectedPostWriteComparison({
  postSlug,
  pair,
  pairPending,
  pairSaving,
  hintId,
}: {
  postSlug: string
  pair: ComparisonPair | undefined
  pairPending: boolean
  pairSaving: boolean
  hintId: string
}) {
  const { t } = useTranslation('models')
  const { post, isPending: postPending, isFetching: postFetching, failure } = usePost(postSlug)
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const start = useStartWriteExperiment()
  const navigate = useNavigate()
  const observeSelection = resolveSelection(observe.models, observe.selected)
  const writeA = resolveSelection(
    write.models,
    pair?.candidateA && !pair.candidateA.missing ? pair.candidateA.ref : undefined,
  )
  const writeB = resolveSelection(
    write.models,
    pair?.candidateB && !pair.candidateB.missing ? pair.candidateB.ref : undefined,
  )
  const precondition = post
    ? comparisonGenerationPreconditions({
        images: post.images,
        videos: post.videos,
        // The lab's write tab takes an owned post in any status (MODEL-31): a comparison run to
        // rank two models is a reading of the post, and the published lock guards writes.
        published: false,
        activeJob: post.activeJob,
        voice: post.voice,
        observe: observeSelection,
        writeA,
        writeB,
      })
    : undefined
  // A post with photos or clips observes, so the observe selection has to have answered first;
  // otherwise its reason would flash while the selection is still loading.
  const modelPending =
    pairPending ||
    write.isPending ||
    (Boolean(post?.images.length || post?.videos.length) && observe.isPending)
  const reason =
    postPending || postFetching
      ? t('page.postChecking')
      : failure || !post
        ? t('page.postLoadFailed')
        : pairSaving
          ? t('page.pairSaving')
          : modelPending
            ? t('page.modelChecking')
            : post.pendingExperimentId
              ? t('page.pendingResult')
              : precondition && !precondition.ok
                ? precondition.reason
                : ''
  const canStart = Boolean(post) && !reason && !start.isPending

  // The model lab is the write comparison's second entry point (MODEL-31), so it goes through
  // the same picker with the same reuse contract. `usePost` already holds the observations.
  const [picking, setPicking] = useState(false)

  const enqueue = async (reobserveFiles?: readonly string[]) => {
    if (!canStart || !post || !writeA || !writeB) return
    try {
      const response = await start.start(
        post.slug,
        'lab',
        // A post with photos or videos observes, as the editor's entry does (MODEL-31).
        post.images.length || post.videos.length ? observeSelection?.ref : undefined,
        writeA.ref,
        writeB.ref,
        post.targetLength,
        reobserveFiles,
      )
      void navigate({
        to: '/ai-models/experiments/$id',
        params: { id: response.experimentId },
        search: { stage: 'write', from: 'compare' },
      })
    } catch {
      // The mutation's transport error is rendered beside the action.
    }
  }

  const startComparison = async () => {
    if (!canStart || !post || !writeA || !writeB) return
    if (needsPicker(post.images, post.observations, post.videos)) {
      setPicking(true)
      return
    }
    await enqueue()
  }

  return (
    <div className="mt-6">
      <Button
        variant="cta"
        className="w-full sm:w-auto"
        pending={start.isPending}
        aria-disabled={!canStart || undefined}
        aria-describedby={reason ? hintId : undefined}
        onClick={() => void startComparison()}
      >
        {t('page.start')}
      </Button>
      <Typography variant="label" as="p" id={hintId} role="status" className="mt-2 empty:hidden">
        {reason}
      </Typography>
      {start.isError && (
        <FieldMessage className="mt-2">{start.errorMessage || t('page.startRetry')}</FieldMessage>
      )}
      {post && (
        <ReobservePicker
          open={picking}
          images={post.images}
          videos={post.videos}
          observations={post.observations}
          observeModel={observeSelection?.ref}
          pending={start.isPending}
          onConfirm={(files) => {
            setPicking(false)
            void enqueue(files)
          }}
          onCancel={() => setPicking(false)}
        />
      )}
    </div>
  )
}

function shownIsStored(
  shown: ShownPair | undefined,
  stage: string,
  stored: { candidateA?: { ref: ModelRef }; candidateB?: { ref: ModelRef } } | undefined,
) {
  if (!shown || shown.stage !== stage) return true
  const key = (ref: ModelRef | undefined) => (ref ? refKey(ref) : '')
  return shown.a === key(stored?.candidateA?.ref) && shown.b === key(stored?.candidateB?.ref)
}

function resolveSelection(
  models: ReturnType<typeof useStageSelection>['models'],
  ref: ModelRef | null | undefined,
): GenerationModelSelection | undefined {
  if (!ref) return undefined
  const model = models.find((candidate) => sameRef(candidate.ref, ref))
  // The same capabilities the editor's entry reads: a video-only post is judged on whether the
  // observe model can watch it, not on vision alone (VIDEO-11).
  return model && !model.disabled
    ? {
        ref,
        vision: model.vision,
        videoInput: model.videoInput,
        signedVideoUrl: model.signedVideoUrl,
      }
    : undefined
}
