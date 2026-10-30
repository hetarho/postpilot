import { useNavigate, useSearch } from '@tanstack/react-router'
import { ChannelActions } from '@/widgets/channel-actions'
import { GuidelineDirectory } from '@/widgets/guideline-directory'

/** /guidelines: a post's 작문 지침 (GUIDE-20), the 지침 section of 네이버 블로그. `?new=1` opens an
 *  empty new one at once — where the channel's 새 지침 (THEME-67) and the template request's
 *  지침 만들기 (TMPL-61) land; closing it drops the parameter. */
export function GuidelinesPage() {
  const search = useSearch({ strict: false }) as { new?: 1 }
  const navigate = useNavigate()
  return (
    <GuidelineDirectory
      kind="post"
      createOpen={search.new === 1}
      onCreateClosed={() => void navigate({ to: '/guidelines', search: {}, replace: true })}
      dock={<ChannelActions section="guidelines" />}
    />
  )
}
