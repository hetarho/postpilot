import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ClipSpeechPlayer,
  spokenState,
  SPOKEN_LIMITS,
  type ClipNarration,
  type ClipSpokenSegment,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { Button, FieldCount, FieldLabel, Textarea, TextField, Typography } from '@/shared/ui'
export function ClipSpokenProperties({
  ownerId,
  projectId,
  narration,
  segment,
  durationMs,
  disabled,
  change,
}: {
  ownerId: string
  projectId: string
  narration: ClipNarration
  segment: ClipSpokenSegment
  durationMs: number
  disabled?: boolean
  change: (edit: TimelineEdit, group?: string) => void
}) {
  const { t } = useTranslation('clips'),
    id = useId(),
    [split, setSplit] = useState(0),
    state = spokenState(narration, segment, durationMs)
  return (
    <div className="min-w-0 space-y-3">
      <Typography variant="meta" role="status">
        {t(`dubbing.states.${state}`)}
      </Typography>
      <fieldset disabled={disabled} className="min-w-0 space-y-3">
        <FieldLabel htmlFor={id}>{t('dubbing.words')}</FieldLabel>
        <Textarea
          id={id}
          rows={3}
          autoGrow
          value={segment.text}
          onSelect={(e) =>
            setSplit(
              Array.from(e.currentTarget.value.slice(0, e.currentTarget.selectionStart)).length,
            )
          }
          onChange={(e) =>
            change(
              { type: 'spokenPatch', id: segment.id, patch: { text: e.target.value } },
              `${segment.id}:words`,
            )
          }
        />
        <FieldCount left={SPOKEN_LIMITS.segmentCharacters - Array.from(segment.text).length} />
        <div className="grid grid-cols-2 gap-3">
          {(['startMs', 'endMs'] as const).map((key) => (
            <div key={key}>
              <FieldLabel htmlFor={`${id}-${key}`}>
                {t(key === 'startMs' ? 'dubbing.start' : 'dubbing.end')}
              </FieldLabel>
              <TextField
                id={`${id}-${key}`}
                type="number"
                inputMode="decimal"
                step="0.001"
                min="0"
                value={segment[key] / 1000}
                onChange={(e) => {
                  const seconds = Number(e.target.value)
                  if (Number.isFinite(seconds))
                    change(
                      {
                        type: 'spokenPatch',
                        id: segment.id,
                        patch: { [key]: Math.round(seconds * 1000) },
                      },
                      `${segment.id}:${key}`,
                    )
                }}
              />
            </div>
          ))}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            disabled={
              split <= 0 ||
              split >= Array.from(segment.text).length ||
              narration.segments.length >= 32
            }
            onClick={() =>
              change({
                type: 'splitSpoken',
                id: segment.id,
                newId: `new-spoken-${crypto.randomUUID()}`,
                character: split,
              })
            }
          >
            {t('dubbing.split')}
          </Button>
          <Button variant="danger" onClick={() => change({ type: 'removeSpoken', id: segment.id })}>
            {t('dubbing.remove')}
          </Button>
        </div>
      </fieldset>
      {segment.speech && (
        <>
          <Typography variant="meta">
            {t(state === 'stale' ? 'dubbing.previousAudioRevision' : 'dubbing.audioRevision', {
              revision: segment.textRevision,
            })}
          </Typography>
          <ClipSpeechPlayer
            ownerId={ownerId}
            projectId={projectId}
            speech={segment.speech}
            previous={state === 'stale'}
          />
        </>
      )}
    </div>
  )
}
