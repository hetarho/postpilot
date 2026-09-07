import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRemovePaymentMethod } from '@/entities/subscription'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

export function RemovePaymentMethodButton() {
  const { t } = useTranslation('billing')
  const [confirming, setConfirming] = useState(false)
  const remove = useRemovePaymentMethod()

  const confirm = async () => {
    try {
      await remove.remove()
    } catch {
      // The catalog-backed error renders beside the action.
    } finally {
      setConfirming(false)
    }
  }

  return (
    <>
      <Button variant="danger" disabled={remove.isPending} onClick={() => setConfirming(true)}>
        {t('paymentMethod.remove')}
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title={t('paymentMethod.removeTitle')}
        confirmLabel={t('paymentMethod.remove')}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('paymentMethod.removeDescription')}
      </Dialog>
    </>
  )
}
