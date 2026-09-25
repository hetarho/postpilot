import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRevokeVoucher, type Voucher } from '@/entities/voucher'
import { AppFailureMessage, Button, Dialog, FieldMessage } from '@/shared/ui'

/** Revokes a voucher after saying what it does to it (GIFT-10): an unredeemed link stops
 *  working; a redeemed one's unspent credits stop counting. Money goes back outside the app. */
export function RevokeVoucherButton({ voucher }: { voucher: Voucher }) {
  const { t } = useTranslation('plans')
  const revoke = useRevokeVoucher()
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await revoke.revoke(voucher.id)
    } catch {
      // The refusal renders beside the button.
    } finally {
      setConfirming(false)
    }
  }

  return (
    <>
      <Button variant="danger" disabled={revoke.isPending} onClick={() => setConfirming(true)}>
        {t('revokeVoucher.action')}
      </Button>
      {revoke.failure && (
        <FieldMessage className="w-full">
          <AppFailureMessage failure={revoke.failure} />
        </FieldMessage>
      )}
      <Dialog
        open={confirming}
        title={t('revokeVoucher.title')}
        confirmLabel={t('revokeVoucher.confirm')}
        pending={revoke.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {voucher.state === 'redeemed'
          ? t('revokeVoucher.redeemed', { credits: voucher.remainingCredits })
          : t('revokeVoucher.unredeemed')}
      </Dialog>
    </>
  )
}
