import { useId, useState, type ReactNode, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import {
  Button,
  Disclosure,
  FieldLabel,
  FieldMessage,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import {
  ADVERTISER_GUIDE_MAX_CHARS,
  ADVERTISER_HASHTAG_MAX,
  ADVERTISER_ITEM_MAX_CHARS,
  ADVERTISER_KEYWORD_MAX,
  ADVERTISER_MIN_CHARS_RANGE,
  ADVERTISER_MIN_PHOTOS_RANGE,
} from '../config'
import type { AdvertiserGuide, AdvertiserGuideItem } from '../model/types'

const ALL_ITEMS: readonly AdvertiserGuideItem[] = ['keywords', 'minChars', 'minPhotos', 'hashtags']

/** Controlled guide inputs shared by posts and clips. The host chooses an expanded or compact
 *  list and owns persistence; every input enforces the same advertiser-guide bounds. */
export function AdvertiserGuideFields({
  value,
  onChange,
  items = ALL_ITEMS,
  disabled = false,
  expanded = false,
  afterText,
}: {
  /** Posts keep every requirement visible; clips retain their compact presentation. */
  expanded?: boolean
  afterText?: ReactNode
  value: AdvertiserGuide
  onChange: (next: AdvertiserGuide) => void
  /** The counted items this host offers; a clip has no character or photo minimum. */
  items?: readonly AdvertiserGuideItem[]
  disabled?: boolean
}) {
  const { t } = useTranslation('posts')
  const id = useId()
  return (
    <div className="mt-3">
      <FieldLabel htmlFor={`${id}-text`}>{t('advertiserGuide.text')}</FieldLabel>
      <Textarea
        id={`${id}-text`}
        value={value.text}
        disabled={disabled}
        rows={3}
        autoGrow
        placeholder={t('advertiserGuide.placeholder')}
        aria-describedby={`${id}-text-help`}
        onChange={(event) =>
          onChange({ ...value, text: event.target.value.slice(0, ADVERTISER_GUIDE_MAX_CHARS) })
        }
        className="mt-1"
      />
      <Typography variant="meta" as="p" id={`${id}-text-help`} className="mt-1">
        {t('advertiserGuide.textHelp')}
      </Typography>
      <Typography variant="meta" as="p" className="mt-1">
        {t('advertiserGuide.count', {
          count: value.text.length,
          max: ADVERTISER_GUIDE_MAX_CHARS,
        })}
      </Typography>
      {afterText}
      {items.length > 0 && (
        <Requirements
          expanded={expanded}
          title={t(expanded ? 'advertiserGuide.requirements' : 'advertiserGuide.counted')}
        >
          <div className="space-y-4 pb-2">
            {items.includes('keywords') && (
              <ItemList
                label={t('advertiserGuide.keywords')}
                placeholder={t('advertiserGuide.keywordPlaceholder')}
                values={value.keywords}
                max={ADVERTISER_KEYWORD_MAX}
                disabled={disabled}
                onChange={(keywords) => onChange({ ...value, keywords })}
              />
            )}
            {items.includes('minChars') && (
              <MinimumField
                label={t('advertiserGuide.minChars')}
                range={ADVERTISER_MIN_CHARS_RANGE}
                value={value.minChars}
                disabled={disabled}
                onChange={(minChars) => onChange({ ...value, minChars })}
              />
            )}
            {items.includes('minPhotos') && (
              <MinimumField
                label={t('advertiserGuide.minPhotos')}
                range={ADVERTISER_MIN_PHOTOS_RANGE}
                value={value.minPhotos}
                disabled={disabled}
                onChange={(minPhotos) => onChange({ ...value, minPhotos })}
              />
            )}
            {items.includes('hashtags') && (
              <ItemList
                label={t('advertiserGuide.hashtags')}
                placeholder={t('advertiserGuide.hashtagPlaceholder')}
                values={value.hashtags}
                max={ADVERTISER_HASHTAG_MAX}
                prefix="#"
                disabled={disabled}
                onChange={(hashtags) => onChange({ ...value, hashtags })}
              />
            )}
          </div>
        </Requirements>
      )}
    </div>
  )
}

function Requirements({
  expanded,
  title,
  children,
}: {
  expanded: boolean
  title: string
  children: ReactNode
}) {
  return expanded ? (
    <section className="mt-6" aria-label={title}>
      <Typography variant="fieldTitle" as="h3" className="mb-4">
        {title}
      </Typography>
      {children}
    </section>
  ) : (
    <Disclosure size="row" headingLevel={4} title={title} className="mt-2">
      {children}
    </Disclosure>
  )
}

/** Keywords or hashtags: Enter adds what is typed as one removable item; a repeat (ignoring case,
 *  spaces and a leading '#') or one past the limit is refused in place. */
function ItemList({
  label,
  placeholder,
  values,
  max,
  prefix = '',
  disabled,
  onChange,
}: {
  label: string
  placeholder: string
  values: readonly string[]
  max: number
  prefix?: string
  disabled: boolean
  onChange: (next: string[]) => void
}) {
  const { t } = useTranslation('posts')
  const id = useId()
  const [draft, setDraft] = useState('')
  const [refusal, setRefusal] = useState('')
  const key = (item: string) => item.replace(/^#/, '').replace(/\s+/g, '').toLowerCase()
  const add = () => {
    const item = draft.trim().replace(/^#/, '').trim()
    if (!item) return
    if (values.length >= max) return setRefusal(t('advertiserGuide.tooMany', { max }))
    if (values.some((existing) => key(existing) === key(item)))
      return setRefusal(t('advertiserGuide.repeated'))
    onChange([...values, item])
    setDraft('')
    setRefusal('')
  }
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
    event.preventDefault()
    add()
  }
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {values.length > 0 && (
        <ul aria-label={label} className="mt-1 flex flex-wrap gap-2">
          {values.map((item) => (
            <li key={item}>
              <Button
                type="button"
                variant="secondary"
                size="compact"
                className="min-h-11"
                disabled={disabled}
                aria-label={t('advertiserGuide.remove', { item: prefix + item })}
                onClick={() => onChange(values.filter((existing) => existing !== item))}
              >
                {prefix + item}
                <X className="size-3.5" aria-hidden />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <TextField
        id={id}
        value={draft}
        disabled={disabled || values.length >= max}
        maxLength={ADVERTISER_ITEM_MAX_CHARS + prefix.length}
        placeholder={placeholder}
        enterKeyHint="done"
        onChange={(event) => {
          setDraft(event.target.value)
          setRefusal('')
        }}
        onKeyDown={onKeyDown}
        onBlur={add}
        className="mt-1"
      />
      {refusal && <FieldMessage className="mt-1">{refusal}</FieldMessage>}
    </div>
  )
}

/** A minimum the guide may ask for: empty is "not asked"; a number outside the range stays in the
 *  field with its reason and is not saved. */
function MinimumField({
  label,
  range,
  value,
  disabled,
  onChange,
}: {
  label: string
  range: { min: number; max: number }
  value: number | undefined
  disabled: boolean
  onChange: (next: number | undefined) => void
}) {
  const { t } = useTranslation('posts')
  const id = useId()
  const [text, setText] = useState(value === undefined ? '' : String(value))
  const [refusal, setRefusal] = useState('')
  const [shownValue, setShownValue] = useState(value)
  if (shownValue !== value) {
    setShownValue(value)
    setText(value === undefined ? '' : String(value))
    setRefusal('')
  }
  const commit = (raw: string) => {
    setText(raw)
    const trimmed = raw.trim()
    if (trimmed === '') {
      setRefusal('')
      if (value !== undefined) onChange(undefined)
      return
    }
    const number = Number(trimmed)
    if (!Number.isInteger(number) || number < range.min || number > range.max) {
      setRefusal(t('advertiserGuide.outOfRange', { min: range.min, max: range.max }))
      return
    }
    setRefusal('')
    if (number !== value) onChange(number)
  }
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <TextField
        id={id}
        value={text}
        disabled={disabled}
        inputMode="numeric"
        placeholder={t('advertiserGuide.notAsked')}
        aria-invalid={refusal ? true : undefined}
        onChange={(event) => commit(event.target.value.replace(/[^0-9]/g, ''))}
        className="mt-1 max-w-48"
      />
      {refusal && <FieldMessage className="mt-1">{refusal}</FieldMessage>}
    </div>
  )
}
