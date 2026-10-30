import { useNavigate, useSearch } from '@tanstack/react-router'
import { CLIP_CHANNELS, DEFAULT_CLIP_CHANNEL, type ClipChannel } from '@/entities/clip-project'
import { ChannelActions } from '@/widgets/channel-actions'
import { GuidelineDirectory } from '@/widgets/guideline-directory'

/** /video-guidelines: a clip's 영상 지침, the same directory as /guidelines (GUIDE-44) and the 지침
 *  section both clip channels share. `?new=1` opens an empty new one at once — where the channel's
 *  새 지침 lands (THEME-67); closing it drops the parameter and keeps the channel. */
export function VideoGuidelinesPage() {
  const search = useSearch({ strict: false }) as { new?: unknown; channel?: unknown }
  const channel = CLIP_CHANNELS.includes(search.channel as ClipChannel)
    ? (search.channel as ClipChannel)
    : DEFAULT_CLIP_CHANNEL
  const navigate = useNavigate()
  return (
    <GuidelineDirectory
      kind="clip"
      createOpen={search.new === 1 || search.new === '1'}
      onCreateClosed={() =>
        void navigate({ to: '/video-guidelines', search: { channel }, replace: true })
      }
      dock={<ChannelActions channel={channel} section="guidelines" />}
    />
  )
}
