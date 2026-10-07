import { useTranslation } from 'react-i18next'
import { Button, Sheet, Typography, buttonStyles } from '@/shared/ui'

/** Defensive compatibility if an already-mounted legacy control asks to open its old sheet. */
export function LegacyReviewSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation(['models', 'common'])
  return (
    <Sheet
      open={open}
      label={t('legacyReview.title')}
      onClose={onClose}
      header={<Typography variant="title">{t('legacyReview.title')}</Typography>}
      footer={
        <Button variant="ghost" onClick={onClose}>
          {t('action.close', { ns: 'common' })}
        </Button>
      }
    >
      <Typography variant="body" as="p">
        {t('legacyReview.readOnly')}
      </Typography>
      <a href="/tests" className={buttonStyles({ variant: 'secondary', className: 'mt-4' })}>
        {t('legacyReview.openTests')}
      </a>
    </Sheet>
  )
}
