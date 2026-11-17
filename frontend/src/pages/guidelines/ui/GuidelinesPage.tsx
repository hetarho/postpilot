import { useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { GuidelineDirectory } from '@/widgets/guideline-directory'
import { KitScreenHeader } from '@/widgets/writing-kit'

/** /guidelines: a post's 작문 지침 (GUIDE-20), one piece of the writing kit. The page opens on
 *  what a guideline does to a post, with the ONE 새 지침 menu in its heading row, over the rules
 *  as the stack the writer is given. `?new=1` opens an empty new one at once — where the template
 *  request's 지침 만들기 (TMPL-61) lands; closing it drops the parameter, keeping a post the page
 *  was opened from (`?post=`, POST-123) so its way back survives. */
export function GuidelinesPage() {
  const { t } = useTranslation('guidelines')
  const search = useSearch({ strict: false }) as { new?: 1; post?: string }
  const navigate = useNavigate()
  return (
    <GuidelineDirectory
      kind="post"
      createOpen={search.new === 1}
      onCreateClosed={() =>
        void navigate({ to: '/guidelines', search: { post: search.post }, replace: true })
      }
      renderHeader={(actions) => (
        <KitScreenHeader
          kit="guideline"
          title={t('page.title')}
          statement={t('statement')}
          actions={actions}
          className="pt-2 sm:pt-4"
        />
      )}
    />
  )
}
