import { useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useExperiments, useLeaderboard } from '@/entities/model-experiment'
import { STAGE_LABELS, type StageName, useModelSetup } from '@/entities/model-catalog'
import { displayTitle, usePosts } from '@/entities/post'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { ModelPairForm } from '@/features/configure-model-pair'
import { useStartModelExperiment } from '@/features/start-model-experiment'
import { Button, FieldLabel, FieldMessage, SegmentedControl, Select } from '@/shared/ui'
import { ModelLeaderboard } from '@/widgets/model-leaderboard'

const STAGE_OPTIONS = [
  { value: 'observe' as const, label: '관찰' },
  { value: 'analyze' as const, label: '문체 분석' },
  { value: 'write' as const, label: '글 작성' },
]

export function AIModelsPage() {
  const [stage, setStage] = useState<StageName>('observe')
  const [postSlug, setPostSlug] = useState('')
  const setup = useModelSetup()
  const { posts } = usePosts()
  const { experiments } = useExperiments(stage)
  const { entries } = useLeaderboard(stage)
  const start = useStartModelExperiment()
  const navigate = useNavigate()
  const pair = setup.pairs.find((item) => item.stage === stage)
  const canStart = pair?.candidateA && pair.candidateB && (stage !== 'observe' || postSlug)
  const startComparison = async () => {
    if (!pair?.candidateA || !pair.candidateB || stage === 'write') return
    const response =
      stage === 'observe'
        ? await start.startObserve(postSlug, pair.candidateA.ref, pair.candidateB.ref)
        : await start.startAnalyze(pair.candidateA.ref, pair.candidateB.ref)
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
        <SegmentedControl
          className="mt-4"
          value={stage}
          options={STAGE_OPTIONS}
          onChange={setStage}
          ariaLabel="AI 단계"
        />
        <div className="mt-6">
          <ModelPairForm stage={stage} />
        </div>
        {stage === 'observe' && (
          <div className="mt-6">
            <FieldLabel htmlFor="experiment-post">사진이 있는 글</FieldLabel>
            <Select
              id="experiment-post"
              className="mt-1"
              value={postSlug}
              onChange={(event) => setPostSlug(event.target.value)}
            >
              <option value="">글을 선택하세요</option>
              {posts.map((post) => (
                <option key={post.slug} value={post.slug}>
                  {displayTitle(post)}
                </option>
              ))}
            </Select>
          </div>
        )}
        {stage === 'write' ? (
          <p className="bg-notice-info-bg text-notice-info-fg mt-6 rounded-md px-3 py-2 text-sm">
            작성 비교는 글 편집기의 `AI 생성`을 누를 때 자동으로 같은 관찰 결과를 사용해 시작됩니다.
          </p>
        ) : (
          <div className="mt-6">
            <Button
              variant="cta"
              disabled={!canStart || start.isPending}
              onClick={() => void startComparison()}
            >
              {start.isPending ? '비교 시작 중…' : '비교 시작'}
            </Button>
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
          <ul className="divide-divider mt-4 divide-y">
            {experiments.slice(0, 8).map((item) => (
              <li key={item.id}>
                <Link
                  to="/ai-models/experiments/$id"
                  params={{ id: item.id }}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center justify-between px-2 py-3 text-sm"
                >
                  <span>{item.postSlug || STAGE_LABELS[item.stage]}</span>
                  <span className="text-content-tertiary">{statusLabel(item.status)}</span>
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

function statusLabel(status: string): string {
  return (
    (
      {
        queued: '대기',
        running: '진행 중',
        review: '결과 확인',
        partial: '일부 오류',
        failed: '오류',
        decided: '선택 완료',
        dismissed: '사용 안 함',
      } as Record<string, string>
    )[status] ?? status
  )
}
