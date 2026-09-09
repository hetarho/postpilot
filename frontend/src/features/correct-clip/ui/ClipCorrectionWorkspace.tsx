import { useRef, useState, type ReactNode } from 'react'
import { useBlocker } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { COPY_POSITIONS, type ClipEditCut, type ClipEditingState } from '@/entities/clip-project'
import { CLIP_ACCENTS, CopyStylePreview } from '@/entities/clip-template'
import type { AppFailure } from '@/shared/api'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  SortableList,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import type { useClipCorrection } from '../model/useClipCorrection'

const INPUT = {
  autoComplete: 'off',
  autoCapitalize: 'off',
  autoCorrect: 'off',
  enterKeyHint: 'next',
} as const
type Correction = ReturnType<typeof useClipCorrection>

function CutNumber({
  id,
  label,
  value,
  min = 0,
  step = 1,
  max,
  error,
  onChange,
}: {
  id: string
  label: string
  value: number
  min?: number
  step?: number
  max?: number
  error?: string
  onChange: (value: number) => void
}) {
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <TextField
        id={id}
        type="number"
        inputMode={step < 1 ? 'decimal' : 'numeric'}
        {...INPUT}
        step={step}
        min={min}
        max={max}
        value={Number.isFinite(value) ? value : ''}
        onChange={(e) => onChange(e.target.value === '' ? NaN : Number(e.target.value))}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : undefined}
      />
      {error && <FieldMessage id={`${id}-error`}>{error}</FieldMessage>}
    </div>
  )
}
function LocalCutPreview({ url, cut }: { url: string; cut: ClipEditCut }) {
  const { t } = useTranslation('clips')
  return (
    <video
      controls
      preload="metadata"
      src={url}
      aria-label={t('correction.sourcePreview')}
      className="aspect-video w-full rounded-md"
      onLoadedMetadata={(e) => {
        e.currentTarget.currentTime = Math.max(
          0,
          Number.isFinite(cut.startMs) ? cut.startMs / 1000 : 0,
        )
      }}
    />
  )
}
export function ClipCorrectionWorkspace({
  correction,
  state,
  disabled,
  renderReady,
  renderPending,
  renderFailure,
  onRender,
  onExit,
  sourcePicker,
  localSources,
}: {
  correction: Correction
  state: ClipEditingState
  disabled: boolean
  renderReady: boolean
  renderPending: boolean
  renderFailure?: AppFailure
  onRender: () => void
  onExit: () => void
  sourcePicker: ReactNode
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
}) {
  const { t } = useTranslation('clips')
  const [confirm, setConfirm] = useState<'exit' | 'reload' | 'delete'>()
  const [deleteId, setDeleteId] = useState('')
  const [inspect, setInspect] = useState('')
  const leaving = useRef(false)
  const blocker = useBlocker({
    shouldBlockFn: () => correction.dirty && !leaving.current,
    enableBeforeUnload: () => correction.dirty && !leaving.current,
    withResolver: true,
  })
  const busy = disabled || correction.pending
  const mutate = (...args: Parameters<Correction['change']>) => {
    if (!busy) correction.change(...args)
  }
  const failure = correction.failure ?? renderFailure
  return (
    <>
      <section aria-labelledby="clip-correction-heading" className="mt-10">
        <Typography variant="title" id="clip-correction-heading">
          {t('correction.title')}
        </Typography>
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('correction.help')}
        </Typography>
        <Typography variant="body" className="mt-3">
          {t('correction.total', {
            ms: Number.isFinite(correction.draft.durationMs) ? correction.draft.durationMs : '—',
          })}
        </Typography>
        {correction.validation?.timeline && (
          <FieldMessage>
            {t('correction.timelineError', { min: state.minDurationMs, max: state.maxDurationMs })}
          </FieldMessage>
        )}
        {correction.validation?.count && <FieldMessage>{t('correction.countError')}</FieldMessage>}
        <fieldset disabled={busy} className="mt-6 min-w-0">
          <SortableList
            disabled={busy}
            labels={{ drag: t('editor.drag'), up: t('editor.up'), down: t('editor.down') }}
            onReorder={(from, to) => mutate({ type: 'move', from, to })}
            items={correction.draft.cuts.map((cut, index) => {
              const source = state.sources.find((s) => s.id === cut.sourceId)
              const errors = correction.validation?.cuts[index]
              const prefix = `clip-cut-${cut.id}`
              const patch = (value: Parameters<Correction['change']>[0]) => mutate(value)
              const windowError = t('correction.captionWindowError')
              const rangeError = t('correction.rangeError', { max: source?.durationMs ?? 0 })
              const url = localSources.find((s) => s.fingerprint === cut.fingerprint)?.url
              return {
                id: cut.id,
                content: (
                  <section
                    aria-label={t('correction.cut', { number: index + 1 })}
                    className="space-y-6 py-3"
                  >
                    <div className="space-y-2">
                      <Typography variant="fieldTitle" as="h3">
                        {t('correction.cut', { number: index + 1 })}
                      </Typography>
                      <Typography variant="body" className="break-words">
                        {source?.filename ?? cut.sourceId}
                      </Typography>
                      {errors?.identity && (
                        <FieldMessage>{t('correction.sourceChanged')}</FieldMessage>
                      )}
                      <Button
                        variant="danger"
                        disabled={busy}
                        onClick={() => {
                          setDeleteId(cut.id)
                          setConfirm('delete')
                        }}
                      >
                        {t('correction.deleteCut')}
                      </Button>
                    </div>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                      <CutNumber
                        id={`${prefix}-start`}
                        label={t('correction.start')}
                        value={cut.startMs}
                        max={source?.durationMs}
                        error={errors?.start ? rangeError : undefined}
                        onChange={(startMs) =>
                          patch({ type: 'cut', id: cut.id, patch: { startMs } })
                        }
                      />
                      <CutNumber
                        id={`${prefix}-end`}
                        label={t('correction.end')}
                        value={cut.endMs}
                        max={source?.durationMs}
                        error={errors?.end ? rangeError : undefined}
                        onChange={(endMs) => patch({ type: 'cut', id: cut.id, patch: { endMs } })}
                      />
                      <CutNumber
                        id={`${prefix}-duration`}
                        label={t('correction.duration')}
                        value={cut.endMs - cut.startMs}
                        min={2 * state.fadeMs + 1}
                        max={(source?.durationMs ?? 0) - cut.startMs}
                        error={
                          errors?.duration
                            ? t('correction.cutDurationError', { min: 2 * state.fadeMs })
                            : undefined
                        }
                        onChange={(duration) =>
                          patch({
                            type: 'cut',
                            id: cut.id,
                            patch: { endMs: cut.startMs + duration },
                          })
                        }
                      />
                    </div>
                    {url && (
                      <Button
                        variant="secondary"
                        onClick={() => setInspect(inspect === cut.id ? '' : cut.id)}
                      >
                        {t('correction.inspect')}
                      </Button>
                    )}
                    {url && inspect === cut.id && (
                      <LocalCutPreview
                        key={`${cut.fingerprint}-${cut.startMs}`}
                        url={url}
                        cut={cut}
                      />
                    )}
                    <div>
                      <FieldLabel htmlFor={`${prefix}-copy`}>{t('correction.copy')}</FieldLabel>
                      <Textarea
                        id={`${prefix}-copy`}
                        autoGrow
                        inputMode="text"
                        {...INPUT}
                        autoCapitalize="sentences"
                        autoCorrect="on"
                        value={cut.copy.text}
                        aria-invalid={errors?.text}
                        aria-describedby={errors?.text ? `${prefix}-copy-error` : undefined}
                        onChange={(e) =>
                          patch({ type: 'copy', id: cut.id, patch: { text: e.target.value } })
                        }
                      />
                      {errors?.text && (
                        <FieldMessage id={`${prefix}-copy-error`}>
                          {t('validation.tooLong', { max: state.maxCopyRunes })}
                        </FieldMessage>
                      )}
                      <CopyStylePreview
                        style={cut.copy.style}
                        accent={cut.copy.accent}
                        text={cut.copy.text}
                      />
                    </div>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <CutNumber
                        id={`${prefix}-copy-start`}
                        label={t('correction.copyStart')}
                        value={cut.copy.startMs}
                        max={cut.endMs - cut.startMs}
                        error={errors?.copyStart ? windowError : undefined}
                        onChange={(startMs) =>
                          patch({ type: 'copy', id: cut.id, patch: { startMs } })
                        }
                      />
                      <CutNumber
                        id={`${prefix}-copy-end`}
                        label={t('correction.copyEnd')}
                        value={cut.copy.endMs}
                        max={cut.endMs - cut.startMs}
                        error={errors?.copyEnd ? windowError : undefined}
                        onChange={(endMs) => patch({ type: 'copy', id: cut.id, patch: { endMs } })}
                      />
                    </div>
                    <Typography variant="body" className="text-content-secondary">
                      {t('correction.captionWindowHelp')}
                    </Typography>
                    <div>
                      <FieldLabel id={`${prefix}-position-label`} htmlFor={`${prefix}-position`}>
                        {t('correction.position')}
                      </FieldLabel>
                      <Listbox
                        id={`${prefix}-position`}
                        aria-labelledby={`${prefix}-position-label`}
                        value={cut.copy.position}
                        disabled={busy}
                        onChange={(position) =>
                          patch({ type: 'copy', id: cut.id, patch: { position } })
                        }
                        options={COPY_POSITIONS.map((value) => ({
                          value,
                          label: t(`correction.positions.${value}`),
                        }))}
                      />
                    </div>
                    <div>
                      <FieldLabel id={`${prefix}-style-label`} htmlFor={`${prefix}-style`}>
                        {t('editor.styles')}
                      </FieldLabel>
                      <Listbox
                        id={`${prefix}-style`}
                        aria-labelledby={`${prefix}-style-label`}
                        value={cut.copy.style}
                        disabled={busy}
                        onChange={(style) => patch({ type: 'copy', id: cut.id, patch: { style } })}
                        options={state.copyStyles.map((value) => ({
                          value,
                          label: t(`style.${value}`),
                        }))}
                      />
                    </div>
                    <div>
                      <FieldLabel id={`${prefix}-accent-label`} htmlFor={`${prefix}-accent`}>
                        {t('editor.accent')}
                      </FieldLabel>
                      <Listbox
                        id={`${prefix}-accent`}
                        aria-labelledby={`${prefix}-accent-label`}
                        value={cut.copy.accent}
                        disabled={busy}
                        onChange={(accent) =>
                          patch({ type: 'copy', id: cut.id, patch: { accent } })
                        }
                        options={CLIP_ACCENTS.map((value) => ({
                          value,
                          label: t(`accent.${value || 'none'}`),
                        }))}
                      />
                    </div>
                    <CutNumber
                      id={`${prefix}-volume`}
                      label={t('correction.volume')}
                      value={cut.volumePermille / 10}
                      step={0.1}
                      max={100}
                      error={errors?.volume ? t('correction.volumeError') : undefined}
                      onChange={(percent) =>
                        patch({
                          type: 'cut',
                          id: cut.id,
                          patch: { volumePermille: Number((percent * 10).toFixed(8)) },
                        })
                      }
                    />
                  </section>
                ),
              }
            })}
          />
        </fieldset>
      </section>
      {sourcePicker}
      <ActionBar className="mt-auto" ariaLabel={t('correction.actions')}>
        {failure && (
          <div role="alert" className="mb-3">
            <AppFailureMessage failure={failure} />
            {failure.reason === 'CLIP_PLAN_CONFLICT' && (
              <Button variant="ghost" disabled={busy} onClick={() => setConfirm('reload')}>
                {t('correction.reload')}
              </Button>
            )}
          </div>
        )}
        <div className="flex flex-wrap justify-end gap-3">
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => (correction.dirty ? setConfirm('exit') : onExit())}
          >
            {t('correction.exit')}
          </Button>
          <Button
            variant={correction.dirty ? 'cta' : 'secondary'}
            className="w-full sm:w-auto"
            pending={correction.pending}
            disabled={busy || !correction.dirty || !correction.validation?.valid}
            onClick={() => void correction.save()}
          >
            {t('correction.save')}
          </Button>
          <Button
            variant={correction.dirty ? 'secondary' : 'cta'}
            className="w-full sm:w-auto"
            pending={renderPending}
            disabled={busy || correction.dirty || !renderReady || !correction.validation?.valid}
            onClick={onRender}
          >
            {t('correction.render')}
          </Button>
        </div>
      </ActionBar>
      <Dialog
        open={!!confirm}
        title={t(confirm === 'delete' ? 'correction.deleteTitle' : 'correction.leaveTitle')}
        confirmLabel={t(
          confirm === 'delete'
            ? 'correction.deleteCut'
            : confirm === 'reload'
              ? 'correction.reload'
              : 'project.leave',
        )}
        onClose={() => setConfirm(undefined)}
        onConfirm={() => {
          if (confirm === 'delete') mutate({ type: 'remove', id: deleteId })
          else if (confirm === 'reload') correction.reload()
          else onExit()
          setConfirm(undefined)
        }}
      >
        {t(confirm === 'delete' ? 'correction.deleteBody' : 'correction.leaveBody')}
      </Dialog>
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('correction.leaveTitle')}
        confirmLabel={t('project.leave')}
        onClose={() => blocker.reset?.()}
        onConfirm={() => {
          leaving.current = true
          blocker.proceed?.()
        }}
      >
        {t('correction.leaveBody')}
      </Dialog>
    </>
  )
}
