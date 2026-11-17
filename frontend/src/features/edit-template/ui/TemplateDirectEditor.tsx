import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { AuthoringArtifact } from '@/entities/ai-authoring'
import {
  TEMPLATE_LIMITS,
  TEMPLATE_PARSE_OPTIONS,
  TemplateSource,
  parseTemplate,
  remainingChars,
  type TemplateArea,
} from '@/entities/template'
import {
  POST_TAG_COUNT_DEFAULT,
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
  POST_TARGET_LENGTH_DEFAULT,
  POST_TARGET_LENGTH_MAX,
  POST_TARGET_LENGTH_MIN,
} from '@/entities/post'
import {
  Checkbox,
  FieldCount,
  FieldLabel,
  FieldMessage,
  TextField,
  Textarea,
  Typography,
} from '@/shared/ui'
import { readBuilderMetadata } from '../model/builder-metadata'

/** The template's own text, typed directly — the secondary path beside the conversation. There
 *  is no block builder: nothing is dragged, moved or reordered by hand; the order of a post is the
 *  order of its text, and the AI changes it when asked. */
export function TemplateDirectEditor({
  source,
  onChange,
  disabled,
}: {
  source: AuthoringArtifact
  onChange: (source: AuthoringArtifact) => void
  disabled: boolean
}) {
  const { t } = useTranslation(['templates', 'common'])
  const id = useId()
  const parsed = parseTemplate(source.titleArea, source.body, TEMPLATE_PARSE_OPTIONS)
  const failureIn = (area: TemplateArea) =>
    !parsed.ok && parsed.failure.area === area ? parsed.failure : null
  const working = readBuilderMetadata(source.builderState)
  const change = (field: 'name' | 'description' | 'body' | 'titleArea', value: string) =>
    onChange({ ...source, [field]: value })
  const numberMemory =
    working.numberMemory && typeof working.numberMemory === 'object'
      ? (working.numberMemory as Record<string, unknown>)
      : {}
  const numberChange = (field: 'targetLength' | 'tagCount', value: string, last: string) =>
    onChange({
      ...source,
      [field]: value,
      builderState: JSON.stringify({
        ...working,
        numberMemory: { ...numberMemory, [field]: last },
      }),
    })
  return (
    <div className="min-w-0 space-y-6">
      <div>
        <FieldLabel htmlFor={id + '-name'}>{t('page.name', { ns: 'templates' })}</FieldLabel>
        <TextField
          id={id + '-name'}
          value={source.name}
          disabled={disabled}
          onChange={(e) => change('name', e.target.value)}
          autoComplete="off"
          className="mt-1"
        />
        <FieldCount left={remainingChars(source.name, TEMPLATE_LIMITS.name)} />
      </div>
      <div>
        <FieldLabel htmlFor={id + '-description'}>
          {t('create.description', { ns: 'templates' })} {t('form.optional', { ns: 'common' })}
        </FieldLabel>
        <Textarea
          id={id + '-description'}
          value={source.description}
          disabled={disabled}
          rows={2}
          autoGrow
          onChange={(e) => change('description', e.target.value)}
          className="mt-1"
        />
        <FieldCount left={remainingChars(source.description, TEMPLATE_LIMITS.description)} />
      </div>
      <section aria-labelledby={id + '-title-area'}>
        <Typography variant="fieldTitle" as="h3" id={id + '-title-area'}>
          {t('screen.titleArea.heading', { ns: 'templates' })}{' '}
          {t('form.optional', { ns: 'common' })}
        </Typography>
        <TemplateSource
          area="title_area"
          value={source.titleArea}
          disabled={disabled}
          failure={failureIn('title_area')}
          onChange={(value) => change('titleArea', value)}
          className="mt-3"
        />
      </section>
      <section aria-labelledby={id + '-body'}>
        <Typography variant="fieldTitle" as="h3" id={id + '-body'}>
          {t('create.body', { ns: 'templates' })}
        </Typography>
        <TemplateSource
          value={source.body}
          disabled={disabled}
          failure={failureIn('body')}
          onChange={(value) => change('body', value)}
          className="mt-3"
        />
      </section>
      <NumberField
        id={id + '-length'}
        value={source.targetLength ?? ''}
        remembered={
          typeof numberMemory.targetLength === 'string' ? numberMemory.targetLength : undefined
        }
        disabled={disabled}
        tick={t('numbers.targetLengthTick', { ns: 'templates' })}
        label={t('numbers.targetLength', { ns: 'templates' })}
        min={POST_TARGET_LENGTH_MIN}
        max={POST_TARGET_LENGTH_MAX}
        fallback={POST_TARGET_LENGTH_DEFAULT}
        onChange={(value, last) => numberChange('targetLength', value, last)}
      />
      <NumberField
        id={id + '-tags'}
        value={source.tagCount ?? ''}
        remembered={typeof numberMemory.tagCount === 'string' ? numberMemory.tagCount : undefined}
        disabled={disabled}
        tick={t('numbers.tagCountTick', { ns: 'templates' })}
        label={t('numbers.tagCount', { ns: 'templates' })}
        min={POST_TAG_COUNT_MIN}
        max={POST_TAG_COUNT_MAX}
        fallback={POST_TAG_COUNT_DEFAULT}
        onChange={(value, last) => numberChange('tagCount', value, last)}
      />
    </div>
  )
}

function NumberField({
  id,
  value,
  remembered,
  tick,
  label,
  min,
  max,
  fallback,
  disabled,
  onChange,
}: {
  id: string
  value: string
  remembered?: string
  tick: string
  label: string
  min: number
  max: number
  fallback: number
  disabled: boolean
  onChange: (value: string, last: string) => void
}) {
  const { t } = useTranslation('templates')
  const last = value || remembered || String(fallback)
  const enabled = value !== ''
  const valid = /^\d+$/.test(value) && Number(value) >= min && Number(value) <= max
  return (
    <div>
      <label className="flex min-h-11 items-center gap-3">
        <Checkbox
          checked={enabled}
          disabled={disabled}
          onChange={(e) => {
            if (e.target.checked) onChange(last, last)
            else onChange('', last)
          }}
        />
        <Typography variant="label" as="span">
          {tick}
        </Typography>
      </label>
      {enabled && (
        <div className="mt-3">
          <FieldLabel htmlFor={id}>{label}</FieldLabel>
          <TextField
            id={id}
            value={value === ' ' ? '' : value}
            type="text"
            inputMode="numeric"
            disabled={disabled}
            aria-invalid={!valid || undefined}
            onChange={(e) => {
              onChange(e.target.value || ' ', e.target.value)
            }}
            className="mt-1"
          />
          {!valid && <FieldMessage>{t('numbers.range', { min, max })}</FieldMessage>}
        </div>
      )}
    </div>
  )
}
