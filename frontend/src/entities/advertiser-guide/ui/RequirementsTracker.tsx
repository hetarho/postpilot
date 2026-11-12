import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Check, Circle } from 'lucide-react'
import { Button, Typography } from '@/shared/ui'

export interface RequirementsCoverage {
  /** Posts only: the photos attached against 최소 사진 수, or `min` undefined when none is asked. */
  photos?: { attached: number; min?: number }
  keywords: readonly { keyword: string; held: boolean }[]
}

/** The slim tracker pinned under the top chrome on every screen after 요청 (POST-124, CLIP-228).
 *  It reads coverage the host computed from the material alone and gates nothing. `list` names
 *  each keyword as held or missing; `notes` are the host's own lines for its 확인 screen. */
export function RequirementsTracker({
  coverage,
  list,
  notes,
  onEdit,
}: {
  coverage: RequirementsCoverage
  list: boolean
  notes?: ReactNode
  onEdit: () => void
}) {
  const { t } = useTranslation('posts')
  const matched = coverage.keywords.filter((item) => item.held).length
  const photos = coverage.photos
  return (
    <>
      <aside
        aria-label={t('advertiserGuide.tracker.heading')}
        className="bg-surface-base top-chrome sticky z-10 mt-2 rounded-lg px-3 py-2 shadow-sm"
      >
        <div className="flex items-center gap-2">
          <div role="status" aria-live="polite" aria-atomic="true" className="min-w-0 flex-1">
            <Typography variant="label" as="p">
              {t('advertiserGuide.tracker.heading')}
            </Typography>
            <Typography variant="meta" as="p" className="break-words">
              {photos &&
                (photos.min !== undefined
                  ? t('advertiserGuide.tracker.photos', {
                      count: photos.attached,
                      minimum: photos.min,
                    })
                  : t('advertiserGuide.tracker.noPhotoMinimum'))}
              {photos && ' · '}
              {t('advertiserGuide.tracker.keywords', {
                matched,
                total: coverage.keywords.length,
              })}
            </Typography>
          </div>
          <Button
            variant="ghost"
            onClick={onEdit}
            aria-label={t('advertiserGuide.tracker.edit')}
            className="min-h-11 shrink-0"
          >
            {t('advertiserGuide.tracker.change')}
          </Button>
        </div>
        {list && coverage.keywords.length > 0 && (
          <ul
            className="mt-1 flex flex-wrap gap-x-3 gap-y-1"
            aria-label={t('advertiserGuide.tracker.keywordList')}
          >
            {coverage.keywords.map(({ keyword, held }) => (
              <li key={keyword} className="flex min-w-0 items-center gap-1">
                {held ? (
                  <Check className="text-link-fg-current size-3.5 shrink-0" aria-hidden />
                ) : (
                  <Circle className="text-content-tertiary size-3.5 shrink-0" aria-hidden />
                )}
                <Typography variant="meta" as="span" className="break-all">
                  {keyword}
                  <span className="sr-only">
                    {' '}
                    ·{' '}
                    {t(
                      held ? 'advertiserGuide.tracker.present' : 'advertiserGuide.tracker.missing',
                    )}
                  </span>
                </Typography>
              </li>
            ))}
          </ul>
        )}
      </aside>
      {notes && <div className="mt-4 space-y-2 px-3">{notes}</div>}
    </>
  )
}
