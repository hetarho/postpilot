import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  requestInspectionTargetKey,
  useRequestInspection,
  type RequestInspectionSelection,
  type RequestInspectionTarget,
} from '@/entities/request-inspection'
import { Button, FieldLabel, Listbox, Sheet, Typography } from '@/shared/ui'
import { DEFAULT_INSPECTION_STAGE, REQUEST_INSPECTION_STATUSES } from '../config'
import { RequestInspectionDocument } from './RequestInspectionDocument'

/** An optional read action. Host inputs are an identity fence, never an invitation to save,
 * start work or reconstruct a historical request from current material. */
export function InspectWritingRequestAction({
  target,
  contextKey = '',
  blockedReason,
  stages,
  defaultStatus = 'current',
  className,
}: {
  target: RequestInspectionTarget | null
  contextKey?: string
  blockedReason?: string
  stages?: readonly string[]
  defaultStatus?: RequestInspectionSelection['status']
  className?: string
}) {
  const { t } = useTranslation('requestInspection')
  const id = useId()
  const scopedTarget = target
    ? { ...target, contextKey: JSON.stringify([target.contextKey ?? '', contextKey]) }
    : null
  const identity = JSON.stringify([
    requestInspectionTargetKey(scopedTarget),
    blockedReason,
    stages,
    defaultStatus,
  ])
  const [session, setSession] = useState({ identity, open: false })
  // Invalidate the latch itself, not only this render's visibility. Reverting local material
  // or returning to a previous account must still require a fresh, intentional open.
  if (session.identity !== identity) setSession({ identity, open: false })
  const open = session.identity === identity && session.open
  return (
    <>
      <Button
        variant="ghost"
        className={className}
        onClick={() => setSession({ identity, open: true })}
      >
        {t('open')}
      </Button>
      <Sheet
        open={open}
        labelledBy={`${id}-title`}
        size="wide"
        onClose={() => setSession({ identity, open: false })}
        header={
          <div className="mb-4 flex items-start justify-between gap-3">
            <Typography variant="title" as="h2" id={`${id}-title`}>
              {t('title')}
            </Typography>
            <Button variant="ghost" onClick={() => setSession({ identity, open: false })}>
              {t('close')}
            </Button>
          </div>
        }
      >
        {/* Also remove the payload during the Sheet exit animation; it cannot remain a picture
            of the previous account, revision or prospective material. */}
        {open && (
          <InspectionSession
            target={scopedTarget}
            blockedReason={blockedReason}
            stages={stages}
            defaultStatus={defaultStatus}
            onScopeLost={() => setSession({ identity, open: false })}
          />
        )}
      </Sheet>
    </>
  )
}

function InspectionSession({
  target,
  blockedReason,
  stages,
  defaultStatus,
  onScopeLost,
}: {
  target: RequestInspectionTarget | null
  blockedReason?: string
  stages?: readonly string[]
  defaultStatus: RequestInspectionSelection['status']
  onScopeLost: () => void
}) {
  const { t } = useTranslation('requestInspection')
  const [stage, setStage] = useState(stages?.[0] ?? DEFAULT_INSPECTION_STAGE)
  const [status, setStatus] = useState(defaultStatus)
  const id = useId()
  const requested = Boolean(target && !blockedReason)
  const query = useRequestInspection(target, { stage, status }, requested)
  const hadScope = useRef(false)
  useEffect(() => {
    if (!requested) return
    if (query.scopeAvailable) hadScope.current = true
    else if (hadScope.current) onScopeLost()
  }, [requested, query.scopeAvailable, onScopeLost])
  const views = query.data?.inspections.length
    ? query.data.inspections
    : query.data?.inspection
      ? [query.data.inspection]
      : []
  return (
    <div className="min-w-0 space-y-6">
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('help')}
      </Typography>
      {blockedReason || !target ? (
        <div role="status">
          <Typography variant="fieldTitle" as="h3">
            {t('unavailable')}
          </Typography>
          <Typography variant="body" className="mt-2">
            {blockedReason ?? t('reason.generic')}
          </Typography>
        </div>
      ) : (
        <>
          <div className="grid min-w-0 gap-3 sm:grid-cols-2">
            <div>
              <FieldLabel id={`${id}-status-label`} htmlFor={`${id}-status`}>
                {t('statusLabel')}
              </FieldLabel>
              <Listbox
                id={`${id}-status`}
                aria-labelledby={`${id}-status-label`}
                value={status}
                options={REQUEST_INSPECTION_STATUSES.map((value) => ({
                  value,
                  label: t(`status.${value}`),
                }))}
                onChange={setStatus}
                className="mt-1 w-full"
              />
            </div>
            {stages && stages.length > 1 && (
              <div>
                <FieldLabel id={`${id}-stage-label`} htmlFor={`${id}-stage`}>
                  {t('stageLabel')}
                </FieldLabel>
                <Listbox
                  id={`${id}-stage`}
                  aria-labelledby={`${id}-stage-label`}
                  value={stage}
                  options={stages.map((value) => ({
                    value,
                    label: t(`stage.${value}`, { defaultValue: value }),
                  }))}
                  onChange={setStage}
                  className="mt-1 w-full"
                />
              </div>
            )}
          </div>
          {/* The live region is mounted before reads complete; no raw Connect error is shown. */}
          <Typography variant="body" role="status" className="text-content-secondary">
            {query.isError
              ? t('loadFailed')
              : query.isPending
                ? t('loading')
                : !query.data
                  ? t('reason.generic')
                  : ''}
          </Typography>
          {!query.isError && views.length > 0 && (
            <RequestInspectionDocument key={`${stage}-${status}`} views={views} />
          )}
        </>
      )}
    </div>
  )
}
