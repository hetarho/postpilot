import { useTranslation } from 'react-i18next'
import { useAccounts } from '@/entities/plan'
import { UserPlanSelect } from '@/features/manage-users'
import { formatDate } from '@/shared/lib'
import { Notice, Typography } from '@/shared/ui'

/** The operator's account list (plan 17), now the 계정 관리 tab of `/admin`. Composition only:
 *  the tier control is its own feature, and every refusal is the server's — this tab is reachable
 *  only for `master`, but the two admin procedures are refused there too, so a direct visit by
 *  anyone else simply cannot read. The page title and the tab row belong to `AdminLayout`. */
export function AdminPage() {
  const { t } = useTranslation('plans')
  const { accounts, isPending, isError } = useAccounts()

  return (
    <section className="mt-8">
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('admin.description')}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
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
        <ul className="mt-8 grid gap-4">
          {accounts.map((account) => (
            // A row per account rather than a table: at 320px a three-column table would either
            // scroll sideways or crush the id, and each row is one unit anyway (§1.4).
            <li key={account.id} className="bg-surface-raised rounded-md p-4">
              <div className="flex items-baseline justify-between gap-3">
                <Typography variant="label" mono className="text-content-primary min-w-0 break-all">
                  {account.id}
                </Typography>
                <Typography variant="meta" className="shrink-0">
                  {formatDate(account.createdAt)}
                </Typography>
              </div>
              <UserPlanSelect account={account} />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
