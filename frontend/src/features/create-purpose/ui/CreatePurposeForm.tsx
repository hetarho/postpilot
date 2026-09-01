import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { PURPOSE_LIMITS, canSavePurpose, remainingChars } from '@/entities/purpose'
import { Button, FieldLabel, FieldMessage, TextField, Textarea, Typography } from '@/shared/ui'
import { useCreatePurpose } from '../api/useCreatePurpose'

/** Three fields and the page's committing action, in flow right after the fields it commits —
 *  never docked, since a text field inside a bottom bar sits behind the keyboard the moment it
 *  is focused (design-language §8.3). */
export function CreatePurposeForm({ ownerId, className }: { ownerId: string; className?: string }) {
  const { t } = useTranslation(['purposes', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  const nameCountId = `${id}-name-count`
  const descriptionCountId = `${id}-description-count`
  const instructionsCountId = `${id}-instructions-count`
  const instructionsHelpId = `${id}-instructions-help`
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [instructions, setInstructions] = useState('')
  const create = useCreatePurpose(ownerId)

  const fields = { name, description, instructions }
  const nameExceeded = remainingChars(name, PURPOSE_LIMITS.name) < 0
  const descriptionExceeded = remainingChars(description, PURPOSE_LIMITS.description) < 0
  const instructionsExceeded = remainingChars(instructions, PURPOSE_LIMITS.instructions) < 0
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
        aria-invalid={nameExceeded || create.isError || undefined}
        aria-describedby={`${nameCountId}${create.isError ? ` ${errorId}` : ''}`}
        className="mt-1"
      />
      <CharCount id={nameCountId} value={name} max={PURPOSE_LIMITS.name} />

      <FieldLabel htmlFor={`${id}-description`} className="mt-6 block">
        {t('create.description', { ns: 'purposes' })}{' '}
        <span className="text-content-tertiary">{t('form.optional', { ns: 'common' })}</span>
      </FieldLabel>
      <Textarea
        id={`${id}-description`}
        value={description}
        onChange={(event) => setDescription(event.target.value)}
        rows={2}
        autoGrow
        placeholder={t('create.descriptionPlaceholder', { ns: 'purposes' })}
        aria-invalid={descriptionExceeded || undefined}
        aria-describedby={descriptionCountId}
        className="mt-1"
      />
      <CharCount id={descriptionCountId} value={description} max={PURPOSE_LIMITS.description} />

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
        aria-invalid={instructionsExceeded || undefined}
        aria-describedby={`${instructionsCountId} ${instructionsHelpId}`}
        className="mt-1"
      />
      <CharCount id={instructionsCountId} value={instructions} max={PURPOSE_LIMITS.instructions} />
      <Typography
        variant="body"
        as="p"
        id={instructionsHelpId}
        className="text-content-secondary mt-2"
      >
        {t('create.help', { ns: 'purposes' })}
      </Typography>

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
function CharCount({ id, value, max }: { id: string; value: string; max: number }) {
  const { t } = useTranslation('common')
  const left = remainingChars(value, max)
  return left < 0 ? (
    <FieldMessage id={id} role="status" className="mt-2">
      {t('count.exceeded', { count: -left })}
    </FieldMessage>
  ) : (
    <Typography variant="meta" as="p" id={id} className="mt-2">
      {t('count.remaining', { count: left })}
    </Typography>
  )
}
