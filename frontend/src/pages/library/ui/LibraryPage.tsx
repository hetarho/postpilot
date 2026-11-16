import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FileText, Film, ArrowRight } from 'lucide-react'
import { Typography, pageStyles } from '@/shared/ui'
export function LibraryPage() {
  const { t } = useTranslation('creation')
  return (
    <main className={pageStyles({ width: 'wide' })}>
      <Typography variant="display">{t('library.title')}</Typography>
      <Typography variant="body" className="text-content-secondary mt-3">
        {t('library.description')}
      </Typography>
      <ul className="divide-divider mt-6 divide-y">
        {(
          [
            { to: '/posts', key: 'posts', Icon: FileText },
            { to: '/clips', key: 'clips', Icon: Film },
          ] as const
        ).map(({ to, key, Icon }) => (
          <li key={key}>
            <Link
              to={to}
              className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center gap-3 py-4"
            >
              <Icon aria-hidden="true" className="text-content-secondary size-5 shrink-0" />
              <div className="min-w-0 flex-1">
                <Typography
                  variant="label"
                  as="span"
                  className="flex items-center justify-between gap-3"
                >
                  {t(`library.${key}`)}
                </Typography>
                <Typography variant="body" as="span" className="text-content-secondary mt-1 block">
                  {t(`library.${key}Description`)}
                </Typography>
              </div>
              <ArrowRight aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
            </Link>
          </li>
        ))}
      </ul>
    </main>
  )
}
