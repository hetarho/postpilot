import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  COPY_FEEDBACK_MS,
  TEMPLATE_ASK_MAX_PER_BODY,
  TEMPLATE_PHOTO_ROW_MAX,
} from '@/shared/config'
import { copyText } from '@/shared/lib'
import {
  Button,
  FieldCount,
  FieldLabel,
  FieldMessage,
  Textarea,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { formatGuide } from '../model/guide'
import { TEMPLATE_LIMITS, remainingChars } from '../model/types'
import type { ParseFailure } from '../lib/grammar'

interface TemplateSourceProps {
  value: string
  onChange: (body: string) => void
  disabled?: boolean
  /** The parse failure of THIS body, or null. Computed by the screen, which already parses once
   *  to decide whether the save is allowed — so the two can never disagree. */
  failure: ParseFailure | null
  /** Focused on mount. Set only when the user arrived here from 원문에서 고치기: they asked to fix
   *  the text, so the caret belongs in it. */
  autoFocus?: boolean
  className?: string
}

/** The template body as text — the one surface where this app's grammar is visible (TEMPLATE-26).
 *
 *  It exists for two things the builder cannot do: reading a body that does not parse, and
 *  IMPORTING one. A body written by an outside AI from the format guide is pasted here byte for
 *  byte — no trim, no normalization, on input or on save (TEMPLATE-42, TEMPLATE-8, TEMPLATE-19) —
 *  and switching back to 블록 reseeds the outline from what was pasted.
 *
 *  It is a controlled input over one of the template's own fields, like `TemplateComposition`, so
 *  it lives in the entity and performs no write: the screen's 저장 is still the only one.
 */
export function TemplateSource({
  value,
  onChange,
  disabled = false,
  failure,
  autoFocus = false,
  className,
}: TemplateSourceProps) {
  const { t } = useTranslation('templates')
  const bodyRef = useRef<HTMLTextAreaElement>(null)
  const guideRef = useRef<HTMLTextAreaElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const [copied, setCopied] = useState<'body' | 'guide' | null>(null)
  const [manual, setManual] = useState(false)
  const [guideOpen, setGuideOpen] = useState(false)
  const guide = formatGuide()
  // The same pair the name and description counters use, so one bound moves all three.
  const left = remainingChars(value, TEMPLATE_LIMITS.body)
  const errorId = 'template-source-error'

  useEffect(() => {
    if (autoFocus) bodyRef.current?.focus()
  }, [autoFocus])
  useEffect(() => () => clearTimeout(timer.current), [])

  const copy = async (what: 'body' | 'guide') => {
    // The guide's fallback field lives inside a closed `<details>`, and a selection inside a
    // collapsed element is a selection nobody can see — so it opens BEFORE the copy runs.
    if (what === 'guide') setGuideOpen(true)
    const target = what === 'body' ? bodyRef.current : guideRef.current
    const result = await copyText(what === 'body' ? value : guide, target)
    setManual(!result.copied)
    if (!result.copied) {
      setCopied(null)
      return
    }
    setCopied(what)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setCopied(null), COPY_FEEDBACK_MS)
  }

  return (
    <div className={className}>
      <FieldLabel htmlFor="template-source">{t('source.label')}</FieldLabel>
      <Textarea
        id="template-source"
        ref={bodyRef}
        value={value}
        disabled={disabled}
        rows={12}
        autoGrow
        spellCheck={false}
        autoComplete="off"
        aria-invalid={failure ? true : undefined}
        aria-describedby={failure ? errorId : undefined}
        onChange={(event) => onChange(event.target.value)}
        className={typographyStyles({ variant: 'body', mono: true, className: 'mt-1' })}
      />
      {failure && (
        <FieldMessage id={errorId} role="alert">
          {t('source.error', {
            line: failure.line,
            // Both ceilings travel with every reason: two of them interpolate one, and a
            // reason rendered without its number shows the user a literal `{{max}}`.
            reason: t(`builder.reasons.${failure.reason}`, {
              max: TEMPLATE_PHOTO_ROW_MAX,
              askMax: TEMPLATE_ASK_MAX_PER_BODY,
            }),
          })}
        </FieldMessage>
      )}
      <FieldCount left={left} />

      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="secondary" disabled={disabled} onClick={() => void copy('body')}>
          {t('source.copy')}
        </Button>
        <Button variant="secondary" onClick={() => void copy('guide')}>
          {t('source.copyGuide')}
        </Button>
      </div>
      {/* Always mounted: a live region inserted already holding its message announces nothing. */}
      <Typography variant="meta" as="p" role="status" className="text-content-secondary mt-2">
        {copied ? t('source.copied') : manual ? t('source.manualCopy') : ''}
      </Typography>

      {/* The guide is readable without copying it: a clipboard the browser refuses is exactly
          when a user needs to select it by hand (TEMPLATE-41). */}
      <details
        open={guideOpen}
        onToggle={(event) => setGuideOpen(event.currentTarget.open)}
        className="mt-3"
      >
        <summary
          className={typographyStyles({
            variant: 'label',
            className:
              'active:bg-row-bg-active text-content-secondary min-h-11 cursor-pointer rounded-md px-2 py-3 select-none',
          })}
        >
          {t('source.showGuide')}
        </summary>
        <Textarea
          ref={guideRef}
          value={guide}
          readOnly
          rows={10}
          spellCheck={false}
          aria-label={t('source.showGuide')}
          className={typographyStyles({ variant: 'body', mono: true, className: 'mt-2' })}
        />
      </details>
    </div>
  )
}
