import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { rememberClipEntry } from '@/entities/clip-project'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowUpRight, PenLine, Clapperboard } from 'lucide-react'
import { FirstUseSetupGate } from '@/widgets/creation-setup'
import { choiceStyles, Typography, pageStyles } from '@/shared/ui'

export function HomePage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  return (
    <FirstUseSetupGate>
      <main
        className={pageStyles({
          width: 'board',
          className: 'flex flex-1 flex-col py-6 sm:justify-center sm:py-16 lg:py-20',
        })}
      >
        <div className="grid w-full gap-6 sm:gap-12 lg:grid-cols-2 lg:items-center lg:gap-16">
          <div className="min-w-0">
            <Typography variant="launch" className="break-words">
              {t('home.heading')}
              <br />
              <span className="text-link-fg-current">{t('home.emphasis')}</span>
            </Typography>
            <Typography
              variant="body"
              className="text-content-secondary max-w-measure mt-3 sm:mt-6"
            >
              {t('home.intro')}
            </Typography>
          </div>
          <nav
            aria-label={t('home.choices')}
            className="grid min-w-0 gap-4 sm:grid-cols-2 sm:gap-6 lg:grid-cols-1"
          >
            <Link
              to="/posts/new"
              onClick={() =>
                rememberPostEntry(user?.id ?? '', {
                  path: '/',
                  section: 'creation',
                  filters: {},
                  scrollY: window.scrollY,
                })
              }
              aria-labelledby="home-post-label"
              aria-describedby="home-post-description"
              className={choiceStyles(
                'min-h-24 flex-row items-center gap-4 p-4 sm:min-h-40 sm:flex-col sm:items-stretch sm:gap-6 sm:p-8 lg:min-h-44 lg:flex-row lg:items-center',
              )}
            >
              <div className="flex shrink-0 items-center justify-between sm:contents">
                <PenLine aria-hidden="true" className="text-link-fg-current size-8 shrink-0" />
                <ArrowUpRight
                  aria-hidden="true"
                  className="text-content-tertiary hidden size-5 shrink-0 sm:block lg:order-last"
                />
              </div>
              <div className="min-w-0 lg:flex-1">
                <Typography variant="title" as="span" id="home-post-label" className="block">
                  {t('home.post')}
                </Typography>
                <Typography
                  variant="body"
                  as="span"
                  id="home-post-description"
                  className="text-content-secondary mt-1 block sm:mt-2"
                >
                  {t('home.postDescription')}
                </Typography>
              </div>
            </Link>
            <Link
              to="/clips/new"
              onClick={() =>
                rememberClipEntry(user?.id ?? '', {
                  path: '/',
                  section: 'creation',
                  filters: {},
                  scrollY: window.scrollY,
                })
              }
              aria-labelledby="home-clip-label"
              aria-describedby="home-clip-description"
              className={choiceStyles(
                'min-h-24 flex-row items-center gap-4 p-4 sm:min-h-40 sm:flex-col sm:items-stretch sm:gap-6 sm:p-8 lg:min-h-44 lg:flex-row lg:items-center',
              )}
            >
              <div className="flex shrink-0 items-center justify-between sm:contents">
                <Clapperboard aria-hidden="true" className="text-link-fg-current size-8 shrink-0" />
                <ArrowUpRight
                  aria-hidden="true"
                  className="text-content-tertiary hidden size-5 shrink-0 sm:block lg:order-last"
                />
              </div>
              <div className="min-w-0 lg:flex-1">
                <Typography variant="title" as="span" id="home-clip-label" className="block">
                  {t('home.clip')}
                </Typography>
                <Typography
                  variant="body"
                  as="span"
                  id="home-clip-description"
                  className="text-content-secondary mt-1 block sm:mt-2"
                >
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
