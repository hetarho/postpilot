import { useParams } from '@tanstack/react-router'
import { ContextualReturn } from '@/shared/ui'
import { ExperimentReview } from './ExperimentReview'

export function WritingTestRecordPage() {
  const { id } = useParams({ from: '/authenticated/tests/records/$id' })
  return <ExperimentReview id={id} backLink={() => <ContextualReturn />} />
}
