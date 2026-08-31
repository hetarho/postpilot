import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  GuidelineScopeField,
  canSaveGuideline,
  globalScope,
  remainingGuidelineChars,
  useCreateGuidelineCall,
  type GuidelineScope,
} from '@/entities/guideline'
import { Button, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'

/** One rule plus its scope, and the page's committing action right after the fields it commits —
 *  never docked, since a text field inside a bottom bar sits behind the keyboard the moment it is
 *  focused (design-language §8.3). 전역 is the default because a guideline is meant to apply
 *  everywhere unless the user narrows it. */
export function CreateGuidelineForm({
  ownerId,
  className,
}: {
  ownerId: string
  className?: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  const [text, setText] = useState('')
  const [scope, setScope] = useState<GuidelineScope>(globalScope)
  const create = useCreateGuidelineCall(ownerId)

  const disabled = !canSaveGuideline(text, scope) || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    const submitted = text
    try {
      await create.create(submitted, scope)
      // Only what was submitted is cleared; anything typed during the round trip stays. The
      // scope is reset too — the next rule is a new decision, not a continuation of this one.
      setText((current) => (current === submitted ? '' : current))
      setScope(globalScope())
    } catch {
      // The mutation's message renders under the field.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className={className}>
      <FieldLabel htmlFor={`${id}-text`}>{t('create.text', { ns: 'guidelines' })}</FieldLabel>
      <Textarea
        id={`${id}-text`}
        value={text}
        onChange={(event) => setText(event.target.value)}
        rows={3}
        autoGrow
        placeholder={t('create.textPlaceholder', { ns: 'guidelines' })}
        aria-invalid={create.isError || undefined}
        aria-describedby={create.isError ? errorId : undefined}
        className="mt-1"
      />
      <CharCount value={text} />
      <p className="text-content-tertiary mt-2 text-xs">{t('create.help', { ns: 'guidelines' })}</p>

      <GuidelineScopeField
        ownerId={ownerId}
        value={scope}
        onChange={setScope}
        disabled={create.isPending}
        className="mt-6"
      />

      {create.isError && (
        <FieldMessage id={errorId} className="mt-3">
          {create.errorMessage}
        </FieldMessage>
      )}
      <Button
        type="submit"
        variant="cta"
        disabled={disabled}
        pending={create.isPending}
        className="mt-5 w-full sm:w-auto"
      >
        {t('create.submit', { ns: 'guidelines' })}
      </Button>
    </form>
  )
}

/** Counts down rather than up: what a writer needs to know is how much room is left, and the count
 *  goes negative rather than clamping so an over-long paste says how much to cut. */
function CharCount({ value }: { value: string }) {
  const { t } = useTranslation('common')
  const left = remainingGuidelineChars(value)
  return (
    <p
      className={left < 0 ? 'text-field-error mt-2 text-xs' : 'text-content-tertiary mt-2 text-xs'}
      role={left < 0 ? 'status' : undefined}
    >
      {left < 0 ? t('count.exceeded', { count: -left }) : t('count.remaining', { count: left })}
    </p>
  )
}
