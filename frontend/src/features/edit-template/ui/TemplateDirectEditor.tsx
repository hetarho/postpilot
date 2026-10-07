import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { AuthoringArtifact } from '@/entities/ai-authoring'
import {
  TEMPLATE_LIMITS,
  TEMPLATE_PARSE_OPTIONS,
  TemplateComposition,
  TemplateSource,
  parseTemplate,
  askFields,
  remainingChars,
  readCompositionWorkingState,
  type CompositionWorkingState,
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
  SegmentedControl,
  TextField,
  Textarea,
  Typography,
} from '@/shared/ui'

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
  const [mode, setMode] = useState<'builder' | 'source'>('builder')
  const [focusSource, setFocusSource] = useState<TemplateArea | null>(null)
  const parsed = parseTemplate(source.titleArea, source.body, TEMPLATE_PARSE_OPTIONS)
  const titleAskLabels = new Set(
    askFields(source.titleArea, { ...TEMPLATE_PARSE_OPTIONS, titleArea: true }).map(
      (field) => field.label,
    ),
  )
  const failureIn = (area: TemplateArea) =>
    !parsed.ok && parsed.failure.area === area ? parsed.failure : null
  let working: Record<string, unknown> = {}
  try {
    working = JSON.parse(source.builderState || '{}') as Record<string, unknown>
  } catch {
    /* Raw source remains authoritative. */
  }
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
  const composition = (area: TemplateArea, state: CompositionWorkingState) => {
    onChange({
      ...source,
      [area === 'body' ? 'body' : 'titleArea']: state.source,
      builderState: JSON.stringify({ ...working, [area]: state }),
    })
  }
  const fix = (area: TemplateArea) => {
    setFocusSource(area)
    setMode('source')
  }
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
      <NumberField
        id={id + '-length'}
        value={source.targetLength ?? ''}
        remembered={
          typeof numberMemory.targetLength === 'string' ? numberMemory.targetLength : undefined
        }
        disabled={disabled}
        tick={t('numbers.targetLengthTick', { ns: 'templates' })}
        label={t('numbers.targetLength', { ns: 'templates' })}
        help={t('numbers.targetLengthHelp', { ns: 'templates' })}
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
        help={t('numbers.tagCountHelp', { ns: 'templates' })}
        min={POST_TAG_COUNT_MIN}
        max={POST_TAG_COUNT_MAX}
        fallback={POST_TAG_COUNT_DEFAULT}
        onChange={(value, last) => numberChange('tagCount', value, last)}
      />
      <SegmentedControl
        value={mode}
        options={[
          { value: 'builder', label: t('screen.mode.builder', { ns: 'templates' }) },
          { value: 'source', label: t('screen.mode.source', { ns: 'templates' }) },
        ]}
        onChange={(next) => {
          setMode(next)
          setFocusSource(null)
        }}
        disabled={disabled}
        ariaLabel={t('screen.mode.aria', { ns: 'templates' })}
      />
      <section aria-labelledby={id + '-title-area'}>
        <Typography variant="fieldTitle" as="h3" id={id + '-title-area'}>
          {t('screen.titleArea.heading', { ns: 'templates' })}{' '}
          {t('form.optional', { ns: 'common' })}
        </Typography>
        <Typography variant="body" as="p" className="text-content-secondary mt-2">
          {t('screen.titleArea.help', { ns: 'templates' })}
        </Typography>
        {mode === 'source' ? (
          <TemplateSource
            area="title_area"
            value={source.titleArea}
            disabled={disabled}
            failure={failureIn('title_area')}
            autoFocus={focusSource === 'title_area'}
            onChange={(value) => change('titleArea', value)}
            className="mt-3"
          />
        ) : (
          <TemplateComposition
            area="title_area"
            value={source.titleArea}
            disabled={disabled}
            failure={failureIn('title_area')}
            workingState={readCompositionWorkingState(working.title_area, source.titleArea)}
            onWorkingChange={(state) => composition('title_area', state)}
            onChange={(value) => change('titleArea', value)}
            onFixInSource={() => fix('title_area')}
            className="mt-3"
          />
        )}
      </section>
      <section aria-labelledby={id + '-body'}>
        <Typography variant="fieldTitle" as="h3" id={id + '-body'}>
          {t('create.body', { ns: 'templates' })}
        </Typography>
        <Typography variant="body" as="p" className="text-content-secondary mt-2">
          {t(mode === 'source' ? 'screen.sourceHelp' : 'screen.compositionHelp', {
            ns: 'templates',
          })}
        </Typography>
        {mode === 'source' ? (
          <TemplateSource
            value={source.body}
            disabled={disabled}
            failure={failureIn('body')}
            autoFocus={focusSource === 'body'}
            onChange={(value) => change('body', value)}
            className="mt-3"
          />
        ) : (
          <TemplateComposition
            value={source.body}
            takenAskTitles={titleAskLabels}
            disabled={disabled}
            failure={failureIn('body')}
            workingState={readCompositionWorkingState(working.body, source.body)}
            onWorkingChange={(state) => composition('body', state)}
            onChange={(value) => change('body', value)}
            onFixInSource={() => fix('body')}
            className="mt-3"
          />
        )}
      </section>
    </div>
  )
}

function NumberField({
  id,
  value,
  remembered,
  tick,
  label,
  help,
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
  help: string
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
      <Typography variant="body" as="p" className="text-content-secondary mt-2">
        {help}
      </Typography>
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
