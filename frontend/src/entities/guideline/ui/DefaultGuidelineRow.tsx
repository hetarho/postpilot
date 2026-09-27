import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import { Badge, FieldMessage, Switch, Typography } from '@/shared/ui'
import type { DefaultGuideline } from '../model/types'

export interface DefaultGuidelineRowProps {
  guideline: DefaultGuideline
  /** Called with the switch's new state; the caller saves it. */
  onToggle: (enabled: boolean) => void
  pending?: boolean
  /** Why the last save was refused, in the catalogue's words. */
  errorMessage?: string
}

/** One 기본 지침 (GUIDE-19): the product's recommended rule, its name, its text and this account's
 *  switch. A switched-off row stays listed, dimmed, so turning one back on is always one flip away.
 *  The text is the server's, word for word, because it is exactly what the writer is given. Laid
 *  out like an owner row — no card sets the defaults apart (ARCH-20). */
export function DefaultGuidelineRow({
  guideline,
  onToggle,
  pending = false,
  errorMessage,
}: DefaultGuidelineRowProps) {
  const { t } = useTranslation('guidelines')
  return (
    <li className="py-4">
      <div className="flex items-start gap-4">
        <div className={twMerge('min-w-0 flex-1', !guideline.enabled && 'opacity-60')}>
          <div className="flex flex-wrap items-center gap-2">
            <Typography variant="label" as="span" className="text-content-primary">
              {guideline.name}
            </Typography>
            <Badge tone="accent">{t('defaults.badge')}</Badge>
          </div>
          <Typography variant="body" className="text-content-secondary mt-1 whitespace-pre-wrap">
            {guideline.text}
          </Typography>
          {guideline.koreanTargetOnly && (
            <Typography variant="meta" as="p" className="mt-1">
              {t('defaults.koreanOnly')}
            </Typography>
          )}
        </div>
        <Switch
          aria-label={t('defaults.use', { name: guideline.name })}
          checked={guideline.enabled}
          disabled={pending}
          onChange={(event) => onToggle(event.currentTarget.checked)}
          className="mt-0.5"
        />
      </div>
      {errorMessage && <FieldMessage className="mt-2">{errorMessage}</FieldMessage>}
    </li>
  )
}
