import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipNoticeList, type ClipNotice, type ClipProjectRegion } from '@/entities/clip-project'
import type { ClipRegionKind } from '@/entities/clip-design'
import {
  ActionMenu,
  Badge,
  FieldLabel,
  FieldMessage,
  Notice,
  Switch,
  Textarea,
  TextField,
  Typography,
  type BadgeTone,
} from '@/shared/ui'
import { slotState, type ClipSlotEdit, type ClipSlotState } from '../model/region-draft'

const STATE_TONES: Record<ClipSlotState, BadgeTone> = {
  owner: 'accent',
  blank: 'neutral',
  bound: 'info',
  written: 'success',
  awaiting: 'warning',
}

/** One intro or outro block of ②'s storyline space (CLIP-179, CLIP-186): whether it is used and
 *  in which preset, the renderer's numbered drawing of that preset's slots (CLIP-165), and each
 *  active slot's writing instruction beside its final display text. Words past the preset's
 *  slots are unused and can be moved into one (CLIP-189). A block that is off says so, keeps
 *  every draft, and turns back on without a template. */
export function ClipRegionBlock({
  kind,
  region,
  capacity,
  presetName,
  diagram,
  canvas,
  drawable,
  errors,
  notices,
  readOnly,
  onEnabled,
  onSlot,
  onMove,
}: {
  kind: ClipRegionKind
  region: ClipProjectRegion
  capacity: number
  presetName: string
  /** The renderer's own drawing of the preset's numbered slots, once it has arrived. */
  diagram?: string
  canvas: { width: number; height: number }
  /** Whether any active slot draws text: an enabled block that draws none is unresolved. */
  drawable: boolean
  /** A refused slot's message, by slot id; the refused words stay in the field. */
  errors: Readonly<Record<string, string>>
  notices: (slotId: string) => readonly ClipNotice[]
  readOnly: boolean
  onEnabled: (enabled: boolean) => void
  onSlot: (slotId: string, change: ClipSlotEdit) => void
  onMove: (fromSlotId: string, toSlotId: string) => void
}) {
  const { t } = useTranslation('clips')
  const heading = useId()
  const name = t(`regions.${kind}`)
  const active = region.slots.slice(0, capacity)
  const unused = region.slots.slice(capacity).filter((slot) => slot.text.trim() !== '')
  return (
    <section aria-labelledby={heading} className="py-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <Typography variant="label" as="h3" id={heading}>
          {name}
        </Typography>
        <Typography variant="meta" as="span" className="text-content-secondary min-w-0 flex-1">
          {region.enabled ? presetName : t('regions.off')}
        </Typography>
        {!readOnly && (
          <Switch
            aria-label={t('regions.use', { region: name })}
            checked={region.enabled}
            onChange={(event) => onEnabled(event.currentTarget.checked)}
          />
        )}
      </div>
      {region.enabled && (
        <>
          {diagram && (
            <span
              aria-hidden="true"
              className="bg-media-canvas-bg mt-3 block w-24 overflow-hidden rounded-sm"
              style={{ aspectRatio: `${canvas.width} / ${canvas.height}` }}
            >
              <svg
                viewBox={`0 0 ${canvas.width} ${canvas.height}`}
                className="block h-full w-full"
                /* The renderer's drawing of the chosen preset's numbered slots, the same one
                   ① shows beside its name (CLIP-165). */
                dangerouslySetInnerHTML={{ __html: diagram }}
              />
            </span>
          )}
          {!drawable && (
            <Notice tone="warning" className="mt-3">
              {t('regions.nothingDrawn')}
            </Notice>
          )}
          <ol className="mt-3 space-y-4">
            {active.map((slot, index) => {
              const n = index + 1
              const state = slotState(slot)
              const error = errors[slot.id]
              return (
                <li key={slot.id} className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Typography variant="label" as="p" className="text-content-secondary">
                      {t('regions.slot', { n })}
                    </Typography>
                    <Badge tone={STATE_TONES[state]}>{t(`regions.state.${state}`)}</Badge>
                  </div>
                  <div>
                    <FieldLabel htmlFor={`${slot.id}-instruction`}>
                      {t('regions.instruction', { n })}
                    </FieldLabel>
                    <Textarea
                      id={`${slot.id}-instruction`}
                      autoGrow
                      rows={1}
                      readOnly={readOnly}
                      value={slot.instruction}
                      onChange={(event) =>
                        onSlot(slot.id, { instruction: event.currentTarget.value })
                      }
                    />
                  </div>
                  <div>
                    <FieldLabel htmlFor={`${slot.id}-text`}>{t('regions.text', { n })}</FieldLabel>
                    <TextField
                      id={`${slot.id}-text`}
                      readOnly={readOnly}
                      aria-invalid={error ? true : undefined}
                      aria-describedby={error ? `${slot.id}-error` : undefined}
                      value={slot.text}
                      onChange={(event) => onSlot(slot.id, { text: event.currentTarget.value })}
                    />
                    {error && <FieldMessage id={`${slot.id}-error`}>{error}</FieldMessage>}
                  </div>
                  <ClipNoticeList notices={notices(slot.id)} />
                </li>
              )
            })}
          </ol>
          {unused.length > 0 && (
            <section aria-label={t('regions.unused')} className="mt-4 space-y-3">
              <Typography variant="label" as="h4">
                {t('regions.unused')}
              </Typography>
              {unused.map((slot, index) => (
                <div key={slot.id} className="space-y-1">
                  <div className="flex items-start gap-2">
                    <TextField
                      className="min-w-0 flex-1"
                      aria-label={t('regions.unusedText', { n: index + 1 })}
                      readOnly={readOnly}
                      value={slot.text}
                      onChange={(event) => onSlot(slot.id, { text: event.currentTarget.value })}
                    />
                    {!readOnly && (
                      <ActionMenu
                        label={t('regions.moveUnused', { n: index + 1 })}
                        triggerLabel={t('regions.move')}
                        items={active.map((target, at) => ({
                          id: target.id,
                          label: t('regions.moveTo', { n: at + 1 }),
                          onSelect: () => onMove(slot.id, target.id),
                        }))}
                      />
                    )}
                  </div>
                  <ClipNoticeList notices={notices(slot.id)} />
                </div>
              ))}
            </section>
          )}
        </>
      )}
    </section>
  )
}
