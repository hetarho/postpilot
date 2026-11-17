import { useNavigate, useSearch } from '@tanstack/react-router'
import { GuidelineDirectory } from '@/widgets/guideline-directory'

/** /video-guidelines: a clip's 영상 지침, the same directory as /guidelines (GUIDE-44) and the 지침
 *  section. `?new=1` opens an empty new one at once — where 새 지침 lands (THEME-67); closing it
 *  drops the parameter. An old link's `channel` is ignored. */
export function VideoGuidelinesPage() {
  const search = useSearch({ strict: false }) as { new?: unknown }
  const navigate = useNavigate()
  return (
    <GuidelineDirectory
      kind="clip"
      createOpen={search.new === 1 || search.new === '1'}
      onCreateClosed={() => void navigate({ to: '/video-guidelines', search: {}, replace: true })}
      createMenu
    />
  )
}
