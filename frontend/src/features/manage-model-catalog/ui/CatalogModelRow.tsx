import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  REASONING_EFFORTS,
  useSetModelPurpose,
  useUpdateModel,
  type AdminCatalogEntry,
  type ReasoningEffortName,
} from '@/entities/model-catalog'
import type { ModelPurpose } from '@/shared/config'
import { AppFailureMessage, Badge, Checkbox, FieldLabel, Listbox, Typography } from '@/shared/ui'

/** One model of the operator's catalog, seen from ONE purpose tab: what it is, whether this
 *  purpose uses it, and the shared per-model reasoning override.
 *
 *  The checkbox acts on the active tab's purpose only — the same model shows its own state on
 *  every tab, and the purposes it is registered to elsewhere are listed so a cross-purpose
 *  registration is visible without switching tabs. A card, because these controls act on one
 *  object and must read apart from the next model's (design-language §1.4). The reasoning
 *  Listbox appears once the model is registered anywhere: an override on a model nobody may
 *  select is a setting for nothing. */
export function CatalogModelRow({
  entry,
  purpose,
}: {
  entry: AdminCatalogEntry
  purpose: ModelPurpose
}) {
  const { t } = useTranslation(['models', 'plans'])
  const setPurpose = useSetModelPurpose()
  const update = useUpdateModel()
  const rowId = useId()
  const reasoningLabelId = `${rowId}-reasoning`

  const pending = setPurpose.isPending || update.isPending
  const failure = setPurpose.failure ?? update.failure
  const registeredHere = entry.purposes.includes(purpose)

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
            checked={registeredHere}
            disabled={pending}
            onChange={(e) => setPurpose.setPurpose(entry.modelId, purpose, e.target.checked)}
          />
          <Typography variant="label">{t('catalog.use')}</Typography>
        </label>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        {entry.vision && <Badge>{t('catalog.vision')}</Badge>}
        {entry.structuredOutput && <Badge>{t('catalog.structured')}</Badge>}
        {entry.imageOutput && <Badge>{t('catalog.imageOutput')}</Badge>}
        {entry.videoOutput && <Badge>{t('catalog.videoOutput')}</Badge>}
        {entry.curated && !entry.listed && <Badge tone="warning">{t('catalog.delisted')}</Badge>}
        {entry.contextTokens > 0n && (
          <Typography variant="meta">
            {t('catalog.context', { tokens: Number(entry.contextTokens).toLocaleString() })}
          </Typography>
        )}
        {/* A video model publishes no token price at all, and a zero there is the absence of
            one rather than "free" — so the row says which of the two it is instead of
            rendering $0 or nothing. */}
        {entry.inputUsdPerMillion !== '' || entry.outputUsdPerMillion !== '' ? (
          <Typography variant="meta">
            {t('catalog.price', { in: entry.inputUsdPerMillion, out: entry.outputUsdPerMillion })}
          </Typography>
        ) : (
          <Typography variant="meta">{t('catalog.priceUnpublished')}</Typography>
        )}
      </div>

      {entry.purposes.length > 0 && (
        <>
          <Typography variant="meta" className="mt-2 block">
            {t('catalog.registeredPurposes', {
              purposes: entry.purposes.map((p) => t(`catalog.purposeTab.${p}`)).join(' · '),
            })}
          </Typography>
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
        </>
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
