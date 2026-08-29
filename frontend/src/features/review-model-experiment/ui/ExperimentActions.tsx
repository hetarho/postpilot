import { useCallback, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { type QueryKey, useQueryClient } from '@tanstack/react-query'
import type { ModelExperiment } from '@/entities/model-experiment'
import { needsExperimentReview, useExperimentActions } from '@/entities/model-experiment'
import { getPostQueryKey, listPostsQueryKey } from '@/entities/post'
import { getSelectionsQueryKey } from '@/entities/model-catalog'
import { Button, Dialog, Notice } from '@/shared/ui'
import { hasExperimentActions } from '../model/experiment-actions'

export function ExperimentActions({
  experiment,
  activeCandidateId,
}: {
  experiment: ModelExperiment
  activeCandidateId: string
}) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const refreshOwner = useCallback(async () => {
    const keys: QueryKey[] = [listPostsQueryKey(transport), getSelectionsQueryKey(transport)]
    if (experiment.postSlug) keys.push(getPostQueryKey(transport, experiment.postSlug))
    await Promise.all(keys.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
  }, [experiment.postSlug, queryClient, transport])
  const actions = useExperimentActions(experiment.id, refreshOwner)
  const [confirmStyle, setConfirmStyle] = useState(false)
  // `useExperimentActions` reports one `isPending` for all six mutations, so the button the thumb
  // actually pressed has to be remembered here — otherwise the whole bar spins at once. Without a
  // pending state at all, a tap on a cellular connection only dropped the button to 50% opacity,
  // which is not feedback on a device with no hover, and the user fired the mutation twice (§6).
  const [pressed, setPressed] = useState('')
  const run = (name: string, action: () => Promise<unknown>) => {
    setPressed(name)
    void action().finally(() => setPressed(''))
  }
  const selected = experiment.candidates.find((candidate) => candidate.id === activeCandidateId)
  const survivor = experiment.status === 'partial' && selected?.status === 'succeeded'
  const canChoose = experiment.status === 'review' && selected?.status === 'succeeded'
  if (!hasExperimentActions(experiment)) return null
  return (
    <div className="grid gap-3">
      {/* Stacked full-width targets on a phone: three Korean labels measure ~410px against the
          296px the bar has at 360px, so a wrapping row became two right-aligned rows of ambiguous
          targets 8px apart, 128px tall over the reading area (§4.1). From `sm:` up they collapse
          back into the desktop row. The CTA is the last child in every status (§4). */}
      <div className="grid gap-3 sm:flex sm:flex-wrap sm:justify-end">
        {(experiment.status === 'partial' || experiment.status === 'failed') && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'retry'}
            onClick={() => run('retry', actions.retry)}
          >
            실패 후보 재시도
          </Button>
        )}
        {needsExperimentReview(experiment.status) && (
          <Button
            variant="ghost"
            disabled={actions.isPending}
            pending={pressed === 'dismiss'}
            onClick={() => run('dismiss', actions.dismiss)}
          >
            둘 다 사용하지 않기
          </Button>
        )}
        {experiment.applyError && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'apply'}
            onClick={() =>
              run('apply', () =>
                experiment.stage === 'write'
                  ? actions.decideWrite(experiment.winnerCandidateId, experiment.adoptionRequested)
                  : actions.apply(experiment.stage === 'analyze'),
              )
            }
          >
            적용 다시 시도
          </Button>
        )}
        {experiment.status === 'decided' && experiment.stage !== 'write' && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'adopt'}
            onClick={() => run('adopt', actions.adopt)}
          >
            활성 모델로 사용
          </Button>
        )}
        {experiment.adoptionError && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'adopt'}
            onClick={() =>
              run('adopt', () => actions.decideWrite(experiment.winnerCandidateId, true))
            }
          >
            활성 모델 변경 다시 시도
          </Button>
        )}
        {survivor && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            pending={pressed === 'useSingle'}
            onClick={() => run('useSingle', () => actions.useSingle(activeCandidateId))}
          >
            이 결과만 사용
          </Button>
        )}
        {canChoose && experiment.stage !== 'write' && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            pending={pressed === 'choose'}
            onClick={() => run('choose', () => actions.choose(activeCandidateId))}
          >
            이 결과로 선택
          </Button>
        )}
        {canChoose && experiment.stage === 'write' && (
          <>
            <Button
              variant="secondary"
              disabled={actions.isPending}
              pending={pressed === 'decide'}
              onClick={() => run('decide', () => actions.decideWrite(activeCandidateId, false))}
            >
              결과 적용
            </Button>
            <Button
              variant="cta"
              disabled={actions.isPending}
              pending={pressed === 'decideAdopt'}
              onClick={() => run('decideAdopt', () => actions.decideWrite(activeCandidateId, true))}
            >
              결과 적용하고 활성 모델로 변경
            </Button>
          </>
        )}
        {experiment.status === 'decided' &&
          experiment.stage !== 'write' &&
          !experiment.appliedAt &&
          !experiment.applyError && (
            <Button
              variant="cta"
              disabled={actions.isPending}
              pending={pressed === 'apply'}
              onClick={() =>
                experiment.stage === 'analyze'
                  ? setConfirmStyle(true)
                  : run('apply', () => actions.apply())
              }
            >
              결과 적용
            </Button>
          )}
      </div>
      {/* The outcome renders inside the dock, right under the button that was pressed — a result
          reported 1,000px up the page has not been shown (§4.3). */}
      {actions.error && (
        <Notice tone="danger" role="alert">
          요청을 처리하지 못했어요.
        </Notice>
      )}
      {experiment.applyError && (
        <Notice tone="danger" role="alert">
          적용하지 못했어요. {experiment.applyError}
        </Notice>
      )}
      {experiment.appliedAt && !experiment.applyError && (
        <Notice tone="success" role="status">
          선택한 결과를 적용했어요.
          {experiment.stage === 'write' &&
            (experiment.adoptedAt
              ? ' 활성 작성 모델도 변경했어요.'
              : ' 활성 작성 모델은 변경하지 않았어요.')}
        </Notice>
      )}
      {experiment.adoptionError && (
        <Notice tone="danger" role="alert">
          결과는 적용했지만 활성 작성 모델은 변경하지 못했어요. {experiment.adoptionError}
        </Notice>
      )}
      <Dialog
        open={confirmStyle}
        title="문체 분석 결과를 적용할까요?"
        confirmLabel="문체 덮어쓰기"
        pending={actions.isPending}
        onClose={() => setConfirmStyle(false)}
        onConfirm={() => {
          void actions.apply(true).then(() => setConfirmStyle(false))
        }}
      >
        현재 styleguide를 선택한 결과로 교체합니다. 직접 작성한 rules는 그대로 유지됩니다.
      </Dialog>
    </div>
  )
}
