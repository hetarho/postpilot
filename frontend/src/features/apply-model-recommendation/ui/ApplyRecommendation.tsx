import type { RecommendationSet } from '@/entities/model-catalog'
import { useApplyRecommendation } from '@/entities/model-catalog'
import { Button, FieldMessage } from '@/shared/ui'

export function ApplyRecommendation({ recommendation }: { recommendation: RecommendationSet }) {
  const mutation = useApplyRecommendation()
  return (
    <div className="bg-surface-raised rounded-lg p-4">
      <p className="text-sm font-medium">{recommendation.label}</p>
      <p className="text-content-tertiary mt-1 text-xs">
        {recommendation.id} · 활성 모델과 세 단계 A/B 쌍 9개를 한 번에 저장합니다.
      </p>
      <Button
        variant="secondary"
        className="mt-4"
        disabled={mutation.isPending}
        onClick={() => void mutation.apply(recommendation.id)}
      >
        {mutation.isPending ? '적용 중…' : '추천 조합 적용'}
      </Button>
      {mutation.isError && (
        <FieldMessage className="mt-2">추천 조합을 적용하지 못했어요.</FieldMessage>
      )}
    </div>
  )
}
