import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import {
  type ComparisonPair,
  type ModelRef,
  sameRef,
  useComparisonPairSavePending,
  useModelSetup,
  useStageSelection,
} from '@/entities/model-catalog'
import { useStartModelExperiment, useStartWriteExperiment } from '@/entities/model-experiment'
import { displayTitle, isPublished, usePost, usePosts } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { ModelPairForm } from '@/features/configure-model-pair'
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
import { useModelStage } from '../model/useModelStage'
import { ModelPageHeader } from './ModelPageHeader'
import { ModelStageTabs } from './ModelStageTabs'

export function ModelComparisonPage() {
  const { t } = useTranslation(['models', 'common'])
  const { stage } = useModelStage()
  const [postSlug, setPostSlug] = useState('')
  const [chosenVoiceId, setChosenVoiceId] = useState('')
  const startHintId = useId()
  const setup = useModelSetup()
  const pairSaving = useComparisonPairSavePending()
  const { posts } = usePosts()
  const { user } = useSession()
  const { active: activeVoices, defaultVoice } = useVoices(user?.id ?? '')
  const start = useStartModelExperiment()
  const navigate = useNavigate()
  const pair = setup.pairs.find((item) => item.stage === stage)
  // An analyze comparison freezes ONE voice's corpus, so the voice is chosen here and sent
  // explicitly — initialized to the default, never guessed by the server
  // (spec/legacy/policy/model-experiments.md). A choice that has since been deleted falls back to the
  // default rather than to a request the server would refuse.
  const voiceId =
    (activeVoices.some((voice) => voice.id === chosenVoiceId) ? chosenVoiceId : '') ||
    defaultVoice?.id ||
    ''
  // What the CTA is still waiting for, in the user's words. `pair` comes from the server, so
  // choosing A and B in the form above is not enough — the combination has to have been SAVED,
  // and a greyed button two screens down cannot say that on its own (§4.3).
  const unmet = [
    !pair?.candidateA || !pair.candidateB ? t('page.requirement.pair', { ns: 'models' }) : '',
    stage === 'observe' && !postSlug ? t('page.requirement.photoPost', { ns: 'models' }) : '',
    stage === 'analyze' && !voiceId ? t('page.requirement.voice', { ns: 'models' }) : '',
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
    const response =
      stage === 'observe'
        ? await start.startObserve(postSlug, pair.candidateA.ref, pair.candidateB.ref)
        : await start.startAnalyze(voiceId, pair.candidateA.ref, pair.candidateB.ref)
    void navigate({
      to: '/ai-models/experiments/$id',
      params: { id: response.experimentId },
      search: { stage, from: 'compare' },
    })
  }
  return (
    <main className={pageStyles({ width: 'board', className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="comparison" description="comparisonDescription" />
      <ModelStageTabs to="/ai-models/compare" />
      <section className="mt-6" aria-label={t('page.pairSettings', { ns: 'models' })}>
        <div className="mt-6">
          {/* Keyed by stage: the form's save mutations live inside the feature, and a '저장했어요'
              or a save error belongs to the tab it was fired from, not to the next one. */}
          <ModelPairForm key={stage} stage={stage} />
        </div>
        {stage === 'analyze' && (
          <div className="mt-6">
            <FieldLabel id="experiment-voice-label" htmlFor="experiment-voice">
              {t('page.voice', { ns: 'models' })}
            </FieldLabel>
            <Listbox
              id="experiment-voice"
              aria-labelledby="experiment-voice-label"
              className="mt-1"
              value={voiceId}
              options={[
                ...(voiceId ? [] : [{ value: '', label: t('page.selectVoice', { ns: 'models' }) }]),
                ...activeVoices.map((voice) => ({ value: voice.id, label: voice.name })),
              ]}
              onChange={setChosenVoiceId}
            />
            <Typography variant="label" as="p" className="mt-2">
              {t('page.voiceHelp', { ns: 'models' })}
            </Typography>
          </div>
        )}
        {stage !== 'analyze' && (
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
              onChange={setPostSlug}
            />
          </div>
        )}
        {stage === 'write' ? (
          <WriteComparisonStart
            postSlug={postSlug}
            pair={pair}
            pairPending={setup.isPending}
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
              onClick={() => void startComparison()}
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
      </section>
    </main>
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
        published: isPublished(post),
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

  // The model lab is the write comparison's second entry point (change 06), so it goes through
  // the same picker with the same reuse contract. `usePost` already holds the observations.
  const [picking, setPicking] = useState(false)

  const enqueue = async (reobserveFiles?: readonly string[]) => {
    if (!canStart || !post || !writeA || !writeB) return
    try {
      const response = await start.start(
        post.slug,
        'lab',
        post.images.length ? observeSelection?.ref : undefined,
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
    if (needsPicker(post.images, post.observations)) {
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

function resolveSelection(
  models: ReturnType<typeof useStageSelection>['models'],
  ref: ModelRef | null | undefined,
): GenerationModelSelection | undefined {
  if (!ref) return undefined
  const model = models.find((candidate) => sameRef(candidate.ref, ref))
  return model && !model.disabled ? { ref, vision: model.vision } : undefined
}
