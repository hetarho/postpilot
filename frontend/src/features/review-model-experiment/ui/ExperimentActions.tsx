import { useCallback, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { type QueryKey, useQueryClient } from '@tanstack/react-query'
import type { ModelExperiment } from '@/entities/model-experiment'
import { useExperimentActions } from '@/entities/model-experiment'
import { getPostQueryKey, listPostsQueryKey } from '@/entities/post'
import { getSelectionsQueryKey } from '@/entities/model-catalog'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

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
  const selected = experiment.candidates.find((candidate) => candidate.id === activeCandidateId)
  const survivor = experiment.status === 'partial' && selected?.status === 'succeeded'
  const canChoose = experiment.status === 'review' && selected?.status === 'succeeded'
  return (
    <div className="bg-surface-highest sticky bottom-0 mt-6 rounded-xl p-4 shadow-lg">
      <div className="flex flex-wrap justify-end gap-2">
        {(experiment.status === 'partial' || experiment.status === 'failed') && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            onClick={() => void actions.retry()}
          >
            실패 후보 재시도
          </Button>
        )}
        {(experiment.status === 'review' ||
          experiment.status === 'partial' ||
          experiment.status === 'failed') && (
          <Button
            variant="ghost"
            disabled={actions.isPending}
            onClick={() => void actions.dismiss()}
          >
            둘 다 사용하지 않기
          </Button>
        )}
        {survivor && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            onClick={() => void actions.useSingle(activeCandidateId)}
          >
            이 결과만 사용
          </Button>
        )}
        {canChoose && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            onClick={() => void actions.choose(activeCandidateId)}
          >
            이 결과로 선택
          </Button>
        )}
        {experiment.status === 'decided' && !experiment.appliedAt && !experiment.applyError && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            onClick={() =>
              experiment.stage === 'analyze' ? setConfirmStyle(true) : void actions.apply()
            }
          >
            결과 적용
          </Button>
        )}
        {experiment.status === 'decided' && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            onClick={() => void actions.adopt()}
          >
            활성 모델로 사용
          </Button>
        )}
        {experiment.applyError && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            onClick={() => void actions.apply(experiment.stage === 'analyze')}
          >
            적용 다시 시도
          </Button>
        )}
      </div>
      {actions.error && <FieldMessage className="mt-2">요청을 처리하지 못했어요.</FieldMessage>}
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
