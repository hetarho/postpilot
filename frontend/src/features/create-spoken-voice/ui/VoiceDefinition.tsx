import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { SpeechProfilePicker, type SpeechProfileChoice } from '@/entities/model-catalog'
import {
  SPOKEN_NAME_MAX,
  SPOKEN_DESCRIPTION_MAX,
  SPOKEN_PREVIEW_MAX,
  type SpokenDraftInput,
} from '@/entities/spoken-voice'
import { FieldLabel, TextField, Textarea, Typography } from '@/shared/ui'
export function VoiceDefinition({
  input,
  profiles,
  onChange,
  disabled,
}: {
  input: SpokenDraftInput
  profiles: readonly SpeechProfileChoice[]
  onChange: (patch: Partial<SpokenDraftInput>) => void
  disabled: boolean
}) {
  const { t } = useTranslation('createSpokenVoice'),
    id = useId(),
    profile = profiles.find((p) => p.id === input.profileId && p.revision === input.profileRevision)
  return (
    <div className="space-y-6">
      <div>
        <FieldLabel htmlFor={`${id}-name`}>{t('name')}</FieldLabel>
        <TextField
          id={`${id}-name`}
          value={input.name}
          maxLength={SPOKEN_NAME_MAX}
          disabled={disabled}
          onChange={(e) => onChange({ name: e.target.value })}
          autoComplete="off"
        />
      </div>
      <div>
        <FieldLabel htmlFor={`${id}-description`}>{t('description')}</FieldLabel>
        <Textarea
          id={`${id}-description`}
          value={input.description}
          disabled={disabled}
          maxLength={profile?.descriptionMax ?? SPOKEN_DESCRIPTION_MAX}
          onChange={(e) => onChange({ description: e.target.value })}
          aria-describedby={`${id}-description-hint`}
        />
        <Typography id={`${id}-description-hint`} variant="meta">
          {t('descriptionHint')}
        </Typography>
      </div>
      <fieldset disabled={disabled} className="min-w-0">
        <SpeechProfilePicker
          profiles={profiles}
          selectedId={profile?.id ?? null}
          onSelect={(p) => onChange({ profileId: p?.id ?? '', profileRevision: p?.revision ?? 0n })}
        />
      </fieldset>
      <div>
        <FieldLabel htmlFor={`${id}-preview`}>{t('preview')}</FieldLabel>
        <Textarea
          id={`${id}-preview`}
          value={input.previewText}
          disabled={disabled}
          maxLength={profile?.previewMax ?? SPOKEN_PREVIEW_MAX}
          onChange={(e) => onChange({ previewText: e.target.value })}
          aria-describedby={`${id}-preview-hint`}
        />
        <Typography id={`${id}-preview-hint`} variant="meta">
          {t('previewHint')}
        </Typography>
      </div>
    </div>
  )
}
