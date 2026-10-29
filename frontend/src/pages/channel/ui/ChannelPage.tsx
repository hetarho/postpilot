import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight, Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { clipChannelPath, rememberClipEntry, type ClipChannel } from '@/entities/clip-project'
import { buttonStyles, pageStyles, Typography, typographyStyles } from '@/shared/ui'

function ChannelPage({ channel }: { channel?: ClipChannel }) {
  const { t } = useTranslation('channel')
  const { user } = useSession()
  const label = !channel ? 'naverBlog' : channel === 'instagram_clip' ? 'instaClip' : 'naverClip'
  const remember = () => {
    if (channel)
      rememberClipEntry(user?.id ?? '', {
        path: clipChannelPath(channel),
        section: channel,
        filters: { channel },
        scrollY: window.scrollY,
      })
    else
      rememberPostEntry(user?.id ?? '', {
        path: '/naver-blog',
        section: 'naverBlog',
        filters: {},
        scrollY: window.scrollY,
      })
  }
  return (
    <main className={pageStyles({ width: 'wide' })}>
      <Typography variant="display">{t(label)}</Typography>
      <Link
        to={channel ? '/clips/new' : '/posts/new'}
        search={channel ? { channel } : {}}
        onClick={remember}
        className={buttonStyles({ variant: 'cta', className: 'mt-6 inline-flex gap-2' })}
      >
        <Plus aria-hidden="true" className="size-5 shrink-0" />
        {t(channel ? 'createClip' : 'createPost')}
      </Link>
      <nav aria-label={t(label)} className="mt-8">
        <ul className="divide-divider divide-y">
          {(
            [
              {
                to: channel ? '/clips' : '/posts',
                key: 'history',
                search: channel ? { channel } : {},
              },
              {
                to: channel ? '/video-templates' : '/templates',
                key: 'templates',
                search: channel ? { channel } : {},
              },
              {
                to: channel ? '/video-guidelines' : '/guidelines',
                key: 'guidelines',
                search: channel ? { channel } : {},
              },
            ] as const
          ).map(({ to, key, search }) => (
            <li key={key}>
              <Link
                to={to}
                search={search}
                className={typographyStyles({
                  variant: 'body',
                  className:
                    'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center justify-between gap-3 rounded-md px-4 py-4',
                })}
              >
                {t(key)}
                <ChevronRight aria-hidden="true" className="size-4 shrink-0" />
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </main>
  )
}
export function NaverBlogChannelPage() {
  return <ChannelPage />
}
export function NaverClipChannelPage() {
  return <ChannelPage channel="naver_clip" />
}
export function InstaClipChannelPage() {
  return <ChannelPage channel="instagram_clip" />
}
