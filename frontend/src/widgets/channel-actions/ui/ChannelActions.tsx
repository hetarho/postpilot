import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { twMerge } from '@/shared/lib'
import { useSession } from '@/entities/session'
import { POST_LIST_PATH, rememberPostEntry } from '@/entities/post'
import { CLIP_LIST_PATH, rememberClipEntry, type ClipChannel } from '@/entities/clip-project'
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
  video = channel !== undefined,
  section,
  className = 'mt-auto',
}: {
  /** Only a new clip's return origin still reads it (T862 removes it); no link carries it. */
  channel?: ClipChannel
  /** The 영상 side of 템플릿 and 지침; 말투 is one section for 글 and 영상. */
  video?: boolean
  section: ChannelSection
  /** The dock's placement; `mt-auto` holds it at the bottom of a short list (THEME-24). A list
   *  whose rows run edge to edge adds `action-list-inset` so the dock keeps the page gutter. */
  className?: string
}) {
  const { t } = useTranslation('channel')
  const { user } = useSession()
  const Link = useNavigationContext()?.Link ?? NativeLink
  const href = {
    work: channel ? `/clips/new?channel=${channel}` : '/posts/new',
    templates: video ? '/video-templates/new' : '/templates/new',
    voices: '/voices?new=1',
    guidelines: video ? '/video-guidelines?new=1' : '/guidelines?new=1',
  }[section]
  // A new post or clip keeps the channel as its origin, so the editor and the clip workspace
  // come back here.
  const rememberCreation = () => {
    if (channel)
      rememberClipEntry(user?.id ?? '', {
        path: CLIP_LIST_PATH,
        section: channel,
        filters: { channel },
        scrollY: window.scrollY,
      })
    else
      rememberPostEntry(user?.id ?? '', {
        path: POST_LIST_PATH,
        section: 'post',
        filters: {},
        scrollY: window.scrollY,
      })
  }
  // Guide: the section's one big action, anchored in the thumb zone — the whole width of a phone,
  // a generous pill at the desk's right edge.
  return (
    <ActionBar dock="list" className={twMerge(className, 'w-full sm:w-fit')}>
      <Link
        href={href}
        onClick={section === 'work' ? rememberCreation : undefined}
        className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full sm:w-auto' })}
      >
        <Plus aria-hidden="true" className="size-6 shrink-0" />
        {t(`button.${section === 'work' && channel ? 'video' : section}`)}
      </Link>
    </ActionBar>
  )
}
