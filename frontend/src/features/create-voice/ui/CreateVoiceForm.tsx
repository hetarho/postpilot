import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import type { ContentLanguage } from '@/shared/api'
import { VOICE_NAME_MAX_CHARS } from '@/shared/config'
import { activeLocale } from '@/shared/lib'
import { Button, FieldLabel, FieldMessage, Select, TextField } from '@/shared/ui'
import { useCreateVoice } from '../api/useCreateVoice'

/** One field and the page's committing action. In flow, right after the field it commits — never
 *  docked: a text field inside a bottom bar sits behind the keyboard the moment it is focused
 *  (design-language §8.3). */
export function CreateVoiceForm({ ownerId, className }: { ownerId: string; className?: string }) {
  const { t } = useTranslation('voices')
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const [name, setName] = useState('')
  const [sourceLanguage, setSourceLanguage] = useState<ContentLanguage>(() => activeLocale())
  const create = useCreateVoice(ownerId)
  // Counted the way the server counts (Unicode scalar values), so the field and the rule agree
  // on a name made of Hangul and emoji alike.
  const chars = Array.from(name.trim()).length
  const disabled = chars === 0 || chars > VOICE_NAME_MAX_CHARS || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    const submitted = name
    try {
      await create.create(submitted.trim(), sourceLanguage)
      // Only what was submitted is cleared; a name typed during the round trip is kept.
      setName((current) => (current === submitted ? '' : current))
    } catch {
      // The mutation's message renders under the field.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className={className}>
      <FieldLabel htmlFor={id}>{t('create.title')}</FieldLabel>
      <TextField
        id={id}
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder={t('create.placeholder')}
        maxLength={VOICE_NAME_MAX_CHARS * 2}
        autoComplete="off"
        autoCapitalize="off"
        autoCorrect="off"
        enterKeyHint="done"
        aria-invalid={create.isError || undefined}
        aria-describedby={create.isError ? `${hintId} ${errorId}` : hintId}
        className="mt-1"
      />
      <p id={hintId} className="text-content-tertiary mt-2 text-xs">
        {t('create.hint', { count: chars, max: VOICE_NAME_MAX_CHARS })}
      </p>
      <FieldLabel htmlFor={`${id}-language`} className="mt-4">
        {t('create.sourceLanguage')}
      </FieldLabel>
      <Select
        id={`${id}-language`}
        value={sourceLanguage}
        disabled={create.isPending}
        onChange={(event) => setSourceLanguage(event.target.value as ContentLanguage)}
        className="mt-1"
      >
        <option value="ko">{t('contentLanguage.ko', { ns: 'common' })}</option>
        <option value="en">{t('contentLanguage.en', { ns: 'common' })}</option>
      </Select>
      <p className="text-content-tertiary mt-2 text-xs leading-relaxed">
        {t('create.sourceLanguageHelp')}
      </p>
      {create.isError && (
        <FieldMessage id={errorId} className="mt-2">
          {create.errorMessage}
        </FieldMessage>
      )}
      <Button
        type="submit"
        variant="cta"
        disabled={disabled}
        pending={create.isPending}
        className="mt-4 w-full sm:w-auto"
      >
        {t('create.submit')}
      </Button>
    </form>
  )
}
