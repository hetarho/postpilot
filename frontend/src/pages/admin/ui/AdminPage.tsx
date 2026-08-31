import { useTranslation } from 'react-i18next'
import { useAccounts } from '@/entities/plan'
import { UserPlanSelect } from '@/features/manage-users'
import { formatDate } from '@/shared/lib'
import { Notice } from '@/shared/ui'

/** The operator's account list (plan 17). Composition only: the tier control is its own feature,
 *  and every refusal is the server's — this screen is reachable only for `master`, but the two
 *  admin procedures are refused there too, so a direct visit by anyone else simply cannot read. */
export function AdminPage() {
  const { t } = useTranslation('plans')
  const { accounts, isPending, isError } = useAccounts()

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t('admin.title')}</h1>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        {t('admin.description')}
      </p>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          {t('admin.loadFailed')}
        </Notice>
      )}
      {!isError && isPending && (
        <p role="status" className="text-content-tertiary mt-8 text-sm">
          {t('admin.loading')}
        </p>
      )}
      {!isError && !isPending && accounts.length === 0 && (
        <p className="text-content-tertiary mt-8 text-sm">{t('admin.empty')}</p>
      )}

      {!isError && !isPending && accounts.length > 0 && (
        <ul className="mt-8 grid gap-4">
          {accounts.map((account) => (
            // A row per account rather than a table: at 320px a three-column table would either
            // scroll sideways or crush the id, and each row is one unit anyway (§1.4).
            <li key={account.id} className="bg-surface-raised rounded-md p-4">
              <div className="flex items-baseline justify-between gap-3">
                <span className="min-w-0 font-mono text-sm break-all">{account.id}</span>
                <span className="text-content-tertiary shrink-0 text-xs">
                  {formatDate(account.createdAt)}
                </span>
              </div>
              <UserPlanSelect account={account} />
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}
