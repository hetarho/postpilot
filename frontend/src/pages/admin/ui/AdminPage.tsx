import { useTranslation } from 'react-i18next'
import { planLabel, useAccounts } from '@/entities/plan'
import { useSession } from '@/entities/session'
import { UserPlanSelect } from '@/features/manage-users'
import { RefundReviewSection } from '@/features/review-refunds'
import { formatDate } from '@/shared/lib'
import { Notice, Typography } from '@/shared/ui'

/** The operator's account list (QUOTA-25), now the 계정 관리 tab of `/admin`. Composition only:
 *  the tier control is its own feature, and every refusal is the server's — this tab is reachable
 *  only for `master`, but the two admin procedures are refused there too, so a direct visit by
 *  anyone else simply cannot read. The page title and the tab row belong to `AdminLayout`.
 *
 *  The operator's own row carries no tier control: a master leaves master only by another
 *  master (QUOTA-63). The server refuses a self-assignment whatever this renders; an unknown
 *  session leaves every row with its control and the refusal to the server. */
export function AdminPage() {
  const { t } = useTranslation('plans')
  const { accounts, isPending, isError } = useAccounts()
  const { user } = useSession()

  return (
    <section className="mt-6 sm:mt-8">
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('admin.description')}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-6 sm:mt-8">
          {t('admin.loadFailed')}
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('admin.loading')}
        </Typography>
      )}
      {!isError && !isPending && accounts.length === 0 && (
        <Typography variant="body" className="text-content-tertiary mt-8">
          {t('admin.empty')}
        </Typography>
      )}

      {!isError && !isPending && accounts.length > 0 && (
        <ul className="mt-6 grid gap-4 sm:mt-8">
          {accounts.map((account) => (
            // A row per account rather than a table: at 320px a three-column table would either
            // scroll sideways or crush the id, and each row is one unit anyway (THEME-13).
            <li key={account.id} className="bg-surface-raised rounded-md p-4">
              <div className="flex items-baseline justify-between gap-3">
                <Typography variant="label" mono className="text-content-primary min-w-0 break-all">
                  {account.id}
                </Typography>
                <Typography variant="meta" className="shrink-0">
                  {formatDate(account.createdAt)}
                </Typography>
              </div>
              {account.id === user?.id ? (
                <div className="mt-3">
                  <Typography variant="fieldTitle" as="p" className="text-content-primary">
                    {planLabel(account.plan)}
                  </Typography>
                  <Typography variant="meta" as="p" className="mt-1">
                    {t('admin.ownPlanFixed')}
                  </Typography>
                </div>
              ) : (
                <UserPlanSelect account={account} />
              )}
            </li>
          ))}
        </ul>
      )}
      <RefundReviewSection />
    </section>
  )
}
