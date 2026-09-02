import { useId, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStageSelection } from '@/entities/model-catalog'
import type { ContentLanguage } from '@/shared/api'
import { VOICE_DESCRIPTION_MAX_CHARS, VOICE_NAME_MAX_CHARS } from '@/shared/config'
import { activeLocale } from '@/shared/lib'
import {
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Sheet,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import { useCreateVoice } from '../api/useCreateVoice'

/** The directory's one committing action and the overlay it opens. The trigger is rendered here
 *  rather than by the page so the open state stays with the form it opens, the way `Popover`
 *  keeps its own. */
export function CreateVoiceSheet({ ownerId, className }: { ownerId: string; className?: string }) {
  const { t } = useTranslation('voices')
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="cta" className={className} onClick={() => setOpen(true)}>
        {t('create.open')}
      </Button>
      {open && <CreateVoicePanel ownerId={ownerId} onClose={() => setOpen(false)} />}
    </>
  )
}

/** Mounted only while the sheet is open, so every field starts blank on each visit and a failed
 *  attempt's message does not greet the next one. */
function CreateVoicePanel({ ownerId, onClose }: { ownerId: string; onClose: () => void }) {
  const { t } = useTranslation(['voices', 'common'])
  const navigate = useNavigate()
  const id = useId()
  const titleId = `${id}-title`
  const nameId = `${id}-name`
  const countId = `${id}-count`
  const hintId = `${id}-hint`
  const languageId = `${id}-language`
  const languageHintId = `${id}-language-hint`
  const descriptionId = `${id}-description`
  const descriptionCountId = `${id}-description-count`
  const descriptionHintId = `${id}-description-hint`
  const errorId = `${id}-error`
  const [name, setName] = useState('')
  const [sourceLanguage, setSourceLanguage] = useState<ContentLanguage>(() => activeLocale())
  const [description, setDescription] = useState('')
  // The seed is written by the same stage that writes every other voice profile, so the two can
  // never disagree about who authored one voice.
  const { selected: analyzeModel, isPending: modelPending } = useStageSelection('analyze')
  const create = useCreateVoice(ownerId)

  // Counted the way the server counts (Unicode scalar values), so the field and the rule agree
  // on text made of Hangul and emoji alike.
  const chars = Array.from(name.trim()).length
  const exceeded = chars > VOICE_NAME_MAX_CHARS
  const describedChars = Array.from(description.trim()).length
  const describedExceeded = describedChars > VOICE_DESCRIPTION_MAX_CHARS
  const described = describedChars > 0
  // A description needs a model; a plain creation never waits on the catalog.
  const modelMissing = described && !modelPending && !analyzeModel
  const disabled =
    chars === 0 ||
    exceeded ||
    describedExceeded ||
    (described && (modelPending || !analyzeModel)) ||
    create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    try {
      const response = await create.create({
        name: name.trim(),
        sourceLanguage,
        description: description.trim(),
        analyzeModel,
      })
      onClose()
      if (response.voice) {
        // Straight into the voice: a seeded one has a run to watch there, and an unseeded one is
        // where the user fills the profile anyway.
        await navigate({ to: '/voices/$voiceId', params: { voiceId: response.voice.id } })
      }
    } catch {
      // The mutation's message renders under the fields, inside the still-open sheet.
    }
  }

  return (
    <Sheet open labelledBy={titleId} onClose={create.isPending ? () => {} : onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('create.title', { ns: 'voices' })}
      </Typography>
      <form onSubmit={(event) => void submit(event)} className="mt-4">
        <FieldLabel htmlFor={nameId}>{t('create.name', { ns: 'voices' })}</FieldLabel>
        <TextField
          id={nameId}
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder={t('create.placeholder', { ns: 'voices' })}
          maxLength={VOICE_NAME_MAX_CHARS * 2}
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="next"
          autoFocus
          aria-invalid={exceeded || create.isError || undefined}
          aria-describedby={`${countId} ${hintId}`}
          className="mt-1"
        />
        {exceeded ? (
          <FieldMessage id={countId} role="status" className="mt-2">
            {t('count.exceeded', { ns: 'common', count: chars - VOICE_NAME_MAX_CHARS })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p" id={countId} className="mt-2">
            {t('create.count', { ns: 'voices', count: chars, max: VOICE_NAME_MAX_CHARS })}
          </Typography>
        )}
        <Typography variant="body" as="p" id={hintId} className="text-content-secondary mt-1">
          {t('create.emptyHelp', { ns: 'voices' })}
        </Typography>

        <FieldLabel id={`${languageId}-label`} htmlFor={languageId} className="mt-6">
          {t('create.sourceLanguage', { ns: 'voices' })}
        </FieldLabel>
        <Listbox<ContentLanguage>
          id={languageId}
          aria-labelledby={`${languageId}-label`}
          value={sourceLanguage}
          options={[
            { value: 'ko', label: t('contentLanguage.ko', { ns: 'common' }) },
            { value: 'en', label: t('contentLanguage.en', { ns: 'common' }) },
          ]}
          disabled={create.isPending}
          aria-describedby={languageHintId}
          onChange={setSourceLanguage}
          className="mt-1"
        />
        <Typography
          variant="body"
          as="p"
          id={languageHintId}
          className="text-content-secondary mt-2"
        >
          {t('create.sourceLanguageHelp', { ns: 'voices' })}
        </Typography>

        <FieldLabel htmlFor={descriptionId} className="mt-6">
          {t('create.description', { ns: 'voices' })}
        </FieldLabel>
        <Textarea
          id={descriptionId}
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder={t('create.descriptionPlaceholder', { ns: 'voices' })}
          rows={3}
          autoGrow
          disabled={create.isPending}
          aria-invalid={describedExceeded || undefined}
          aria-describedby={`${descriptionCountId} ${descriptionHintId}`}
          className="max-h-field mt-1"
        />
        {describedExceeded ? (
          <FieldMessage id={descriptionCountId} role="status" className="mt-2">
            {t('count.exceeded', {
              ns: 'common',
              count: describedChars - VOICE_DESCRIPTION_MAX_CHARS,
            })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p" id={descriptionCountId} className="mt-2">
            {t('create.descriptionCount', {
              ns: 'voices',
              count: describedChars,
              max: VOICE_DESCRIPTION_MAX_CHARS,
            })}
          </Typography>
        )}
        <Typography
          variant="body"
          as="p"
          id={descriptionHintId}
          className="text-content-secondary mt-1"
        >
          {modelMissing
            ? t('create.descriptionNeedsModel', { ns: 'voices' })
            : t('create.descriptionHelp', { ns: 'voices' })}
        </Typography>

        {create.isError && (
          <FieldMessage id={errorId} className="mt-3">
            {create.errorMessage}
          </FieldMessage>
        )}
        {/* In flow after the last field, NOT in the sheet's pinned footer: the panel is anchored
            to the layout viewport, which the software keyboard does not resize, so a pinned
            footer sits behind the keyboard exactly while these fields are being typed into
            (design-language §8.3). */}
        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={create.isPending} onClick={onClose}>
            {t('action.cancel', { ns: 'common' })}
          </Button>
          <Button type="submit" variant="cta" disabled={disabled} pending={create.isPending}>
            {t('create.submit', { ns: 'voices' })}
          </Button>
        </div>
      </form>
    </Sheet>
  )
}
