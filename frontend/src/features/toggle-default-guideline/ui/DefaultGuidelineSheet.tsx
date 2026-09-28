import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import {
  DefaultGuidelineDetail,
  useGuidelines,
  useSetDefaultGuidelineEnabled,
  type DefaultGuideline,
  type GuidelineKind,
} from '@/entities/guideline'
import { Button, FieldMessage, Sheet, Typography } from '@/shared/ui'

/** The dock's `기본 지침` and the sheet it opens (GUIDE-48): every 기본 지침 of the kind in the
 *  product's order, each with its text, so choosing one is informed — the closed list shows names
 *  only. One out of use carries `추가`; one in use reads `적용 중`. Adding keeps the sheet open,
 *  because adding several is one visit, and taking one out of use happens on its row, never here. */
export function DefaultGuidelineSheet({ ownerId, kind }: { ownerId: string; kind: GuidelineKind }) {
  const { t } = useTranslation('guidelines')
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        {t('defaults.open')}
      </Button>
      {open && (
        <DefaultGuidelinePanel ownerId={ownerId} kind={kind} onClose={() => setOpen(false)} />
      )}
    </>
  )
}

function DefaultGuidelinePanel({
  ownerId,
  kind,
  onClose,
}: {
  ownerId: string
  kind: GuidelineKind
  onClose: () => void
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const headingId = useId()
  // The page's own list query, so in-use state follows the same cache the list reads: an 추가
  // here and the row appearing there are one optimistic write.
  const { defaults } = useGuidelines(ownerId, kind)
  return (
    <Sheet
      open
      labelledBy={headingId}
      onClose={onClose}
      header={
        <div className="flex items-center gap-2">
          <Typography variant="title" as="h2" id={headingId} className="min-w-0 flex-1">
            {t('defaults.sheetTitle', { ns: 'guidelines' })}
          </Typography>
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('action.close', { ns: 'common' })}
            onClick={onClose}
            className="shrink-0"
          >
            <X className="size-4" aria-hidden />
          </Button>
        </div>
      }
    >
      <ul className="divide-divider mt-2 divide-y">
        {defaults.map((guideline) => (
          <DefaultGuidelineOption
            key={guideline.key}
            ownerId={ownerId}
            kind={kind}
            guideline={guideline}
          />
        ))}
      </ul>
    </Sheet>
  )
}

/** One 기본 지침 in the sheet. Its own mutation, so a refused 추가 is said under the row it was
 *  refused for (GUIDE-48). */
function DefaultGuidelineOption({
  ownerId,
  kind,
  guideline,
}: {
  ownerId: string
  kind: GuidelineKind
  guideline: DefaultGuideline
}) {
  const { t } = useTranslation('guidelines')
  const toggle = useSetDefaultGuidelineEnabled(ownerId, kind)
  return (
    <li className="py-3">
      <Typography variant="fieldTitle" as="h3">
        {guideline.name}
      </Typography>
      <DefaultGuidelineDetail
        guideline={guideline}
        action={
          guideline.enabled ? (
            <Typography variant="meta" as="span">
              {t('defaults.inUse')}
            </Typography>
          ) : (
            <Button
              variant="secondary"
              aria-label={t('defaults.addNamed', { name: guideline.name })}
              pending={toggle.isPending}
              // The refusal renders under the row from the mutation's own error; the rejected
              // promise has nothing more to say.
              onClick={() => void toggle.setEnabled(guideline.key, true).catch(() => {})}
            >
              {t('defaults.add')}
            </Button>
          )
        }
      />
      {toggle.errorMessage && <FieldMessage className="mt-2">{toggle.errorMessage}</FieldMessage>}
    </li>
  )
}
