import { useNavigate, useSearch } from '@tanstack/react-router'
import { GuidelineDirectory } from '@/widgets/guideline-directory'

/** /guidelines: a post's 작문 지침 (GUIDE-20). `?new=1` opens an empty new one at once, which is
 *  where the template request's 지침 만들기 lands (TMPL-61); closing it drops the parameter. */
export function GuidelinesPage() {
  const search = useSearch({ strict: false }) as { new?: 1 }
  const navigate = useNavigate()
  return (
    <GuidelineDirectory
      kind="post"
      createOpen={search.new === 1}
      onCreateClosed={() => void navigate({ to: '/guidelines', search: {}, replace: true })}
    />
  )
}
