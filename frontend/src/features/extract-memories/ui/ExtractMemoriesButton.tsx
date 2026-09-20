import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isTerminal, useJob } from '@/entities/generation-job'
import { useStartMemoryExtraction } from '@/entities/memory'
import { Button, FieldMessage, Typography } from '@/shared/ui'
import { MemoryCandidateSheet } from './MemoryCandidateSheet'

/** ③'s `기억으로 저장`, beside 말투 학습 (POST-72). It starts the extraction job, polls it, and
 *  opens its candidates in a sheet.
 *
 *  Nothing is stored by pressing it: the job PROPOSES and the user approves (MEM-14, MEM-15). A
 *  start refusal, a failed job or a closed sheet all leave the post exactly as it was. */
export function ExtractMemoriesButton({
  ownerId,
  postSlug,
  hasContent,
  className,
}: {
  ownerId: string
  postSlug: string
  /** Canonical content is the whole input: there is nothing to read out of a post that has none. */
  hasContent: boolean
  className?: string
}) {
  const { t } = useTranslation('posts')
  const start = useStartMemoryExtraction()
  const [jobId, setJobId] = useState('')
  const [open, setOpen] = useState(false)
  const { job } = useJob(jobId)
  const finished = Boolean(job && isTerminal(job))
  const failed = finished && job?.status === 'failed'
  const running = jobId !== '' && !finished

  const run = async () => {
    try {
      const response = await start.start(postSlug)
      setJobId(response.jobId)
      setOpen(true)
    } catch {
      // The refusal renders below — an insufficient balance is the ordinary one — and nothing
      // was started, so the post is untouched.
    }
  }

  return (
    <div className={className}>
      <Button
        variant="secondary"
        disabled={!hasContent || start.isPending || running}
        pending={start.isPending || running}
        onClick={() => void run()}
      >
        {t('memories.extract')}
      </Button>
      {/* The reason is stated in place rather than hidden behind a disabled control with no
          explanation: a post with no generated text has nothing to read. */}
      {!hasContent && (
        <Typography variant="meta" as="p" className="text-content-secondary mt-1">
          {t('memories.extractNeedsContent')}
        </Typography>
      )}
      {start.isError && <FieldMessage className="mt-1">{start.errorMessage}</FieldMessage>}
      {failed && <FieldMessage className="mt-1">{t('memories.extractFailed')}</FieldMessage>}
      {open && finished && !failed && (
        <MemoryCandidateSheet
          ownerId={ownerId}
          jobId={jobId}
          onClose={() => {
            // Unchecked candidates are discarded with the sheet. There is deliberately no
            // "later": an extraction is repeatable on demand, so a pending queue would preserve
            // exactly what the user just declined (MEM-15).
            setOpen(false)
            setJobId('')
          }}
        />
      )}
    </div>
  )
}
