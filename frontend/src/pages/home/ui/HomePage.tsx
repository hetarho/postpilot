import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowUpRight, PenLine, Clapperboard } from 'lucide-react'
import { FirstUseSetupGate } from '@/widgets/creation-setup'
import { choiceStyles, Typography, pageStyles } from '@/shared/ui'

export function HomePage() {
  const { t } = useTranslation('creation')
  return (
    <FirstUseSetupGate>
      <main
        className={pageStyles({
          width: 'wide',
          className: 'flex flex-1 flex-col justify-center py-12 sm:py-20',
        })}
      >
        <div className="mx-auto w-full max-w-3xl">
          <Typography variant="launch" className="break-words">
            {t('home.heading')}
            <br />
            <span className="text-link-fg-current">{t('home.emphasis')}</span>
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-6">
            {t('home.intro')}
          </Typography>
          <nav aria-label={t('home.choices')} className="mt-12 grid gap-4 sm:grid-cols-2 sm:gap-6">
            <Link to="/posts/new" className={choiceStyles()}>
              <div className="flex items-center justify-between">
                <PenLine aria-hidden="true" className="text-link-fg-current size-7" />
                <ArrowUpRight aria-hidden="true" className="text-content-tertiary size-5" />
              </div>
              <div className="min-w-0">
                <Typography variant="title" as="span" className="block">
                  {t('home.post')}
                </Typography>
                <Typography variant="body" as="span" className="text-content-secondary mt-2 block">
                  {t('home.postDescription')}
                </Typography>
              </div>
            </Link>
            <Link to="/clips/new" className={choiceStyles()}>
              <div className="flex items-center justify-between">
                <Clapperboard aria-hidden="true" className="text-link-fg-current size-7" />
                <ArrowUpRight aria-hidden="true" className="text-content-tertiary size-5" />
              </div>
              <div className="min-w-0">
                <Typography variant="title" as="span" className="block">
                  {t('home.clip')}
                </Typography>
                <Typography variant="body" as="span" className="text-content-secondary mt-2 block">
                  {t('home.clipDescription')}
                </Typography>
              </div>
            </Link>
          </nav>
        </div>
      </main>
    </FirstUseSetupGate>
  )
}
