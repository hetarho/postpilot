import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refKey, type ModelRef } from '@/entities/model-catalog'
import type { PostImage } from '@/entities/image'
import type { Observation } from '@/shared/api'
import { Button, Checkbox, Dialog, Notice, Typography } from '@/shared/ui'
import {
  defaultSelection,
  reobserveRows,
  storedObservationModels,
  type ReobserveRow,
} from '../model/reobserve'

interface ReobservePickerProps {
  open: boolean
  images: readonly PostImage[]
  observations: readonly Observation[]
  /** The observation model this run will use, for the changed-model notice. */
  observeModel?: ModelRef
  pending?: boolean
  /** The filenames to observe again. An EMPTY array is a real answer — reuse everything. */
  onConfirm: (files: string[]) => void
  onCancel: () => void
}

/** Which photos the next run observes again, decided before it is enqueued.
 *
 *  ONE surface on ONE mount, in two presentations: `Dialog` is `Sheet`, which is a bottom sheet on
 *  a phone and a centred dialog from `md:` up — purely in CSS. Mounting two overlays instead would
 *  be actively wrong, because a sheet locks the body scroll and traps focus from its first render.
 *
 *  Everything the decision MEANS lives in `model/reobserve.ts`; this file only renders it. */
export function ReobservePicker({
  open,
  images,
  observations,
  observeModel,
  pending = false,
  onConfirm,
  onCancel,
}: ReobservePickerProps) {
  const { t } = useTranslation('posts')
  const rows = reobserveRows(images, observations)
  const [selected, setSelected] = useState<readonly string[]>(() => defaultSelection(rows))
  // The defaults are recomputed on OPENING, not on every render, so a photo attached or an
  // observation arriving while the picker is up cannot silently move the user's checkboxes.
  // Adjusted during render rather than in an effect — the same pattern `Sheet` uses for its own
  // `present` — because an effect here would render once with the previous answer's checkboxes.
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setSelected(defaultSelection(rows))
  }

  const forced = defaultSelection(rows)
  // The ONE answer the checkboxes, the count and the confirm all read. A forced row belongs to
  // the selection whether or not `selected` has caught up with it — an upload confirming while
  // the picker is open adds a forced row the seeded state has never seen — and deriving it here
  // is what stops the dialog from showing a checked row that its own count and confirmed set
  // leave out.
  const effective = rows
    .filter((row) => row.forced || selected.includes(row.filename))
    .map((row) => row.filename)
  const storedModels = storedObservationModels(rows)
  const selectedRef = observeModel ? refKey(observeModel) : ''
  // Only a KNOWN provenance can differ. An entry written before the model was recorded says
  // nothing about which model produced it, so claiming a difference would fire this warning on
  // every observation that predates the field — which is all of them.
  const knownStoredModels = storedModels.filter((model) => model !== '')
  const changedModel =
    selectedRef !== '' && knownStoredModels.length > 0 && !knownStoredModels.includes(selectedRef)

  const toggle = (filename: string, checked: boolean) =>
    setSelected((current) =>
      checked
        ? current.includes(filename)
          ? current
          : [...current, filename]
        : current.filter((name) => name !== filename),
    )

  // Confirm sends the checked names in POST order, so the frozen set reads the way the picker
  // looked. The server re-derives the forced photos regardless of what arrives here.
  const confirm = () => onConfirm(effective)

  const modelNames = (refs: readonly string[]) =>
    refs.map((ref) => ref || t('generation.reobserve.storedModelUnknown')).join(', ')

  return (
    <Dialog
      open={open}
      title={t('generation.reobserve.title')}
      confirmLabel={t('generation.reobserve.confirm')}
      onConfirm={confirm}
      onClose={onCancel}
      pending={pending}
    >
      <Typography variant="body" as="p" className="text-content-secondary">
        {t('generation.reobserve.purpose')}
      </Typography>
      {storedModels.length > 0 && (
        <Typography variant="label" as="p" className="text-content-tertiary mt-2">
          {t('generation.reobserve.storedModel', { models: modelNames(storedModels) })}
        </Typography>
      )}
      {changedModel && (
        <Notice tone="warning" className="mt-3">
          {t('generation.reobserve.modelChanged', {
            selected: selectedRef,
            stored: modelNames(knownStoredModels),
          })}
        </Notice>
      )}

      {/* The two bulk controls and the running count on one row. Clearing everything is the
          reuse-everything answer, so it is as reachable as selecting everything. */}
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          variant="secondary"
          onClick={() => setSelected(rows.map((row) => row.filename))}
          disabled={pending}
        >
          {t('generation.reobserve.selectAll')}
        </Button>
        <Button variant="secondary" onClick={() => setSelected(forced)} disabled={pending}>
          {t('generation.reobserve.clearAll')}
        </Button>
        <Typography variant="label" as="p" role="status" className="text-content-tertiary min-w-0">
          {effective.length === 0
            ? t('generation.reobserve.selectedNone')
            : t('generation.reobserve.selectedCount', { count: effective.length })}
        </Typography>
      </div>

      {/* Rows, not cards: they are peers in a list and carry no surface of their own, so the
          hairline is the §1.3 divider exception rather than a plane change per photo. */}
      <ul className="divide-divider mt-4 divide-y">
        {rows.map((row) => (
          <PhotoRow
            key={row.filename}
            row={row}
            checked={effective.includes(row.filename)}
            disabled={pending}
            onChange={(checked) => toggle(row.filename, checked)}
          />
        ))}
      </ul>
    </Dialog>
  )
}

function PhotoRow({
  row,
  checked,
  disabled,
  onChange,
}: {
  row: ReobserveRow
  checked: boolean
  disabled: boolean
  onChange: (checked: boolean) => void
}) {
  const { t } = useTranslation('posts')
  // A `blob:` preview means the photo was attached since the last observation and has no
  // presigned view URL yet. Only the capability minted by `GetPost` may fetch an R2 thumbnail,
  // so the local URL is not used here — and such a photo is forced anyway.
  const viewUrl = row.image.viewUrl.startsWith('blob:') ? '' : row.image.viewUrl
  return (
    <li className="flex items-start gap-3 py-3">
      {viewUrl ? (
        <img
          src={viewUrl}
          alt={t('observation.imageAlt', { filename: row.filename })}
          width={row.image.width}
          height={row.image.height}
          loading="lazy"
          decoding="async"
          className="size-14 shrink-0 rounded-md object-cover"
        />
      ) : (
        <span aria-hidden="true" className="bg-surface-recessed size-14 shrink-0 rounded-md" />
      )}
      <div className="min-w-0 flex-1">
        <Typography variant="label" as="p" className="text-content-primary truncate">
          {row.filename}
        </Typography>
        <Typography variant="body" as="p" className="text-content-secondary mt-1">
          {row.stored
            ? [row.stored.scene, row.stored.mood, row.stored.visibleText]
                .filter(Boolean)
                .join(' · ')
            : t('generation.reobserve.nothingStored')}
        </Typography>
        {row.forced && (
          <Typography variant="label" as="p" className="text-content-tertiary mt-1">
            {t('generation.reobserve.forcedReason')}
          </Typography>
        )}
      </div>
      <Checkbox
        className="mt-1"
        checked={checked}
        // A forced photo states its reason above and cannot be cleared (§7): the run must not
        // write from a photo nothing has ever looked at, and the server enforces the same rule.
        disabled={disabled || row.forced}
        aria-label={t('generation.reobserve.observeAgain', { filename: row.filename })}
        onChange={(event) => onChange(event.currentTarget.checked)}
      />
    </li>
  )
}
