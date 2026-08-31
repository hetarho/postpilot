import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useUpdateVoiceProfile } from '@/entities/voice'
import { Button, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'

export function RulesEditor({
  ownerId,
  voiceId,
  rules,
  readOnly = false,
}: {
  ownerId: string
  voiceId: string
  rules: string
  readOnly?: boolean
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  const [draft, setDraft] = useState<string | null>(null)
  const update = useUpdateVoiceProfile(ownerId, voiceId)
  const value = draft ?? rules
  const dirty = draft !== null

  const save = async () => {
    const submitted = value
    try {
      await update.saveRules(submitted)
      setDraft((current) => (current === submitted ? null : current))
    } catch {
      // The mutation error is rendered below.
    }
  }

  return (
    <section>
      <h2 className="text-lg font-semibold tracking-tight">
        {t('profile.customRules', { ns: 'voices' })}
      </h2>
      <p className="text-content-tertiary mt-1 text-xs">
        {t('profile.customRulesHelp', { ns: 'voices' })}
      </p>
      <FieldLabel htmlFor={id} className="mt-4">
        {t('profile.customRules', { ns: 'voices' })}
      </FieldLabel>
      <Textarea
        id={id}
        disabled={readOnly}
        value={value}
        onChange={(event) => {
          const next = event.target.value
          setDraft(next === rules ? null : next)
        }}
        // Grows with the value rather than scrolling inside itself: the page is the one scroller
        // (§4.4). `rows` is now just the minimum height of an empty editor.
        rows={4}
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
      {/* 저장 goes AFTER the field it commits (§4.3): from the section header it was ~240px above
          the bottom of the rules field, which is off-screen while the keyboard is up. This is the
          last section on a ~1900px page, so scrolling back up to reach it costs the most here. */}
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
        {update.isSaving ? t('profile.savingRules', { ns: 'voices' }) : ''}
      </span>
    </section>
  )
}
