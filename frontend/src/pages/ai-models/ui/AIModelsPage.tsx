import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  type ComparisonPair,
  type ModelRef,
  stageLabel,
  type StageName,
  sameRef,
  useComparisonPairSavePending,
  useModelSetup,
  useStageSelection,
} from '@/entities/model-catalog'
import {
  type ExperimentStatusName,
  useExperiments,
  useLeaderboard,
} from '@/entities/model-experiment'
import { displayTitle, usePost, usePosts } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useVoices, voiceRefLabel } from '@/entities/voice'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { ModelPairForm } from '@/features/configure-model-pair'
import {
  comparisonGenerationPreconditions,
  type GenerationModelSelection,
  useStartWriteExperiment,
} from '@/features/generate-post'
import { PostCreditEstimate } from '@/features/select-model'
import { useStartModelExperiment } from '@/features/start-model-experiment'
import {
  Badge,
  type BadgeTone,
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  Listbox,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { ModelLeaderboard } from '@/widgets/model-leaderboard'

export function AIModelsPage() {
  const { t } = useTranslation(['models', 'common'])
  const [stage, setStage] = useState<StageName>('observe')
  const [postSlug, setPostSlug] = useState('')
  const [chosenVoiceId, setChosenVoiceId] = useState('')
  const startHintId = useId()
  const setup = useModelSetup()
  const pairSaving = useComparisonPairSavePending()
  const { posts } = usePosts()
  const { user } = useSession()
  const { voices, active: activeVoices, defaultVoice } = useVoices(user?.id ?? '')
  const { experiments } = useExperiments(stage)
  const { entries } = useLeaderboard(stage)
  const start = useStartModelExperiment()
  const navigate = useNavigate()
  const stageOptions = [
    { value: 'observe' as const, label: t('stage.observe', { ns: 'models' }) },
    { value: 'analyze' as const, label: t('stage.analyze', { ns: 'models' }) },
    { value: 'write' as const, label: t('stage.write', { ns: 'models' }) },
  ]
  const pair = setup.pairs.find((item) => item.stage === stage)
  // An analyze comparison freezes ONE voice's corpus, so the voice is chosen here and sent
  // explicitly — initialized to the default, never guessed by the server
  // (spec/policy/model-experiments.md). A choice that has since been deleted falls back to the
  // default rather than to a request the server would refuse.
  const voiceId =
    (activeVoices.some((voice) => voice.id === chosenVoiceId) ? chosenVoiceId : '') ||
    defaultVoice?.id ||
    ''
  const voiceName = (id: string) => {
    const voice = voices.find((candidate) => candidate.id === id)
    return voice ? voiceRefLabel(voice) : ''
  }
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
    void navigate({ to: '/ai-models/experiments/$id', params: { id: response.experimentId } })
  }
  return (
    <main className="mx-auto w-full max-w-4xl px-4 py-8 sm:px-6">
      <Typography variant="display">{t('title', { ns: 'models' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.description', { ns: 'models' })}
      </Typography>

      {/* Placed with the model choice rather than in the header: what a pair costs is the
          consequence of the decision this page exists to make. */}
      <PostCreditEstimate className="mt-6" />

      <section className="mt-10" aria-labelledby="recommendation-heading">
        <Typography variant="title" id="recommendation-heading">
          {t('page.recommendation', { ns: 'models' })}
        </Typography>
        <div className="mt-4">
          {setup.recommendations[0] ? (
            <ApplyRecommendation recommendation={setup.recommendations[0]} />
          ) : (
            <Typography variant="body" className="text-content-tertiary">
              {t('page.recommendationLoading', { ns: 'models' })}
            </Typography>
          )}
        </div>
      </section>

      <section className="mt-12" aria-labelledby="settings-heading">
        <Typography variant="title" id="settings-heading">
          {t('page.settings', { ns: 'models' })}
        </Typography>
        {/* The switch drives four sections, the last of them ~1,000px below it, so it follows the
            scroll instead of existing only at the top of the section: otherwise comparing two
            stages' leaderboards is a ~1,000px round trip each way (§4.3). It carries the page's own
            plane out to the gutters so the content scrolling underneath is covered, and clears the
            desktop header, which is sticky and 64px tall from `sm:` up. */}
        <div className="bg-surface-base sticky top-0 z-10 -mx-4 mt-4 px-4 py-2 sm:top-16 sm:-mx-6 sm:px-6">
          <SegmentedControl
            value={stage}
            options={stageOptions}
            onChange={setStage}
            ariaLabel={t('page.stageAria', { ns: 'models' })}
          />
        </div>
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

      <section className="mt-12" aria-labelledby="recent-heading">
        <Typography variant="title" id="recent-heading">
          {t('page.recent', { ns: 'models', stage: stageLabel(stage) })}
        </Typography>
        {experiments.length === 0 ? (
          <Typography variant="body" className="text-content-tertiary mt-4">
            {t('page.noComparison', { ns: 'models' })}
          </Typography>
        ) : (
          // Full-bleed rows: the negative gutter puts the row's text edge on the same line as the
          // section headings and lets its pressed plane run to the screen edge (§4.2).
          <ul className="divide-divider -mx-4 mt-4 divide-y sm:-mx-6">
            {experiments.slice(0, 8).map((item) => (
              <li key={item.id}>
                <Link
                  to="/ai-models/experiments/$id"
                  params={{ id: item.id }}
                  className={typographyStyles({
                    variant: 'label',
                    className:
                      'text-content-primary hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center justify-between gap-3 px-4 py-3 sm:px-6',
                  })}
                >
                  {/* `min-w-0` is what makes `truncate` work: a slug is `YYYYMMDD-` plus up to 60
                      runes of the title, so a spaceless Korean one is ~420px of max-content in a
                      312px row and would otherwise crush the status chip to a column of single
                      syllables (§8.5). */}
                  <span className="min-w-0 truncate">
                    {item.postSlug || voiceName(item.voiceId) || stageLabel(item.stage)}
                  </span>
                  <Badge tone={STATUS_TONES[item.status]}>
                    {t(`experimentStatus.${item.status}`, { ns: 'models' })}
                  </Badge>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="mt-12" aria-labelledby="leaderboard-heading">
        <Typography variant="title" id="leaderboard-heading">
          {t('page.myLeaderboard', { ns: 'models', stage: stageLabel(stage) })}
        </Typography>
        <div className="mt-4">
          <ModelLeaderboard entries={entries} />
        </div>
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
    ? comparisonGenerationPreconditions(
        post.images,
        observeSelection,
        writeA,
        writeB,
        post.activeJob,
        post.voice,
      )
    : undefined
  const modelPending =
    pairPending || write.isPending || (Boolean(post?.images.length) && observe.isPending)
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

  const startComparison = async () => {
    if (!canStart || !post || !writeA || !writeB) return
    try {
      const response = await start.start(
        post.slug,
        post.images.length ? observeSelection?.ref : undefined,
        writeA.ref,
        writeB.ref,
        post.targetLength,
      )
      void navigate({ to: '/ai-models/experiments/$id', params: { id: response.experimentId } })
    } catch {
      // The mutation's transport error is rendered beside the action.
    }
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

/** The row's status chip. The tone reinforces the label and never replaces it, so nothing is
 *  carried by colour alone (§2.6). */
const STATUS_TONES: Record<ExperimentStatusName, BadgeTone> = {
  queued: 'neutral',
  running: 'info',
  review: 'info',
  partial: 'warning',
  failed: 'danger',
  decided: 'success',
  dismissed: 'neutral',
}
