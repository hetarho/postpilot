import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { clipChannelPath, rememberClipEntry } from '@/entities/clip-project'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowUpRight, PenLine, Clapperboard, Film } from 'lucide-react'
import { FirstUseSetupGate } from '@/widgets/creation-setup'
import { choiceStyles, Typography, pageStyles } from '@/shared/ui'

const CHANNELS = [
  { key: 'naverBlog', channel: undefined, icon: PenLine },
  { key: 'naverClip', channel: 'naver_clip', icon: Clapperboard },
  { key: 'instaClip', channel: 'instagram_clip', icon: Film },
] as const
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
          <nav aria-label={t('home.choices')} className="grid min-w-0 gap-4 sm:gap-6">
            {CHANNELS.map(({ key, channel, icon: Icon }) => (
              <Link
                key={key}
                to={channel ? '/clips/new' : '/posts/new'}
                search={channel ? { channel } : {}}
                onClick={() =>
                  channel
                    ? rememberClipEntry(user?.id ?? '', {
                        path: clipChannelPath(channel),
                        section: channel,
                        filters: { channel },
                        scrollY: window.scrollY,
                      })
                    : rememberPostEntry(user?.id ?? '', {
                        path: '/naver-blog',
                        section: 'naverBlog',
                        filters: {},
                        scrollY: window.scrollY,
                      })
                }
                aria-labelledby={`home-${key}-label`}
                aria-describedby={`home-${key}-description`}
                className={choiceStyles(
                  'min-h-24 flex-row items-center gap-4 p-4 sm:min-h-28 sm:p-6',
                )}
              >
                <Icon aria-hidden="true" className="text-link-fg-current size-8 shrink-0" />
                <div className="min-w-0 flex-1">
                  <Typography variant="title" as="span" id={`home-${key}-label`} className="block">
                    {t(`home.${key}`)}
                  </Typography>
                  <Typography
                    variant="body"
                    as="span"
                    id={`home-${key}-description`}
                    className="text-content-secondary mt-1 block"
                  >
                    {t(`home.${key}Description`)}
                  </Typography>
                </div>
                <ArrowUpRight
                  aria-hidden="true"
                  className="text-content-tertiary size-5 shrink-0"
                />
              </Link>
            ))}
          </nav>
        </div>
      </main>
    </FirstUseSetupGate>
  )
}
