import { useId, useState } from 'react'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { VoiceValue } from '@/entities/voice-profile'
import { voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice-profile'
import { VoiceLayer, VoiceService } from '@/shared/api'
import { Badge, Button, Editable, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'

/** One overridable profile field, read first. The published value is prose that wraps to as many
 *  lines as it needs; the form control exists only while the owner is actually editing.
 *
 *  Each field owns its own mutation so the fields stay independent: a save in one must not put the
 *  other four into a pending state, and a rejected save must show its message under its own field. */
export function ProfileField({
  ownerId,
  label,
  layer,
  field,
  value,
}: {
  ownerId: string
  label: string
  layer: VoiceLayer
  field: string
  value: VoiceValue
}) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const override = useMutation(VoiceService.method.updateVoiceOverride)
  // An override publishes a new whole-profile version, so the version list is stale too.
  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) })
    await queryClient.invalidateQueries({ queryKey: voiceVersionsQueryKey(transport, ownerId) })
  }
  const commit = async (exit: () => void, next?: string) => {
    try {
      await override.mutateAsync({ layer, field, value: next })
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
      editLabel={`${label} 수정`}
      edit={(exit) => (
        <ProfileFieldEditor
          label={label}
          value={value}
          pending={override.isPending}
          errorMessage={override.error?.message}
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
          {value.unknown ? '알 수 없음' : sourceLabel(value.source)}
        </Badge>
      </div>
      {/* `break-words`: a Korean description has no spaces to break at, so without it a long value
          pushes the grid column wider than the screen (§3.2). */}
      <p className="mt-1 text-sm leading-relaxed break-words">
        {value.unknown || value.value.trim() === '' ? (
          <span className="text-content-tertiary">알 수 없음</span>
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
          저장
        </Button>
        <Button variant="ghost" disabled={pending} onClick={onCancel}>
          취소
        </Button>
        {showClear && (
          <Button variant="ghost" disabled={pending} onClick={onClear}>
            직접 설정 해제
          </Button>
        )}
      </div>
    </div>
  )
}

const sourceLabel = (source: VoiceValue['source']) =>
  ({ measured: '측정', analyzed: '분석', manual: '직접 설정', unknown: '알 수 없음' })[source]
