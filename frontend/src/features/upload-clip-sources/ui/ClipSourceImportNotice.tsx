import { useTranslation } from 'react-i18next'
import { Button, Notice } from '@/shared/ui'
import type { ClipImportProblem } from '../api/import'

export function ClipSourceImportNotice({
  problems,
  uncertain,
  onDismiss,
}: {
  problems: readonly ClipImportProblem[]
  uncertain: boolean
  onDismiss: () => void
}) {
  const { t } = useTranslation('clips')
  if (!problems.length && !uncertain) return null
  const reasons = [...new Set(problems.map((problem) => problem.reason))]
  return (
    <Notice tone="warning" role="alert" className="my-4">
      <div className="w-full min-w-0 space-y-2">
        <p>{t('sourceImport.kept')}</p>
        {uncertain && <p>{t('sourceImport.uncertain')}</p>}
        {reasons.map((reason) => (
          <p key={reason}>
            {t(`sourceImport.reason.${reason}`)}{' '}
            <span className="break-all">
              {problems
                .filter((problem) => problem.reason === reason)
                .map((problem) => problem.filename)
                .join(', ')}
            </span>
          </p>
        ))}
      </div>
      <Button variant="ghost" onClick={onDismiss}>
        {t('sourceImport.dismiss')}
      </Button>
    </Notice>
  )
}
