import { useTranslation } from 'react-i18next'
import { Check, Circle } from 'lucide-react'
import type { AdvertiserGuide } from '@/entities/advertiser-guide'
import { Button, Typography } from '@/shared/ui'
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
  const matched = coverage.filter((item) => item.present).length
  const details = step === 'details' || step === 'story'
  return (
    <>
      <aside
        aria-label={t('guide.tracker.heading')}
        className="bg-surface-base top-chrome sticky z-10 mt-2 rounded-lg px-3 py-2 shadow-sm"
      >
        <div className="flex items-center gap-2">
          <div role="status" aria-live="polite" aria-atomic="true" className="min-w-0 flex-1">
            <Typography variant="label" as="p">
              {t('guide.tracker.heading')}
            </Typography>
            <Typography variant="meta" as="p" className="break-words">
              {guide.minPhotos !== undefined
                ? t('guide.tracker.photos', { count: photoCount, minimum: guide.minPhotos })
                : t('guide.tracker.noPhotoMinimum')}
              {' · '}
              {t('guide.tracker.keywords', { matched, total: coverage.length })}
            </Typography>
          </div>
          <Button
            variant="ghost"
            onClick={onEdit}
            aria-label={t('guide.tracker.edit')}
            className="min-h-11 shrink-0"
          >
            {t('guide.change')}
          </Button>
        </div>
        {details && coverage.length > 0 && (
          <ul
            className="mt-1 flex flex-wrap gap-x-3 gap-y-1"
            aria-label={t('guide.tracker.keywordList')}
          >
            {coverage.map(({ keyword, present }) => (
              <li key={keyword} className="flex min-w-0 items-center gap-1">
                {present ? (
                  <Check className="text-link-fg-current size-3.5 shrink-0" aria-hidden />
                ) : (
                  <Circle className="text-content-tertiary size-3.5 shrink-0" aria-hidden />
                )}
                <Typography variant="meta" as="span" className="break-all">
                  {keyword}
                  <span className="sr-only">
                    {' '}
                    · {t(present ? 'guide.tracker.present' : 'guide.tracker.missing')}
                  </span>
                </Typography>
              </li>
            ))}
          </ul>
        )}
      </aside>
      {step === 'review' && (
        <div className="mt-4 space-y-2 px-3">
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
        </div>
      )}
    </>
  )
}
