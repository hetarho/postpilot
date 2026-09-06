import { useTranslation } from 'react-i18next'
import { FieldMessage } from './FieldMessage'
import { Typography } from '../typography/Typography'

/** How much of a bounded field is left, and how much it is over.
 *
 *  One primitive rather than a copy per field: the third caller is what earned it, and a counter
 *  that says "12자 남음" in one place and "12 left" in another is the kind of drift a shared field
 *  part exists to prevent.
 *
 *  Over the bound it becomes a `FieldMessage` with `role="status"` — the same red the field's own
 *  refusal uses, announced politely because the user is still typing. */
export function FieldCount({ left, className }: { left: number; className?: string }) {
  const { t } = useTranslation('common')
  return left < 0 ? (
    <FieldMessage role="status" className={className ?? 'mt-2'}>
      {t('count.exceeded', { count: -left })}
    </FieldMessage>
  ) : (
    <Typography variant="meta" as="p" className={className ?? 'mt-2'}>
      {t('count.remaining', { count: left })}
    </Typography>
  )
}
