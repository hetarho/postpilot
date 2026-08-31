import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { PURPOSE_LIMITS, canSavePurpose, remainingChars } from '@/entities/purpose'
import { Button, FieldLabel, FieldMessage, TextField, Textarea } from '@/shared/ui'
import { useCreatePurpose } from '../api/useCreatePurpose'

/** Three fields and the page's committing action, in flow right after the fields it commits —
 *  never docked, since a text field inside a bottom bar sits behind the keyboard the moment it
 *  is focused (design-language §8.3). */
export function CreatePurposeForm({ ownerId, className }: { ownerId: string; className?: string }) {
  const { t } = useTranslation(['purposes', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [instructions, setInstructions] = useState('')
  const create = useCreatePurpose(ownerId)

  const fields = { name, description, instructions }
  const disabled = !canSavePurpose(fields) || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    const submitted = { ...fields }
    try {
      await create.create({
        name: submitted.name.trim(),
        description: submitted.description.trim(),
        instructions: submitted.instructions.trim(),
      })
      // Only what was submitted is cleared; anything typed during the round trip stays.
      setName((current) => (current === submitted.name ? '' : current))
      setDescription((current) => (current === submitted.description ? '' : current))
      setInstructions((current) => (current === submitted.instructions ? '' : current))
    } catch {
      // The mutation's message renders under the fields.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className={className}>
      <FieldLabel htmlFor={`${id}-name`}>{t('create.name', { ns: 'purposes' })}</FieldLabel>
      <TextField
        id={`${id}-name`}
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder={t('create.namePlaceholder', { ns: 'purposes' })}
        autoComplete="off"
        enterKeyHint="next"
        aria-invalid={create.isError || undefined}
        aria-describedby={create.isError ? errorId : undefined}
        className="mt-1"
      />
      <CharCount value={name} max={PURPOSE_LIMITS.name} />

      <FieldLabel htmlFor={`${id}-description`} className="mt-6 block">
        {t('create.description', { ns: 'purposes' })}{' '}
        <span className="text-content-tertiary font-normal">
          {t('form.optional', { ns: 'common' })}
        </span>
      </FieldLabel>
      <Textarea
        id={`${id}-description`}
        value={description}
        onChange={(event) => setDescription(event.target.value)}
        rows={2}
        autoGrow
        placeholder={t('create.descriptionPlaceholder', { ns: 'purposes' })}
        className="mt-1"
      />
      <CharCount value={description} max={PURPOSE_LIMITS.description} />

      <FieldLabel htmlFor={`${id}-instructions`} className="mt-6 block">
        {t('create.instructions', { ns: 'purposes' })}
      </FieldLabel>
      <Textarea
        id={`${id}-instructions`}
        value={instructions}
        onChange={(event) => setInstructions(event.target.value)}
        rows={5}
        autoGrow
        placeholder={t('create.instructionsPlaceholder', { ns: 'purposes' })}
        className="mt-1"
      />
      <CharCount value={instructions} max={PURPOSE_LIMITS.instructions} />
      <p className="text-content-tertiary mt-2 text-xs">{t('create.help', { ns: 'purposes' })}</p>

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
        {t('create.submit', { ns: 'purposes' })}
      </Button>
    </form>
  )
}

/** Counts down rather than up: what a writer needs to know is how much room is left, and the
 *  count goes negative rather than clamping so an over-long paste says how much to cut. */
function CharCount({ value, max }: { value: string; max: number }) {
  const { t } = useTranslation('common')
  const left = remainingChars(value, max)
  return (
    <p
      className={left < 0 ? 'text-field-error mt-2 text-xs' : 'text-content-tertiary mt-2 text-xs'}
      role={left < 0 ? 'status' : undefined}
    >
      {left < 0 ? t('count.exceeded', { count: -left }) : t('count.remaining', { count: left })}
    </p>
  )
}
