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
  Popover,
  TextField,
} from '@/shared/ui'
import { formatNumber } from '@/shared/lib'
import { useGenerationOptions } from '../api/useGenerationOptions'

export function GenerationOptions({
  slug,
  targetLength,
  disabled,
  onSaved,
}: {
  slug: string
  targetLength?: number
  disabled: boolean
  onSaved: (value?: number) => void
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
    <Popover label={t('generation.options.title', { ns: 'posts' })} disabled={disabled}>
      {(close) => (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            if (!valid) return
            const next = enabled ? parsed : undefined
            void save
              .save(slug, next)
              .then(() => {
                onSaved(next)
                close()
              })
              .catch(() => undefined)
          }}
        >
          <label className="flex min-h-11 items-center gap-3 text-sm font-medium">
            <Checkbox checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
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
            <Button type="button" variant="ghost" onClick={close}>
              {t('action.cancel', { ns: 'common' })}
            </Button>
            <Button type="submit" variant="secondary" disabled={!valid} pending={save.isPending}>
              {t('action.save', { ns: 'common' })}
            </Button>
          </div>
        </form>
      )}
    </Popover>
  )
}
