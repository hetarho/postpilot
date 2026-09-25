import { useTranslation } from 'react-i18next'
import { CopyGiftLink, useVouchers, type Voucher, type VoucherState } from '@/entities/voucher'
import { IssueVoucherForm } from '@/features/issue-voucher'
import { RevokeVoucherButton } from '@/features/revoke-voucher'
import { formatDate } from '@/shared/lib'
import { Badge, Notice, Typography, type BadgeTone } from '@/shared/ui'

const STATE_TONE = {
  redeemable: 'accent',
  redeemed: 'success',
  expired: 'warning',
  revoked: 'neutral',
  unknown: 'neutral',
} as const satisfies Record<VoucherState, BadgeTone>

const won = new Intl.NumberFormat('ko-KR')

/** The operator's vouchers (GIFT-2, GIFT-14): the issue form over every voucher issued, newest
 *  first. The list is the pilot's sales ledger — each row says whether it was sold and for what,
 *  who redeemed it, and what is left. The page title and the tab row belong to `AdminLayout`. */
export function AdminVouchersPage() {
  const { t } = useTranslation('plans')
  const { vouchers, presets, isPending, isError } = useVouchers()

  return (
    <section className="mt-8">
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('adminVouchers.description')}
      </Typography>

      {!isError && !isPending && <IssueVoucherForm presets={presets} />}

      <Typography variant="title" as="h2" className="mt-10">
        {t('adminVouchers.heading')}
      </Typography>
      {isError && (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('adminVouchers.loadFailed')}
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-4">
          {t('adminVouchers.loading')}
        </Typography>
      )}
      {!isError && !isPending && vouchers.length === 0 && (
        <Typography variant="body" className="text-content-tertiary mt-4">
          {t('adminVouchers.empty')}
        </Typography>
      )}
      {!isError && !isPending && vouchers.length > 0 && (
        <ul className="mt-4 grid gap-4">
          {vouchers.map((voucher) => (
            <VoucherRow key={voucher.id} voucher={voucher} />
          ))}
        </ul>
      )}
    </section>
  )
}

/** One voucher as a stacked row: at 320px a many-column table would scroll sideways, and each
 *  voucher is one unit anyway. From md the facts sit beside the actions. */
function VoucherRow({ voucher }: { voucher: Voucher }) {
  const { t } = useTranslation('plans')

  return (
    <li className="bg-surface-raised grid gap-3 rounded-md p-4 md:grid-cols-[1fr_auto] md:items-start">
      <div className="grid min-w-0 gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <Typography variant="label" as="p" className="text-content-primary">
            {t('adminVouchers.summary', { credits: voucher.credits, days: voucher.validityDays })}
          </Typography>
          <Badge tone={STATE_TONE[voucher.state]}>
            {t(`adminVouchers.state.${voucher.state}`)}
          </Badge>
        </div>
        <Typography variant="meta" as="p">
          {t('adminVouchers.issued', { date: formatDate(voucher.issuedAt) })}
          {' · '}
          {voucher.sale
            ? t('adminVouchers.sold', {
                amount: won.format(voucher.sale.amountKrw),
                payer: voucher.sale.payerName,
              })
            : t('adminVouchers.given')}
        </Typography>
        {voucher.message && (
          <Typography variant="meta" as="p" className="break-words">
            {voucher.message}
          </Typography>
        )}
        {voucher.redeemedBy && (
          <Typography variant="meta" as="p" className="break-all">
            {t('adminVouchers.redeemedBy', {
              account: voucher.redeemedBy,
              date: formatDate(voucher.redeemedAt),
            })}
          </Typography>
        )}
        {voucher.state === 'redeemed' && voucher.creditsExpireAt && (
          <Typography variant="meta" as="p">
            {t('adminVouchers.remaining', {
              credits: voucher.remainingCredits,
              date: formatDate(voucher.creditsExpireAt),
            })}
          </Typography>
        )}
        {voucher.state === 'redeemable' && voucher.token && (
          <div className="mt-2">
            <CopyGiftLink token={voucher.token} compact />
          </div>
        )}
      </div>
      {voucher.state !== 'revoked' && (
        <div className="flex flex-wrap items-start gap-2 md:justify-end">
          <RevokeVoucherButton voucher={voucher} />
        </div>
      )}
    </li>
  )
}
