import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useBulkReviewGuidelineCandidates,
  type BulkReviewOutcome,
  type GuidelineCandidate,
} from '@/entities/guideline'
import { Button, Dialog, Typography } from '@/shared/ui'

/** 전부 수락 / 전부 거절 for the whole queue the user is looking at (GUIDE-27).
 *
 *  Acceptance saves everything that passes as a 전역 guideline and leaves each refusal — over 300
 *  characters, a duplicate, the account cap — where it was, with its reason on its own row, so a
 *  single 승인 can fix that one. Nothing is rolled back: the rules that saved are rules the user
 *  asked for.
 *
 *  Dismissal asks once, because there is no undo for fifty rows at a time. A single 무시 stays
 *  dialog-free (GUIDE-12) — one row is a small, deliberate act, and fifty is not. */
export function BulkGuidelineCandidateActions({
  ownerId,
  candidates,
  onFailures,
  onFinished,
}: {
  ownerId: string
  candidates: readonly GuidelineCandidate[]
  /** The refusals belong to the rows, which the page renders, so they are reported upward. */
  onFailures: (failures: BulkReviewOutcome['failures']) => void
  /** So is the run's own report: an accepted queue empties, and these buttons go with it, while
   *  what the run did still has to be readable. */
  onFinished: (outcome: { kind: 'approve' | 'dismiss'; moved: number; attempted: number }) => void
}) {
  const { t } = useTranslation('guidelines')
  const bulk = useBulkReviewGuidelineCandidates(ownerId)
  const [confirming, setConfirming] = useState(false)

  if (candidates.length === 0) return null

  const run = async (kind: 'approve' | 'dismiss') => {
    const attempted = candidates.length
    onFailures([])
    const result =
      kind === 'approve' ? await bulk.approveAll(candidates) : await bulk.dismissAll(candidates)
    onFailures(result.failures)
    onFinished({ kind, moved: result.moved, attempted })
  }

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      <Button
        variant="secondary"
        disabled={bulk.isPending}
        pending={bulk.running === 'approve'}
        onClick={() => void run('approve')}
      >
        {t('candidate.approveAll')}
      </Button>
      <Button
        variant="ghost"
        disabled={bulk.isPending}
        pending={bulk.running === 'dismiss'}
        onClick={() => setConfirming(true)}
      >
        {t('candidate.dismissAll')}
      </Button>
      <Dialog
        open={confirming}
        title={t('candidate.dismissAllTitle', { count: candidates.length })}
        confirmLabel={t('candidate.dismissAll')}
        pending={bulk.running === 'dismiss'}
        onClose={() => setConfirming(false)}
        onConfirm={() => {
          setConfirming(false)
          void run('dismiss')
        }}
      >
        <Typography variant="body" className="text-content-secondary">
          {t('candidate.dismissAllDescription')}
        </Typography>
      </Dialog>
    </div>
  )
}
