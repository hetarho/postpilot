import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  TEMPLATE_LIMITS,
  TemplateBodyEditor,
  canSaveTemplate,
  remainingChars,
} from '@/entities/template'
import { Button, FieldLabel, FieldMessage, TextField, Textarea, Typography } from '@/shared/ui'
import { useCreateTemplate } from '../api/useCreateTemplate'

/** Three fields and the page's committing action, in flow right after the fields it commits —
 *  never docked, since a text field inside a bottom bar sits behind the keyboard the moment it
 *  is focused (design-language §8.3). */
export function CreateTemplateForm({
  ownerId,
  className,
}: {
  ownerId: string
  className?: string
}) {
  const { t } = useTranslation(['templates', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  const nameCountId = `${id}-name-count`
  const descriptionCountId = `${id}-description-count`
  const bodyHelpId = `${id}-body-help`
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [body, setBody] = useState('')
  const create = useCreateTemplate(ownerId)

  const fields = { name, description, body }
  const nameExceeded = remainingChars(name, TEMPLATE_LIMITS.name) < 0
  const descriptionExceeded = remainingChars(description, TEMPLATE_LIMITS.description) < 0
  const disabled = !canSaveTemplate(fields) || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    const submitted = { ...fields }
    try {
      await create.create({
        name: submitted.name.trim(),
        description: submitted.description.trim(),
        body: submitted.body.trim(),
      })
      // Only what was submitted is cleared; anything typed during the round trip stays.
      setName((current) => (current === submitted.name ? '' : current))
      setDescription((current) => (current === submitted.description ? '' : current))
      setBody((current: string) => (current === submitted.body ? '' : current))
    } catch {
      // The mutation's message renders under the fields.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className={className}>
      <FieldLabel htmlFor={`${id}-name`}>{t('create.name', { ns: 'templates' })}</FieldLabel>
      <TextField
        id={`${id}-name`}
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder={t('create.namePlaceholder', { ns: 'templates' })}
        autoComplete="off"
        enterKeyHint="next"
        aria-invalid={nameExceeded || create.isError || undefined}
        aria-describedby={`${nameCountId}${create.isError ? ` ${errorId}` : ''}`}
        className="mt-1"
      />
      <CharCount id={nameCountId} value={name} max={TEMPLATE_LIMITS.name} />

      <FieldLabel htmlFor={`${id}-description`} className="mt-6 block">
        {t('create.description', { ns: 'templates' })}{' '}
        <span className="text-content-tertiary">{t('form.optional', { ns: 'common' })}</span>
      </FieldLabel>
      <Textarea
        id={`${id}-description`}
        value={description}
        onChange={(event) => setDescription(event.target.value)}
        rows={2}
        autoGrow
        placeholder={t('create.descriptionPlaceholder', { ns: 'templates' })}
        aria-invalid={descriptionExceeded || undefined}
        aria-describedby={descriptionCountId}
        className="mt-1"
      />
      <CharCount id={descriptionCountId} value={description} max={TEMPLATE_LIMITS.description} />

      <div className="mt-6">
        <FieldLabel htmlFor={`${id}-body`} className="block">
          {t('create.body', { ns: 'templates' })}
        </FieldLabel>
        {/* The structure editor owns the body, its counter and its parse errors: the field's
            value is the grammar's source string, and a plain textarea here would be a second
            way to author the same thing. */}
        <TemplateBodyEditor
          value={body}
          onChange={setBody}
          disabled={create.isPending}
          className="mt-1"
        />
      </div>
      <Typography variant="body" as="p" id={bodyHelpId} className="text-content-secondary mt-2">
        {t('create.help', { ns: 'templates' })}
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
        {t('create.submit', { ns: 'templates' })}
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
