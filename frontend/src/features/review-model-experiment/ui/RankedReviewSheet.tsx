import type { ModelExperiment, VerdictBadgeName } from '@/entities/model-experiment'
import type { AppFailure } from '@/shared/api'
import { LegacyReviewSheet } from './LegacyReviewSheet'

export interface RankedReviewAnswer {
  candidateId: string
  rank: number
  badges: VerdictBadgeName[]
  otherNote: string
}

export function RankedReviewSheet(props: {
  experiment: ModelExperiment
  open: boolean
  pending: boolean
  failure?: AppFailure
  onConfirm: (ranks: RankedReviewAnswer[]) => Promise<unknown>
  onClose: () => void
}) {
  return <LegacyReviewSheet open={props.open} onClose={props.onClose} />
}
