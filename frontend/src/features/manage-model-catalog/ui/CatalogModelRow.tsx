import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  REASONING_EFFORTS,
  useEnableModel,
  useUpdateModel,
  type AdminCatalogEntry,
  type ReasoningEffortName,
} from '@/entities/model-catalog'
import { AppFailureMessage, Badge, Checkbox, FieldLabel, Listbox, Typography } from '@/shared/ui'

/** One model of the operator's catalog: what it is, and the three decisions that can be made
 *  about it — whether it may be used at all, from which tier, and with which reasoning effort.
 *
 *  A card, because those controls act on one object and must read apart from the next model's
 *  (design-language §1.4). The two Listboxes appear only once the model is in use: a floor and
 *  an override on a model nobody may select are settings for nothing. */
export function CatalogModelRow({ entry }: { entry: AdminCatalogEntry }) {
  const { t } = useTranslation(['models', 'plans'])
  const enable = useEnableModel()
  const update = useUpdateModel()
  const rowId = useId()
  const reasoningLabelId = `${rowId}-reasoning`

  const pending = enable.isPending || update.isPending
  const failure = enable.failure ?? update.failure

  function toggle(next: boolean) {
    if (next && !entry.curated) {
      enable.enable(entry.modelId)
      return
    }
    update.update(entry.modelId, { enabled: next })
  }

  return (
    // A div rather than the list item itself: the virtualized list owns the `li`, because that is
    // the element it has to position and measure.
    <div className="bg-surface-raised rounded-lg p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <Typography variant="label" className="text-content-primary block break-words">
            {entry.label}
          </Typography>
          <Typography variant="meta" mono className="mt-0.5 block break-all">
            {entry.modelId}
          </Typography>
        </div>
        {/* The label wraps the box, so the words are part of the same target the thumb finds. */}
        <label className="flex shrink-0 items-center gap-2">
          <Checkbox
            checked={entry.enabled}
            disabled={pending}
            onChange={(e) => toggle(e.target.checked)}
          />
          <Typography variant="label">{t('catalog.use')}</Typography>
        </label>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        {entry.vision && <Badge>{t('catalog.vision')}</Badge>}
        {entry.structuredOutput && <Badge>{t('catalog.structured')}</Badge>}
        {entry.curated && !entry.listed && <Badge tone="warning">{t('catalog.delisted')}</Badge>}
        {entry.contextTokens > 0n && (
          <Typography variant="meta">
            {t('catalog.context', { tokens: Number(entry.contextTokens).toLocaleString() })}
          </Typography>
        )}
        {entry.inputUsdPerMillion !== '' && (
          <Typography variant="meta">
            {t('catalog.price', { in: entry.inputUsdPerMillion, out: entry.outputUsdPerMillion })}
          </Typography>
        )}
      </div>

      {entry.enabled && (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <div className="min-w-0">
            <FieldLabel id={reasoningLabelId} htmlFor={`${rowId}-reasoning-control`}>
              {t('catalog.reasoning')}
            </FieldLabel>
            <Listbox<ReasoningEffortName>
              id={`${rowId}-reasoning-control`}
              value={entry.reasoningEffort}
              options={REASONING_EFFORTS.map((effort) => ({
                value: effort,
                label: effort === '' ? t('catalog.reasoningDefault') : effort,
              }))}
              disabled={pending}
              aria-labelledby={reasoningLabelId}
              onChange={(next) => {
                if (next !== entry.reasoningEffort) {
                  update.update(entry.modelId, { reasoningEffort: next })
                }
              }}
              className="mt-1"
            />
          </div>
        </div>
      )}

      {failure && (
        <Typography
          variant="body"
          as="div"
          role="alert"
          className="text-field-error mt-2 break-words"
        >
          <AppFailureMessage failure={failure} />
        </Typography>
      )}
    </div>
  )
}
