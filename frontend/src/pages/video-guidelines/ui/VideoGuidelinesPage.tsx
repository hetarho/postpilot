import { useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { GuidelineDirectory } from '@/widgets/guideline-directory'
import { KitScreenHeader } from '@/widgets/writing-kit'
import { guidelineIllustration } from '@/shared/assets'

/** /video-guidelines: a clip's 영상 지침, the same directory and the same stack as /guidelines
 *  (GUIDE-44), under 영상's own title. `?new=1` opens an empty new one at once; closing it drops
 *  the parameter. An old link's `channel` is ignored. */
export function VideoGuidelinesPage() {
  const { t } = useTranslation('guidelines')
  const search = useSearch({ strict: false }) as { new?: unknown }
  const navigate = useNavigate()
  return (
    <GuidelineDirectory
      kind="clip"
      createOpen={search.new === 1 || search.new === '1'}
      onCreateClosed={() => void navigate({ to: '/video-guidelines', search: {}, replace: true })}
      renderHeader={(actions) => (
        <KitScreenHeader
          size="section"
          title={t('clipPage.title')}
          statement={t('clipStatement')}
          illustration={guidelineIllustration}
          actions={actions}
          className="pt-2"
        />
      )}
    />
  )
}
