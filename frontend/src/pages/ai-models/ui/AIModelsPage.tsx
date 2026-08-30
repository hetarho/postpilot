import { useId, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  type ComparisonPair,
  type ModelRef,
  STAGE_LABELS,
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
import { useStartModelExperiment } from '@/features/start-model-experiment'
import {
  Badge,
  type BadgeTone,
  Button,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  Select,
} from '@/shared/ui'
import { ModelLeaderboard } from '@/widgets/model-leaderboard'

const STAGE_OPTIONS = [
  { value: 'observe' as const, label: '관찰' },
  { value: 'analyze' as const, label: '문체 분석' },
  { value: 'write' as const, label: '글 작성' },
]

export function AIModelsPage() {
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
    !pair?.candidateA || !pair.candidateB ? 'A/B 조합을 저장' : '',
    stage === 'observe' && !postSlug ? '사진이 있는 글을 선택' : '',
    stage === 'analyze' && !voiceId ? '말투를 선택' : '',
  ].filter(Boolean)
  const canStart = !pairSaving && unmet.length === 0
  const startHint = pairSaving
    ? 'A/B 조합을 저장하는 중이에요.'
    : canStart
      ? ''
      : `${unmet.join('하고, ')}하면 비교를 시작할 수 있어요.`
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
      <h1 className="text-2xl font-semibold tracking-tight">AI 모델</h1>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        추천 조합을 명시적으로 적용하거나, 내 사진·문체·글에서 더 나은 모델을 직접 비교합니다.
      </p>

      <section className="mt-10" aria-labelledby="recommendation-heading">
        <h2 id="recommendation-heading" className="text-lg font-semibold tracking-tight">
          추천 조합
        </h2>
        <div className="mt-4">
          {setup.recommendations[0] ? (
            <ApplyRecommendation recommendation={setup.recommendations[0]} />
          ) : (
            <p className="text-content-tertiary text-sm">추천 조합을 불러오는 중…</p>
          )}
        </div>
      </section>

      <section className="mt-12" aria-labelledby="settings-heading">
        <h2 id="settings-heading" className="text-lg font-semibold tracking-tight">
          단계별 설정과 비교
        </h2>
        {/* The switch drives four sections, the last of them ~1,000px below it, so it follows the
            scroll instead of existing only at the top of the section: otherwise comparing two
            stages' leaderboards is a ~1,000px round trip each way (§4.3). It carries the page's own
            plane out to the gutters so the content scrolling underneath is covered, and clears the
            desktop header, which is sticky and 64px tall from `sm:` up. */}
        <div className="bg-surface-base sticky top-0 z-10 -mx-4 mt-4 px-4 py-2 sm:top-16 sm:-mx-6 sm:px-6">
          <SegmentedControl
            value={stage}
            options={STAGE_OPTIONS}
            onChange={setStage}
            ariaLabel="AI 단계"
          />
        </div>
        <div className="mt-6">
          {/* Keyed by stage: the form's save mutations live inside the feature, and a '저장했어요'
              or a save error belongs to the tab it was fired from, not to the next one. */}
          <ModelPairForm key={stage} stage={stage} />
        </div>
        {stage === 'analyze' && (
          <div className="mt-6">
            <FieldLabel htmlFor="experiment-voice">말투</FieldLabel>
            <Select
              id="experiment-voice"
              className="mt-1"
              value={voiceId}
              onChange={(event) => setChosenVoiceId(event.target.value)}
            >
              {!voiceId && <option value="">말투를 선택하세요</option>}
              {activeVoices.map((voice) => (
                <option key={voice.id} value={voice.id}>
                  {voice.name}
                </option>
              ))}
            </Select>
            <p className="text-content-secondary mt-2 text-sm">
              이 말투의 글 전체만 비교하고, 선택한 결과도 이 말투에만 적용돼요.
            </p>
          </div>
        )}
        {stage !== 'analyze' && (
          <div className="mt-6">
            <FieldLabel htmlFor="experiment-post">
              {stage === 'observe' ? '사진이 있는 글' : '비교할 글'}
            </FieldLabel>
            <Select
              id="experiment-post"
              className="mt-1"
              value={postSlug}
              onChange={(event) => setPostSlug(event.target.value)}
            >
              <option value="">
                {stage === 'observe' ? '사진이 있는 글을 선택하세요' : '글을 선택하세요'}
              </option>
              {posts.map((post) => (
                <option key={post.slug} value={post.slug}>
                  {displayTitle(post)}
                </option>
              ))}
            </Select>
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
              비교 시작
            </Button>
            {startHint && (
              <p id={startHintId} className="text-content-secondary mt-2 text-sm">
                {startHint}
              </p>
            )}
            {start.error && <FieldMessage className="mt-2">비교를 시작하지 못했어요.</FieldMessage>}
          </div>
        )}
      </section>

      <section className="mt-12" aria-labelledby="recent-heading">
        <h2 id="recent-heading" className="text-lg font-semibold tracking-tight">
          최근 {STAGE_LABELS[stage]} 비교
        </h2>
        {experiments.length === 0 ? (
          <p className="text-content-tertiary mt-4 text-sm">아직 비교가 없어요.</p>
        ) : (
          // Full-bleed rows: the negative gutter puts the row's text edge on the same line as the
          // section headings and lets its pressed plane run to the screen edge (§4.2).
          <ul className="divide-divider -mx-4 mt-4 divide-y sm:-mx-6">
            {experiments.slice(0, 8).map((item) => (
              <li key={item.id}>
                <Link
                  to="/ai-models/experiments/$id"
                  params={{ id: item.id }}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center justify-between gap-3 px-4 py-3 text-sm sm:px-6"
                >
                  {/* `min-w-0` is what makes `truncate` work: a slug is `YYYYMMDD-` plus up to 60
                      runes of the title, so a spaceless Korean one is ~420px of max-content in a
                      312px row and would otherwise crush the status chip to a column of single
                      syllables (§8.5). */}
                  <span className="min-w-0 truncate">
                    {item.postSlug || voiceName(item.voiceId) || STAGE_LABELS[item.stage]}
                  </span>
                  <Badge tone={STATUS_META[item.status].tone}>
                    {STATUS_META[item.status].label}
                  </Badge>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="mt-12" aria-labelledby="leaderboard-heading">
        <h2 id="leaderboard-heading" className="text-lg font-semibold tracking-tight">
          내 {STAGE_LABELS[stage]} 리더보드
        </h2>
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
  const hintId = useId()
  if (!postSlug) {
    return (
      <div className="mt-6">
        <Button variant="cta" className="w-full sm:w-auto" aria-disabled aria-describedby={hintId}>
          비교 시작
        </Button>
        <p id={hintId} className="text-content-secondary mt-2 text-sm">
          비교할 글을 선택하면 비교를 시작할 수 있어요.
        </p>
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
      ? '글 정보를 확인하는 중이에요.'
      : failure || !post
        ? '글 정보를 불러오지 못했어요. 글을 다시 선택해 주세요.'
        : pairSaving
          ? 'A/B 조합을 저장하는 중이에요.'
          : modelPending
            ? '모델 선택을 확인하는 중이에요.'
            : post.pendingExperimentId
              ? '먼저 대기 중인 A/B 결과를 확인해 주세요.'
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
        비교 시작
      </Button>
      <p id={hintId} role="status" className="text-content-secondary mt-2 text-sm empty:hidden">
        {reason}
      </p>
      {start.isError && (
        <FieldMessage className="mt-2">
          {start.errorMessage || '비교를 시작하지 못했어요. 다시 시도해 주세요.'}
        </FieldMessage>
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
const STATUS_META: Record<ExperimentStatusName, { label: string; tone: BadgeTone }> = {
  queued: { label: '대기', tone: 'neutral' },
  running: { label: '진행 중', tone: 'info' },
  review: { label: '결과 확인', tone: 'info' },
  partial: { label: '일부 오류', tone: 'warning' },
  failed: { label: '오류', tone: 'danger' },
  decided: { label: '선택 완료', tone: 'success' },
  dismissed: { label: '사용 안 함', tone: 'neutral' },
}
