import { useId, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import {
  AppFailureMessage,
  Checkbox,
  FieldLabel,
  Listbox,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { appFailureFromConnect } from '@/shared/api'
export function ClipVoiceChoice({
  ownerId,
  enabled,
  voiceId,
  disabled,
  onChange,
}: {
  ownerId: string
  enabled: boolean
  voiceId: string
  disabled?: boolean
  onChange: (value: { enabled: boolean; voiceId: string }) => void
}) {
  const { t } = useTranslation('clips'),
    id = useId(),
    library = useSpokenLibrary(enabled || voiceId ? ownerId : '', false, false),
    [error, setError] = useState<unknown>()
  const voice = library.voices.find((v) => v.id === voiceId),
    available = library.voices.filter((v) => !v.removedAt)
  return (
    <div className="min-w-0 space-y-3">
      <fieldset disabled={disabled} className="min-w-0 space-y-3">
        <FieldLabel className="flex items-center gap-2">
          <Checkbox
            checked={enabled}
            onChange={(event) => onChange({ enabled: event.target.checked, voiceId })}
          />
          {t('dubbing.enabled')}
        </FieldLabel>
        {(enabled || voiceId) && (
          <>
            <FieldLabel htmlFor={id}>{t('dubbing.voice')}</FieldLabel>
            <Listbox
              id={id}
              value={voiceId}
              placeholder={t('dubbing.choose')}
              options={available.map((v) => ({ value: v.id, label: v.name }))}
              onChange={(voiceId) => onChange({ enabled, voiceId })}
            />
          </>
        )}
      </fieldset>
      {voice && (
        <SpokenSamplePlayer
          ownerId={ownerId}
          assetId={voice.sampleAssetId}
          name={voice.name}
          durationMs={voice.sampleDurationMs}
          onFailure={setError}
        />
      )}
      {enabled && !voice && voiceId && (
        <Typography variant="meta">{t('dubbing.voiceUnavailable')}</Typography>
      )}
      {library.voicesQuery.error && (
        <AppFailureMessage failure={appFailureFromConnect(library.voicesQuery.error)} />
      )}
      {error !== undefined && <AppFailureMessage failure={appFailureFromConnect(error)} />}
      <Link to="/spoken-voices" className={buttonStyles({ variant: 'ghost' })}>
        {t('editorEntries.voices')}
      </Link>
      <Link to="/spoken-voices/new" className={buttonStyles({ variant: 'ghost' })}>
        {t('dubbing.createVoice')}
      </Link>
    </div>
  )
}
