import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { appFailureFromConnect } from '@/shared/api'
import { POST_TARGET_LENGTH_MAX, POST_TARGET_LENGTH_MIN } from '@/shared/config'
import {
  AppFailureMessage,
  Button,
  Checkbox,
  FieldLabel,
  FieldMessage,
  Notice,
  TextField,
  typographyStyles,
} from '@/shared/ui'
import { formatNumber } from '@/shared/lib'
import { useGenerationOptions } from '../api/useGenerationOptions'

/** 목표 분량 — the one part of the writing brief that is a validated number rather than a choice,
 *  so it keeps an explicit save while its neighbours in the brief apply on selection.
 *
 *  It renders as a form BODY, with no surface of its own: the brief widget owns the overlay it
 *  sits in (`widgets/generation-brief`), and a popover inside a popover is not a shape. */
export function GenerationOptions({
  slug,
  targetLength,
  disabled,
  onSaved,
  onClose,
}: {
  slug: string
  targetLength?: number
  disabled: boolean
  onSaved: (value?: number) => void
  /** Dismisses the surface this sits in, on 취소 and on a landed save. */
  onClose: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  const save = useGenerationOptions()
  const [enabled, setEnabled] = useState(targetLength !== undefined)
  const [value, setValue] = useState(targetLength?.toString() ?? '')
  const parsed = Number(value)
  const valid =
    !enabled ||
    (Number.isInteger(parsed) &&
      parsed >= POST_TARGET_LENGTH_MIN &&
      parsed <= POST_TARGET_LENGTH_MAX)

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        if (!valid) return
        const next = enabled ? parsed : undefined
        void save
          .save(slug, next)
          .then(() => {
            onSaved(next)
            onClose()
          })
          .catch(() => undefined)
      }}
    >
      <label
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 items-center gap-3',
        })}
      >
        <Checkbox
          checked={enabled}
          disabled={disabled}
          onChange={(event) => setEnabled(event.target.checked)}
        />
        {t('generation.options.useTarget', { ns: 'posts' })}
      </label>
      {enabled && (
        <div className="mt-3">
          <FieldLabel htmlFor={`generation-target-${slug}`}>
            {t('generation.options.target', { ns: 'posts' })}
          </FieldLabel>
          <TextField
            id={`generation-target-${slug}`}
            type="number"
            min={POST_TARGET_LENGTH_MIN}
            max={POST_TARGET_LENGTH_MAX}
            value={value}
            disabled={disabled}
            onChange={(event) => setValue(event.target.value)}
            aria-invalid={!valid || undefined}
            className="mt-1"
          />
          {!valid && (
            <FieldMessage className="mt-1">
              {t('generation.options.range', {
                ns: 'posts',
                min: formatNumber(POST_TARGET_LENGTH_MIN),
                max: formatNumber(POST_TARGET_LENGTH_MAX),
              })}
            </FieldMessage>
          )}
        </div>
      )}
      {save.error && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={appFailureFromConnect(save.error)} />
        </Notice>
      )}
      <div className="mt-4 flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose}>
          {t('action.cancel', { ns: 'common' })}
        </Button>
        <Button
          type="submit"
          variant="secondary"
          disabled={disabled || !valid}
          pending={save.isPending}
        >
          {t('action.save', { ns: 'common' })}
        </Button>
      </div>
    </form>
  )
}
