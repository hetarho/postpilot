import { useTranslation } from 'react-i18next'
import { Clapperboard, LayoutTemplate, ListChecks, MessageSquareQuote, PenLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { rememberPostEntry } from '@/entities/post'
import { clipChannelPath, rememberClipEntry, type ClipChannel } from '@/entities/clip-project'
import { FloatingActionMenu, type FloatingActionItem } from '@/shared/ui'

/** The channel section a page lists — the same four keys as the shell's section tab row. */
export type ChannelSection = 'work' | 'templates' | 'voices' | 'guidelines'
const SECTION_ORDER: readonly ChannelSection[] = ['work', 'templates', 'voices', 'guidelines']

/** A channel's creation menu (THEME-67): every creation of the channel, the current section's
 *  alone nearest the thumb and the other three above it in section order. Each item opens its
 *  creation directly — the editor, the clip workspace, the template wizard or the section's own
 *  creation sheet (`?new=1`). */
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
  const clip = channel ? `channel=${channel}` : ''
  const items: Record<ChannelSection, FloatingActionItem> = {
    work: channel
      ? {
          id: 'work',
          label: t('create.video'),
          icon: Clapperboard,
          href: `/clips/new?${clip}`,
          onSelect: rememberCreation,
        }
      : {
          id: 'work',
          label: t('create.post'),
          icon: PenLine,
          href: '/posts/new',
          onSelect: rememberCreation,
        },
    templates: {
      id: 'templates',
      label: t('create.template'),
      icon: LayoutTemplate,
      href: channel ? `/video-templates/new?${clip}` : '/templates/new',
    },
    voices: {
      id: 'voices',
      label: t('create.voice'),
      icon: MessageSquareQuote,
      href: channel ? `/voices?new=1&${clip}` : '/voices?new=1',
    },
    guidelines: {
      id: 'guidelines',
      label: t('create.guideline'),
      icon: ListChecks,
      href: channel ? `/video-guidelines?new=1&${clip}` : '/guidelines?new=1',
    },
  }
  const menu = !channel ? 'naverBlog' : channel === 'instagram_clip' ? 'instaClip' : 'naverClip'
  return (
    <FloatingActionMenu
      label={t(`button.${section === 'work' && channel ? 'video' : section}`)}
      closeLabel={t('close')}
      menuLabel={t(`menu.${menu}`)}
      groups={[
        SECTION_ORDER.filter((key) => key !== section).map((key) => items[key]),
        [items[section]],
      ]}
      className={className}
    />
  )
}
