import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FileText, Film, ArrowRight } from 'lucide-react'
import { choiceStyles, Typography, pageStyles } from '@/shared/ui'
export function LibraryPage() {
  const { t } = useTranslation('creation')
  return (
    <main className={pageStyles({ width: 'wide' })}>
      <Typography variant="display">{t('library.title')}</Typography>
      <Typography variant="body" className="text-content-secondary mt-3">
        {t('library.description')}
      </Typography>
      <div className="mt-10 grid gap-4 sm:grid-cols-2">
        {(
          [
            { to: '/posts', key: 'posts', Icon: FileText },
            { to: '/clips', key: 'clips', Icon: Film },
          ] as const
        ).map(({ to, key, Icon }) => (
          <Link to={to} key={key} className={choiceStyles()}>
            <Icon aria-hidden="true" className="size-6" />
            <div>
              <Typography
                variant="title"
                as="span"
                className="flex items-center justify-between gap-3"
              >
                {t(`library.${key}`)}
                <ArrowRight aria-hidden="true" className="size-5" />
              </Typography>
              <Typography variant="body" as="span" className="text-content-secondary mt-2 block">
                {t(`library.${key}Description`)}
              </Typography>
            </div>
          </Link>
        ))}
      </div>
    </main>
  )
}
