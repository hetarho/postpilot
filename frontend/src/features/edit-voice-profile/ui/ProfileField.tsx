import i18next from 'i18next'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { VoiceValue } from '@/entities/voice'
import { voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice'
import { appFailureFromConnect, VoiceLayer, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { Badge, Button, Editable, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'

/** One overridable profile field, read first. The published value is prose that wraps to as many
 *  lines as it needs; the form control exists only while the owner is actually editing.
 *
 *  Each field owns its own mutation so the fields stay independent: a save in one must not put the
 *  other four into a pending state, and a rejected save must show its message under its own field. */
export function ProfileField({
  ownerId,
  voiceId,
  label,
  layer,
  field,
  value,
  readOnly = false,
}: {
  ownerId: string
  voiceId: string
  label: string
  layer: VoiceLayer
  field: string
  value: VoiceValue
  readOnly?: boolean
}) {
  const { t } = useTranslation(['voices', 'common'])
  const transport = useTransport()
  const queryClient = useQueryClient()
  const override = useMutation(VoiceService.method.updateVoiceOverride)
  // An override publishes a new whole-profile version, so the version list is stale too.
  const refresh = async () => {
    await queryClient.invalidateQueries({
      queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
    })
    await queryClient.invalidateQueries({
      queryKey: voiceVersionsQueryKey(transport, ownerId, voiceId),
    })
  }
  const commit = async (exit: () => void, next?: string) => {
    try {
      await override.mutateAsync({ voiceId, layer, field, value: next })
      await refresh()
      // Only a successful save leaves edit mode. A rejected one keeps the draft on screen, because
      // discarding the owner's text is a worse outcome than showing the server's message twice.
      exit()
    } catch {
      // The mutation's message renders under the field.
    }
  }
  return (
    <Editable
      editLabel={t('action.editNamed', { ns: 'common', name: label })}
      readOnly={readOnly}
      edit={(exit) => (
        <ProfileFieldEditor
          label={label}
          value={value}
          pending={override.isPending}
          errorMessage={
            override.error ? formatAppFailure(appFailureFromConnect(override.error)) : undefined
          }
          showClear={value.source === 'manual'}
          onSave={(next) => commit(exit, next)}
          onClear={() => commit(exit)}
          onCancel={() => {
            // The mutation outlives this editor, so its rejection would greet the owner again the
            // next time they press the pencil — on text they already chose to discard.
            override.reset()
            exit()
          }}
        />
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <FieldLabel>{label}</FieldLabel>
        <Badge tone={value.source === 'manual' ? 'info' : 'neutral'}>
          {value.unknown ? t('profile.unknown', { ns: 'voices' }) : sourceLabel(value.source)}
        </Badge>
      </div>
      {/* `break-words`: a Korean description has no spaces to break at, so without it a long value
          pushes the grid column wider than the screen (§3.2). */}
      <p className="mt-1 text-sm leading-relaxed break-words">
        {value.unknown || value.value.trim() === '' ? (
          <span className="text-content-tertiary">{t('profile.unknown', { ns: 'voices' })}</span>
        ) : (
          value.value
        )}
      </p>
    </Editable>
  )
}

function ProfileFieldEditor({
  label,
  value,
  pending,
  errorMessage,
  showClear,
  onSave,
  onClear,
  onCancel,
}: {
  label: string
  value: VoiceValue
  pending: boolean
  errorMessage?: string
  showClear: boolean
  onSave: (value: string) => void
  onClear: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const errorId = `${id}-error`
  // Seeded once, on mount: this component exists only while the field is in edit mode, so 취소 —
  // which unmounts it — is what discards the draft, and reopening starts from the published value.
  const [draft, setDraft] = useState(value.unknown ? '' : value.value)
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Textarea
        id={id}
        rows={2}
        autoGrow
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        aria-invalid={errorMessage ? true : undefined}
        aria-describedby={errorMessage ? errorId : undefined}
        // Capped per §4.4's in-form rule: past the cap the field scrolls itself rather than pushing
        // 저장 off the screen the caret is on.
        className="max-h-field mt-1 leading-relaxed"
      />
      {errorMessage && (
        <FieldMessage id={errorId} className="mt-2">
          {errorMessage}
        </FieldMessage>
      )}
      <div className="mt-2 flex flex-wrap gap-2">
        <Button
          variant="secondary"
          disabled={!draft.trim()}
          pending={pending}
          onClick={() => onSave(draft.trim())}
        >
          {t('action.save', { ns: 'common' })}
        </Button>
        <Button variant="ghost" disabled={pending} onClick={onCancel}>
          {t('action.cancel', { ns: 'common' })}
        </Button>
        {showClear && (
          <Button variant="ghost" disabled={pending} onClick={onClear}>
            {t('profile.clearManual', { ns: 'voices' })}
          </Button>
        )}
      </div>
    </div>
  )
}

const sourceLabel = (source: VoiceValue['source']) =>
  i18next.t(`source.${source}`, { ns: 'voices' })
