import { useTranslation } from 'react-i18next'
import { formatDateTime } from '@/shared/lib'
import { Typography } from '@/shared/ui'
import type { ClipProjectRequest } from '../model/revision'

/** What the owner asked the AI for, kept with the project (CLIP-133).
 *
 *  History, not a control: a read-only disclosure that sits beside the
 *  observations in ① and ②, so it needs no dock action and no step of its own.
 *  Newest first, exactly as the server answered — this screen never reorders,
 *  trims or folds together what was asked. */
export function ClipRequestRecord({ requests = [] }: { requests?: readonly ClipProjectRequest[] }) {
  const { t } = useTranslation('clips')
  if (!requests.length) return null
  return (
    <details className="mt-10">
      <summary className="text-content-secondary cursor-pointer">
        {t('record.title', { count: requests.length })}
      </summary>
      <ol className="mt-3 space-y-3" aria-label={t('record.title', { count: requests.length })}>
        {requests.map((request, index) => (
          <li key={`${request.createdAt}-${index}`} className="space-y-1">
            <Typography variant="meta" as="p">
              {t(`record.kind.${request.kind === 'instruction' ? 'instruction' : 'revision'}`, {
                target: t(
                  `revision.targets.${request.kind.replace('revision:', '') as 'flow' | 'narration' | 'both'}`,
                ),
              })}
              {' · '}
              {formatDateTime(request.createdAt)}
            </Typography>
            <Typography variant="body" as="p" className="break-words whitespace-pre-wrap">
              {request.body || t('record.noInstruction')}
            </Typography>
          </li>
        ))}
      </ol>
    </details>
  )
}
