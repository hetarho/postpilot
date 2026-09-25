import { useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, FieldLabel, FieldMessage, TextField, Typography } from '@/shared/ui'
import { giftLinkFor } from '../model/gift-link'

/** A gift link the operator can copy (GIFT-6). The token is shown only inside the link field
 *  and nothing logs it. A clipboard that refuses selects the field instead of throwing, so the
 *  link can still be copied by hand. */
export function CopyGiftLink({ token, compact = false }: { token: string; compact?: boolean }) {
  const { t } = useTranslation('plans')
  const id = useId()
  const field = useRef<HTMLInputElement>(null)
  const [status, setStatus] = useState<'idle' | 'copied' | 'failed'>('idle')
  const link = giftLinkFor(token, window.location.origin)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link)
      setStatus('copied')
    } catch {
      field.current?.select()
      setStatus('failed')
    }
  }

  return (
    <div className="grid gap-2">
      {!compact && <FieldLabel htmlFor={id}>{t('giftLink.label')}</FieldLabel>}
      <div className="flex gap-2">
        <TextField
          id={id}
          ref={field}
          readOnly
          value={link}
          aria-label={compact ? t('giftLink.label') : undefined}
          className="min-w-0 flex-1"
          onFocus={(event) => event.currentTarget.select()}
        />
        <Button variant="secondary" onClick={() => void copy()}>
          {t('giftLink.copy')}
        </Button>
      </div>
      {status === 'copied' && (
        <Typography variant="label" as="p" role="status" className="text-content-secondary">
          {t('giftLink.copied')}
        </Typography>
      )}
      {status === 'failed' && <FieldMessage>{t('giftLink.copyFailed')}</FieldMessage>}
    </div>
  )
}
