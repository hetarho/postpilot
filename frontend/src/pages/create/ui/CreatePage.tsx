import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight, Clapperboard, Film, PenLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { clipChannelPath, rememberClipEntry } from '@/entities/clip-project'
import { ListRowIcon, Typography, cardStyles, pageStyles } from '@/shared/ui'

const CHOICES = [
  { key: 'blog', channel: undefined, icon: PenLine },
  { key: 'naverClip', channel: 'naver_clip', icon: Clapperboard },
  { key: 'instaClip', channel: 'instagram_clip', icon: Film },
] as const

/** Guide: 만들기 — the one tab that asks a question rather than showing a list. Three big tiles,
 *  each the start of its guided flow; the new work comes back to 내 글 under its own channel. */
export function CreatePage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const remember = (channel: (typeof CHOICES)[number]['channel']) =>
    channel
      ? rememberClipEntry(user?.id ?? '', {
          path: clipChannelPath(channel),
          section: channel,
          filters: { channel },
          scrollY: 0,
        })
      : rememberPostEntry(user?.id ?? '', {
          path: '/naver-blog',
          section: 'naverBlog',
          filters: {},
          scrollY: 0,
        })
  return (
    <main
      className={pageStyles({ width: 'prose', className: 'flex flex-1 flex-col pt-4 sm:pt-10' })}
    >
      <Typography variant="largeTitle" className="text-balance">
        {t('guideCreate.title')}
      </Typography>
      <nav aria-label={t('guideCreate.choices')} className="mt-8">
        <ul className="grid gap-4">
          {CHOICES.map(({ key, channel, icon }, index) => (
            <li key={key} className="animate-step-in" style={{ animationDelay: `${index * 60}ms` }}>
              <Link
                to={channel ? '/clips/new' : '/posts/new'}
                search={channel ? { channel } : {}}
                onClick={() => remember(channel)}
                aria-describedby={`create-${key}-outcome`}
                className={cardStyles({
                  interactive: true,
                  className: 'flex min-h-24 items-center gap-4',
                })}
              >
                <ListRowIcon icon={icon} size="lg" />
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <Typography variant="title" as="span">
                    {t(`guideCreate.${key}`)}
                  </Typography>
                  <Typography
                    variant="body"
                    as="span"
                    id={`create-${key}-outcome`}
                    className="text-content-secondary"
                  >
                    {t(`guideCreate.${key}Outcome`)}
                  </Typography>
                </span>
                <ChevronRight
                  aria-hidden="true"
                  className="text-content-tertiary size-6 shrink-0"
                />
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </main>
  )
}
