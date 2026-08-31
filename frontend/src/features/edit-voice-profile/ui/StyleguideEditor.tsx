import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useUpdateVoiceProfile } from '@/entities/voice'
import { Button, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'

export function StyleguideEditor({
  ownerId,
  voiceId,
  styleguide,
  readOnly = false,
}: {
  ownerId: string
  voiceId: string
  styleguide: string
  readOnly?: boolean
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  const [draft, setDraft] = useState<string | null>(null)
  const update = useUpdateVoiceProfile(ownerId, voiceId)
  const value = draft ?? styleguide
  const dirty = draft !== null

  const save = async () => {
    const submitted = value
    try {
      await update.saveStyleguide(submitted)
      setDraft((current) => (current === submitted ? null : current))
    } catch {
      // The mutation error is rendered below.
    }
  }

  return (
    <section>
      <h2 className="text-lg font-semibold tracking-tight">
        {t('profile.styleguide', { ns: 'voices' })}
      </h2>
      <p className="text-content-tertiary mt-1 text-xs">
        {t('profile.styleguideHelp', { ns: 'voices' })}
      </p>
      <FieldLabel htmlFor={id} className="mt-4">
        {t('profile.styleguide', { ns: 'voices' })}
      </FieldLabel>
      <Textarea
        id={id}
        disabled={readOnly}
        value={value}
        onChange={(event) => {
          const next = event.target.value
          setDraft(next === styleguide ? null : next)
        }}
        // A generated styleguide is longer than any fixed box, so the field grows with it rather
        // than owning every vertical swipe that lands on it (§4.4). `max-h-field` is the ceiling:
        // past it the field becomes its own bounded scroller, because an uncapped one would put the
        // 저장 below it thousands of pixels from the caret (§4.3). `rows` is the empty minimum.
        rows={6}
        autoGrow
        aria-invalid={update.isError || undefined}
        aria-describedby={update.isError ? errorId : undefined}
        className="max-h-field mt-1 leading-relaxed"
      />
      {update.isError && (
        <FieldMessage id={errorId} className="mt-2">
          {update.errorMessage}
        </FieldMessage>
      )}
      {/* 저장 goes AFTER the field it commits (§4.3). In the section header it sat ~377px above
          the bottom of the styleguide — off the top of the screen exactly when the keyboard is up
          and the user is typing into it, and out of thumb reach even when it is not. There is no
          autosave here, so an unreachable 저장 is a lost edit. */}
      <div className="mt-4 flex justify-end">
        <Button
          variant="secondary"
          onClick={() => void save()}
          disabled={readOnly || !dirty}
          pending={update.isSaving}
          className="w-full sm:w-auto"
        >
          {t('action.save', { ns: 'common' })}
        </Button>
      </div>
      <span role="status" className="sr-only">
        {update.isSaving ? t('profile.savingStyleguide', { ns: 'voices' }) : ''}
      </span>
    </section>
  )
}
