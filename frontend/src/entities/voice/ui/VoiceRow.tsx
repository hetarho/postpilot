import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight } from 'lucide-react'
import { Badge, typographyStyles } from '@/shared/ui'
import { useVoiceProfile } from '../api/useVoiceProfile'
import type { Voice } from '../model/types'

/** One voice as a row: its name and how it sounds in one line, or how far it has been made; the
 *  whole row opens it, and `action` — making it the 기본 (VOICE-12) — sits at its end. The link
 *  stretches over the row through its `::after`; the action paints above that layer. The voices
 *  screen lists its other voices this way, and 홈's 말투 sheet lists every voice this way. The row
 *  pads its own content; a list that wants the words on its column's edge pulls itself out by
 *  that padding (`-mx-3`). */
export function VoiceRow({
  ownerId,
  voice,
  action,
}: {
  ownerId: string
  voice: Voice
  action?: ReactNode
}) {
  const { t } = useTranslation(['voices', 'common'])
  const { profile } = useVoiceProfile(ownerId, voice.made ? voice.id : '')
  const impression = profile?.analysis?.ai.impression
  return (
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 rounded-xl px-3 py-3">
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="flex min-w-0 items-center gap-2">
          <Link
            to="/voices/$voiceId"
            params={{ voiceId: voice.id }}
            className={typographyStyles({
              variant: 'fieldTitle',
              className: 'text-content-primary truncate after:absolute after:inset-0',
            })}
          >
            {voice.name}
          </Link>
          {voice.isDefault && (
            <Badge tone="accent" className="shrink-0">
              {t('state.default', { ns: 'common' })}
            </Badge>
          )}
        </span>
        <span className={typographyStyles({ variant: 'meta', className: 'truncate' })}>
          {voice.made ? (impression ?? '') : t('row.learning', { percent: voice.readinessPercent })}
        </span>
      </span>
      {action && <span className="relative shrink-0">{action}</span>}
      <ChevronRight aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
    </li>
  )
}
