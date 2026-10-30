import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { clipChannelPath, rememberClipEntry, type ClipChannel } from '@/entities/clip-project'
import {
  ActionBar,
  buttonStyles,
  useNavigationContext,
  type NavigationLinkProps,
} from '@/shared/ui'

/** The channel section a page lists — the same four keys as the shell's section tab row. */
export type ChannelSection = 'work' | 'templates' | 'voices' | 'guidelines'

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}

/** A section list page's one creation (THEME-67): a floating button that opens this section's
 *  creation directly — the editor, the clip workspace, the template wizard or the section's own
 *  creation sheet (`?new=1`). The other sections' creations are one section tab away. */
export function ChannelActions({
  channel,
  section,
  className = 'mt-auto',
}: {
  channel?: ClipChannel
  section: ChannelSection
  /** The dock's placement; `mt-auto` holds it at the bottom of a short list (THEME-24). A list
   *  whose rows run edge to edge adds `action-list-inset` so the dock keeps the page gutter. */
  className?: string
}) {
  const { t } = useTranslation('channel')
  const { user } = useSession()
  const Link = useNavigationContext()?.Link ?? NativeLink
  const clip = channel ? `channel=${channel}` : ''
  const href = {
    work: channel ? `/clips/new?${clip}` : '/posts/new',
    templates: channel ? `/video-templates/new?${clip}` : '/templates/new',
    voices: channel ? `/voices?new=1&${clip}` : '/voices?new=1',
    guidelines: channel ? `/video-guidelines?new=1&${clip}` : '/guidelines?new=1',
  }[section]
  // A new post or clip keeps the channel as its origin, so the editor and the clip workspace
  // come back here.
  const rememberCreation = () => {
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
    <ActionBar dock="list" className={className}>
      <Link
        href={href}
        onClick={section === 'work' ? rememberCreation : undefined}
        className={buttonStyles({ variant: 'cta', className: 'rounded-full' })}
      >
        <Plus aria-hidden="true" className="size-5 shrink-0" />
        {t(`button.${section === 'work' && channel ? 'video' : section}`)}
      </Link>
    </ActionBar>
  )
}
