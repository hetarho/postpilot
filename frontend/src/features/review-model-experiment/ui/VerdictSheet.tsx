import type { CandidateBadges, ModelExperiment } from '@/entities/model-experiment'
import { LegacyReviewSheet } from './LegacyReviewSheet'

export function VerdictSheet(props: {
  experiment: ModelExperiment
  chosenCandidateId: string
  title: string
  confirmLabel: string
  open: boolean
  pending: boolean
  onConfirm: (badges: CandidateBadges[]) => void
  onClose: () => void
}) {
  return <LegacyReviewSheet open={props.open} onClose={props.onClose} />
}
