import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipQuote } from '@/entities/clip-project'
import { Badge, Button, Checkbox, Sheet, Typography } from '@/shared/ui'
import {
  ClipQuotedPopover,
  QuotedAction,
  type ClipGenerationContext,
} from './ClipGenerationActions'

/** ① 확인's big action while a selected source lacks an observation (CLIP-234, CLIP-40): the
 *  analysis alone, quoted and approved from itself with only that work reserved (QUOTA-45). */
export function ClipAnalyzeAction(context: ClipGenerationContext) {
  const { t } = useTranslation('clips')
  return (
    <ClipQuotedPopover
      {...context}
      mode="analysis"
      hero
      className="min-w-fit flex-1"
      label={t('analysisActions.analyze')}
      variant="cta"
    />
  )
}

/** A source 다시 관찰하기 offers: its batch identity, its name, and whether the owner corrected
 *  its stored observation by hand (CLIP-235). */
export interface ClipReanalyzeSource {
  id: string
  filename: string
  ownerEdited: boolean
}

/** ②'s heading-row 다시 관찰하기 (CLIP-234): the owner chooses sources, a source edited by hand
 *  first confirms that its edits will be lost (CLIP-235), then the analysis of exactly those
 *  sources is quoted and approved on its own (QUOTA-45). */
export function ClipReanalyzeAction({
  sources,
  onApprove,
  ...context
}: Omit<ClipGenerationContext, 'onApprove'> & {
  sources: readonly ClipReanalyzeSource[]
  onApprove(quote: ClipQuote, sourceIds: readonly string[]): void
}) {
  const { t } = useTranslation('clips')
  const [open, setOpen] = useState(false)
  const [chosen, setChosen] = useState<readonly string[]>([])
  const [stage, setStage] = useState<'choose' | 'confirm' | 'approve'>('choose')
  const [confirmed, setConfirmed] = useState<readonly string[]>([])
  const { batch, observe, write, ready, pending } = context
  const live = ready && !!batch && !!observe && !!write && sources.length > 0
  const edited = sources.filter((source) => source.ownerEdited && chosen.includes(source.id))
  const close = () => {
    setOpen(false)
    setStage('choose')
  }
  const toggle = (id: string, on: boolean) =>
    setChosen((current) => (on ? [...current, id] : current.filter((value) => value !== id)))
  return (
    <>
      <Button
        type="button"
        variant="secondary"
        disabled={!live || pending}
        aria-describedby={!live ? context.describedBy : undefined}
        onClick={() => {
          setChosen([])
          setConfirmed([])
          setStage('choose')
          setOpen(true)
        }}
      >
        {t('analysisActions.reanalyze')}
      </Button>
      <Sheet
        open={open && live}
        label={
          stage === 'confirm'
            ? t('analysisActions.confirmTitle')
            : t('analysisActions.reanalyzeTitle')
        }
        onClose={close}
        footer={
          stage === 'approve' ? undefined : (
            <div className="flex flex-wrap justify-end gap-3">
              <Button
                type="button"
                variant="ghost"
                onClick={() => (stage === 'confirm' ? setStage('choose') : close())}
              >
                {stage === 'confirm' ? t('analysisActions.back') : t('analysisActions.cancel')}
              </Button>
              <Button
                type="button"
                variant="cta"
                disabled={chosen.length === 0}
                onClick={() => {
                  if (stage === 'confirm') setConfirmed(edited.map((source) => source.id))
                  setStage(stage === 'choose' && edited.length > 0 ? 'confirm' : 'approve')
                }}
              >
                {stage === 'confirm' ? t('analysisActions.confirm') : t('analysisActions.choose')}
              </Button>
            </div>
          )
        }
      >
        {stage === 'choose' && (
          <div className="space-y-4">
            <Typography variant="body" className="text-content-secondary">
              {t('analysisActions.reanalyzeHelp')}
            </Typography>
            <ul className="space-y-2">
              {sources.map((source) => (
                <li key={source.id}>
                  <label className="flex min-h-11 min-w-0 items-center gap-3">
                    <Checkbox
                      checked={chosen.includes(source.id)}
                      onChange={(event) => toggle(source.id, event.currentTarget.checked)}
                    />
                    <Typography variant="body" as="span" className="min-w-0 break-words">
                      {source.filename}
                    </Typography>
                    {source.ownerEdited && (
                      <Badge tone="neutral">{t('analysisActions.editedMark')}</Badge>
                    )}
                  </label>
                </li>
              ))}
            </ul>
            {chosen.length === 0 && (
              <Typography variant="body" role="status" className="text-content-secondary">
                {t('analysisActions.chooseNone')}
              </Typography>
            )}
          </div>
        )}
        {stage === 'confirm' && (
          <Typography variant="body">
            {t('analysisActions.confirmBody', {
              names: edited.map((source) => source.filename).join(', '),
            })}
          </Typography>
        )}
        {stage === 'approve' && batch && observe && write && (
          <QuotedAction
            ownerId={context.ownerId}
            project={context.project}
            batch={batch}
            observe={observe}
            write={write}
            observeStatus={context.observeStatus}
            mode="analysis"
            reanalyze={chosen}
            preparationRoute={context.preparationRoute}
            label={t('analysisActions.reanalyze')}
            onApprove={(quote) => {
              // A correction can finish saving while the chooser is open. Its latest owner
              // mark must still be confirmed before replacing that source's observations.
              if (edited.some((source) => !confirmed.includes(source.id))) {
                setStage('confirm')
                return
              }
              close()
              onApprove(quote, chosen)
            }}
          />
        )}
      </Sheet>
    </>
  )
}
