import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { appFailureFromConnect } from '@/shared/api'
import {
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
  POST_TARGET_LENGTH_DEFAULT,
  POST_TARGET_LENGTH_MAX,
  POST_TARGET_LENGTH_MIN,
} from '@/shared/config'
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
import { useGenerationOptions, type GenerationOptionValues } from '../api/useGenerationOptions'

/** 목표 분량 and 태그 개수 — the parts of the writing brief that are validated numbers rather than
 *  choices, so they keep one explicit save while their neighbours in the brief apply on selection.
 *  The length is opt-in (unticked is natural length); the tag count is always a number (POST-63).
 *
 *  It renders as a form BODY, with no surface of its own: the brief widget owns the overlay it
 *  sits in (`widgets/generation-brief`), and a popover inside a popover is not a shape. */
export function GenerationOptions({
  slug,
  targetLength,
  tagCount,
  disabled,
  onSaved,
  onClose,
}: {
  slug: string
  targetLength?: number
  tagCount: number
  disabled: boolean
  onSaved: (values: GenerationOptionValues) => void
  /** Dismisses the surface this sits in, on 취소 and on a landed save. */
  onClose: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  const save = useGenerationOptions()
  const [enabled, setEnabled] = useState(targetLength !== undefined)
  const [value, setValue] = useState(targetLength?.toString() ?? '')
  const [tags, setTags] = useState(String(tagCount))
  const parsed = Number(value)
  const lengthValid =
    !enabled ||
    (Number.isInteger(parsed) &&
      parsed >= POST_TARGET_LENGTH_MIN &&
      parsed <= POST_TARGET_LENGTH_MAX)
  const parsedTags = Number(tags)
  const tagsValid =
    tags !== '' &&
    Number.isInteger(parsedTags) &&
    parsedTags >= POST_TAG_COUNT_MIN &&
    parsedTags <= POST_TAG_COUNT_MAX
  const valid = lengthValid && tagsValid

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        if (!valid) return
        const next: GenerationOptionValues = {
          targetLength: enabled ? parsed : undefined,
          tagCount: parsedTags,
        }
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
          // Ticking the box reveals the field with a usable number ALREADY in it. Revealing an
          // empty one puts a range error under a control nobody has touched yet, and asks the
          // user to invent a character count before they have any reason to have one. A value
          // they typed earlier outranks the default, so unticking and reticking never loses it.
          onChange={(event) => {
            setEnabled(event.target.checked)
            if (event.target.checked && !value) setValue(String(POST_TARGET_LENGTH_DEFAULT))
          }}
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
            aria-invalid={!lengthValid || undefined}
            className="mt-1"
          />
          {!lengthValid && (
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
      {/* Always visible, no enabling tick: a tag list has no "natural" count to fall back to, so
          the field arrives holding the number the next run will use (POST-63). */}
      <div className="mt-3">
        <FieldLabel htmlFor={`generation-tags-${slug}`}>
          {t('generation.options.tagCount', { ns: 'posts' })}
        </FieldLabel>
        <TextField
          id={`generation-tags-${slug}`}
          type="number"
          min={POST_TAG_COUNT_MIN}
          max={POST_TAG_COUNT_MAX}
          value={tags}
          disabled={disabled}
          onChange={(event) => setTags(event.target.value)}
          aria-invalid={!tagsValid || undefined}
          className="mt-1"
        />
        {!tagsValid && (
          <FieldMessage className="mt-1">
            {t('generation.options.tagCountRange', {
              ns: 'posts',
              min: POST_TAG_COUNT_MIN,
              max: POST_TAG_COUNT_MAX,
            })}
          </FieldMessage>
        )}
      </div>
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
