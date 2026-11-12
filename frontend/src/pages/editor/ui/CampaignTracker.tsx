import { useTranslation } from 'react-i18next'
import { RequirementsTracker, type AdvertiserGuide } from '@/entities/advertiser-guide'
import { Typography } from '@/shared/ui'
import type { GuideStep } from '../model/guide'
import { draftKeywordCoverage } from '../model/requirements'

/** Input coverage, not a verdict on generated content. Advisory throughout ①. */
export function CampaignTracker({
  guide,
  photoCount,
  title,
  memo,
  answers,
  step,
  onEdit,
}: {
  guide: AdvertiserGuide
  photoCount: number
  title: string
  memo: string
  answers: readonly { text: string; enabled: boolean }[]
  step: GuideStep
  onEdit: () => void
}) {
  const { t } = useTranslation('posts')
  const coverage = draftKeywordCoverage(guide.keywords, { title, memo, answers })
  const details = step === 'details' || step === 'story'
  return (
    <RequirementsTracker
      coverage={{
        photos: { attached: photoCount, min: guide.minPhotos },
        keywords: coverage.map(({ keyword, present }) => ({ keyword, held: present })),
      }}
      list={details}
      onEdit={onEdit}
      notes={
        step === 'review' && (
          <>
            {guide.minPhotos !== undefined && photoCount < guide.minPhotos && (
              <Typography variant="meta" as="p">
                {t('guide.tracker.morePhotos', { count: guide.minPhotos - photoCount })}
              </Typography>
            )}
            <Typography variant="meta" as="p">
              {t('guide.tracker.remaining', {
                keywords:
                  coverage
                    .filter((item) => !item.present)
                    .map((item) => item.keyword)
                    .join(', ') || t('guide.tracker.none'),
              })}
            </Typography>
            {guide.minChars !== undefined && (
              <Typography variant="meta" as="p">
                {t('guide.tracker.charsLater', { minimum: guide.minChars })}
              </Typography>
            )}
            {guide.hashtags.length > 0 && (
              <Typography variant="meta" as="p" className="break-words">
                {t('guide.tracker.tagsLater', {
                  tags: guide.hashtags.map((tag) => `#${tag}`).join(' '),
                })}
              </Typography>
            )}
            <Typography variant="meta" as="p">
              {t('guide.tracker.advisory')}
            </Typography>
          </>
        )
      }
    />
  )
}
