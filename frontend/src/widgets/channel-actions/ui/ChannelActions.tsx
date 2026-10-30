import { useTranslation } from 'react-i18next'
import { LayoutTemplate, ListChecks, MessageSquareQuote, PenLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { clipChannelPath, rememberClipEntry, type ClipChannel } from '@/entities/clip-project'
import { FloatingActionMenu, type FloatingActionItem } from '@/shared/ui'

/** A channel's floating action menu (THEME-38, THEME-67): its settings destinations above, its
 *  creation nearest the thumb. 말투 is the section every channel shares, so its link carries a
 *  clip channel. */
export function ChannelActions({ channel }: { channel?: ClipChannel }) {
  const { t } = useTranslation('channel')
  const { user } = useSession()
  const menu = !channel ? 'naverBlog' : channel === 'instagram_clip' ? 'instaClip' : 'naverClip'
  // Creation keeps the channel as its origin, so the editor and the clip workspace come back here.
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
  const voices: FloatingActionItem = {
    id: 'voices',
    label: t('voices'),
    icon: MessageSquareQuote,
    href: channel ? `/voices?channel=${channel}` : '/voices',
  }
  const settings: FloatingActionItem[] = channel
    ? [
        {
          id: 'templates',
          label: t('videoTemplates'),
          icon: LayoutTemplate,
          href: `/video-templates?channel=${channel}`,
        },
        {
          id: 'guidelines',
          label: t('videoGuidelines'),
          icon: ListChecks,
          href: `/video-guidelines?channel=${channel}`,
        },
        voices,
      ]
    : [
        { id: 'templates', label: t('postTemplates'), icon: LayoutTemplate, href: '/templates' },
        { id: 'guidelines', label: t('postGuidelines'), icon: ListChecks, href: '/guidelines' },
        voices,
      ]
  const creation: FloatingActionItem = {
    id: 'create',
    label: t(channel ? 'createClip' : 'createPost'),
    icon: PenLine,
    href: channel ? `/clips/new?channel=${channel}` : '/posts/new',
    onSelect: rememberCreation,
  }
  return (
    <FloatingActionMenu
      label={t(channel ? 'newClip' : 'newPost')}
      closeLabel={t('close')}
      menuLabel={t(`menu.${menu}`)}
      groups={[settings, [creation]]}
      // The histories run their rows edge to edge, so the dock keeps the page gutter itself;
      // `mt-auto` holds it at the bottom of a short list (THEME-24).
      className="action-list-inset mt-auto"
    />
  )
}
