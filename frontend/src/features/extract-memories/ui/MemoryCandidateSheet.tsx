import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useApproveMemoryCandidates,
  useMemoryExtraction,
  type CandidateSaveOutcome,
} from '@/entities/memory'
import {
  Badge,
  Button,
  Checkbox,
  FieldMessage,
  Sheet,
  Typography,
  typographyStyles,
} from '@/shared/ui'

/** What the extraction proposed, as a checkbox list the user confirms (POST-72, MEM-15).
 *
 *  Everything is checked to begin with: the list is what a model thought was worth keeping, and
 *  the user's work is removing what is wrong rather than picking what is right. Saving creates
 *  ONLY the checked rows, each through the ordinary create — which is where every field rule and
 *  the account cap live. */
export function MemoryCandidateSheet({
  ownerId,
  jobId,
  onClose,
}: {
  ownerId: string
  jobId: string
  onClose: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  const titleId = `memory-candidates-${jobId}`
  const extraction = useMemoryExtraction(jobId, true)
  const approve = useApproveMemoryCandidates(ownerId)
  // What the user has UNCHECKED, so the default — everything checked — needs no effect to seed
  // it when the candidates arrive. The list is what a model thought was worth keeping, and the
  // user's work is removing what is wrong rather than picking what is right.
  const [unchecked, setUnchecked] = useState<Set<number>>(new Set())
  const [outcome, setOutcome] = useState<CandidateSaveOutcome | null>(null)
  const isChecked = (index: number) => !unchecked.has(index)
  const checkedCount = extraction.candidates.filter((_, index) => isChecked(index)).length

  const save = async () => {
    const selected = extraction.candidates
      .map((candidate, index) => ({ index, ...candidate }))
      .filter((candidate) => isChecked(candidate.index))
    if (selected.length === 0) return
    const result = await approve.approve(extraction.postSlug, selected)
    setOutcome(result)
    // A clean run is done: the sheet's whole purpose is finished. A partial one stays open with
    // its reasons, and the rows that saved are unchecked so a retry sends only what failed.
    if (result.failures.length === 0) {
      onClose()
      return
    }
    // Only the refused rows stay checked, so a retry sends exactly what failed.
    const refused = new Set(result.failures.map((failure) => failure.index))
    setUnchecked(
      new Set(
        extraction.candidates.map((_, index) => index).filter((index) => !refused.has(index)),
      ),
    )
  }

  const toggle = (index: number) => {
    setUnchecked((current) => {
      const next = new Set(current)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  return (
    <Sheet open labelledBy={titleId} onClose={approve.isPending ? () => {} : onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('memories.candidateTitle', { ns: 'posts' })}
      </Typography>
      <Typography variant="body" as="p" className="text-content-secondary mt-2">
        {t('memories.candidateHelp', { ns: 'posts' })}
      </Typography>

      {extraction.isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-4">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}
      {extraction.isError && (
        <FieldMessage className="mt-4">{extraction.errorMessage}</FieldMessage>
      )}

      {!extraction.isPending && !extraction.isError && extraction.candidates.length === 0 && (
        // Repeatable on demand, so this says so instead of pretending something went wrong.
        <Typography variant="body" as="p" role="status" className="text-content-secondary mt-4">
          {t('memories.candidateEmpty', { ns: 'posts' })}
        </Typography>
      )}

      {extraction.candidates.length > 0 && (
        <ul className="divide-divider mt-4 divide-y">
          {extraction.candidates.map((candidate, index) => (
            <li key={`${candidate.text}-${index}`} className="py-3">
              <label
                className={typographyStyles({
                  variant: 'body',
                  className: 'flex min-h-11 items-start gap-3',
                })}
              >
                <span className="mt-1">
                  <Checkbox
                    checked={isChecked(index)}
                    disabled={approve.isPending}
                    onChange={() => toggle(index)}
                  />
                </span>
                <span className="min-w-0">
                  <span className="block break-words">{candidate.text}</span>
                  <span className="mt-2 flex flex-wrap items-center gap-2">
                    <Badge tone="accent">{t(`kind.${candidate.kind}`, { ns: 'memories' })}</Badge>
                    {candidate.tags.map((tag) => (
                      <Badge key={tag}>{tag}</Badge>
                    ))}
                  </span>
                </span>
              </label>
              {outcome?.failures.find((failure) => failure.index === index) && (
                <FieldMessage className="mt-1">
                  {outcome.failures.find((failure) => failure.index === index)?.message}
                </FieldMessage>
              )}
            </li>
          ))}
        </ul>
      )}

      <div className="mt-6 flex flex-wrap justify-end gap-2">
        <Button variant="ghost" disabled={approve.isPending} onClick={onClose}>
          {t('action.close', { ns: 'common' })}
        </Button>
        {extraction.candidates.length > 0 && (
          <Button
            variant="cta"
            disabled={checkedCount === 0 || approve.isPending}
            pending={approve.isPending}
            onClick={() => void save()}
          >
            {t('memories.candidateSave', { ns: 'posts', count: checkedCount })}
          </Button>
        )}
      </div>
    </Sheet>
  )
}
