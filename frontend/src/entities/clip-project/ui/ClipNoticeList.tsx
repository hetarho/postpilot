import { useTranslation } from 'react-i18next'
import { Typography } from '@/shared/ui'
import { clipNoticeKey, type ClipNotice } from '../model/notices'
import type { ClipEditCut } from '@/entities/clip-plan/@x/clip-project'

export function ClipNoticeList({
  notices = [],
  language,
  cuts = [],
  withTargets = false,
}: {
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  cuts?: readonly ClipEditCut[]
  withTargets?: boolean
}) {
  const { t } = useTranslation('clips', { lng: language })
  if (!notices.length) return null
  return (
    <ul className="text-content-secondary space-y-1" aria-label={t('notices.label')}>
      {notices.map((notice, index) => {
        const cut = cuts.findIndex((c) => c.id === notice.cutId)
        const target = notice.elementId
          ? t('notices.text')
          : cut >= 0
            ? t('correction.cut', { number: cut + 1 })
            : notice.cutId
              ? t('notices.omittedCut')
              : ''
        return (
          <li key={`${notice.code}:${notice.cutId}:${notice.elementId}:${index}`}>
            <Typography variant="meta">
              {withTargets && target ? `${target} · ` : ''}
              {t(clipNoticeKey(notice))}
            </Typography>
          </li>
        )
      })}
    </ul>
  )
}
