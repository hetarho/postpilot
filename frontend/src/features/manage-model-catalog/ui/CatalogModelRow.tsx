import { useId } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import {
  reasoningShare,
  useSetModelPurpose,
  useUpdateModel,
  type AdminCatalogEntry,
  type ReasoningEffortName,
  type ReasoningSpend,
} from '@/entities/model-catalog'
import type { ModelPurpose } from '@/shared/config'
import { AppFailureMessage, Badge, Checkbox, FieldLabel, Listbox, Typography } from '@/shared/ui'
import { offersReasoningControl, reasoningOptionsFor } from '../model/catalog-view'

/** One model of the operator's catalog, seen from ONE purpose tab: what it is, whether this
 *  purpose uses it, and the reasoning effort it runs at FOR THIS PURPOSE.
 *
 *  Both controls act on the active tab's purpose only — the same model shows its own state
 *  and its own effort on every tab, and the purposes it is registered to elsewhere are listed
 *  so a cross-purpose registration is visible without switching tabs. A card, because these
 *  controls act on one object and must read apart from the next model's (design-language
 *  §1.4).
 *
 *  The reasoning Listbox appears once the model is registered to THIS purpose AND the source
 *  says the model reasons: the effort is a property of the registration, and the server
 *  refuses one for a purpose the model does not serve. Its options are the model's own
 *  published list where there is one (change 27), so the operator cannot pick a value the
 *  model does not take. Beside it sits what the model actually spent at this stage — a
 *  declared list says what the model accepts, and the measurement says what it did with it:
 *  an unhonored effort behaves like sending none, and reasoning runs to the cap. */
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
  const showReasoning = offersReasoningControl(entry)

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
        <Typography variant="meta" className="mt-2 block">
          {t('catalog.registeredPurposes', {
            purposes: entry.purposes.map((p) => t(`catalog.purposeTab.${p}`)).join(' · '),
          })}
        </Typography>
      )}

      {registeredHere && (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {showReasoning && (
            <div className="min-w-0">
              <FieldLabel id={reasoningLabelId} htmlFor={`${rowId}-reasoning-control`}>
                {t('catalog.reasoning')}
              </FieldLabel>
              <Listbox<ReasoningEffortName>
                id={`${rowId}-reasoning-control`}
                value={entry.reasoningEffort}
                options={reasoningOptionsFor(entry).map((effort) => ({
                  value: effort,
                  label: reasoningOptionLabel(effort, entry, t),
                }))}
                disabled={pending}
                aria-labelledby={reasoningLabelId}
                onChange={(next) => {
                  if (next !== entry.reasoningEffort) {
                    update.update(entry.modelId, purpose, { reasoningEffort: next })
                  }
                }}
                className="mt-1"
              />
              {/* A warning, never a correction: the override is kept and still sent, exactly
                  as a delisted model is kept rather than retired. */}
              {entry.reasoning.drifted && (
                <Typography
                  variant="meta"
                  as="p"
                  role="status"
                  className="text-notice-warning-fg mt-1"
                >
                  {t('catalog.reasoningDrifted', {
                    effort: entry.reasoningEffort,
                    supported: entry.reasoning.efforts.join(' · '),
                  })}
                </Typography>
              )}
            </div>
          )}
          {/* Beside the control it acts on, not in a panel of its own (§4.3): the number and
              the decision it argues for have to be readable in one glance. */}
          <ReasoningSpendSignal spend={entry.reasoningSpend} />
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

/** What this model recently spent its completion budget on at this purpose's stage.
 *
 *  A model with no recorded call for this stage renders NOTHING rather than a zero: zero
 *  reasoning out of zero tokens is the absence of a measurement, and showing it as one is how
 *  an operator would come to trust a number nothing stands behind. */
function ReasoningSpendSignal({ spend }: { spend: ReasoningSpend | undefined }) {
  const { t } = useTranslation('models')
  if (!spend || spend.calls <= 0n) return null
  const share = reasoningShare(spend)
  // The tone reinforces the words, never replaces them (§2.6): the sentence names the share
  // and the call count either way.
  const heavy = share >= 0.5
  return (
    <div className="min-w-0">
      <Typography variant="label" as="p" className="text-content-secondary">
        {t('catalog.reasoningSpend')}
      </Typography>
      <Typography
        variant="meta"
        as="p"
        className={heavy ? 'text-notice-warning-fg mt-1' : 'text-content-tertiary mt-1'}
      >
        {t('catalog.reasoningSpendValue', {
          percent: Math.round(share * 100),
          calls: Number(spend.calls),
        })}
      </Typography>
      {heavy && (
        <Typography variant="meta" as="p" className="text-content-tertiary mt-1">
          {t('catalog.reasoningSpendHeavy')}
        </Typography>
      )}
    </div>
  )
}

/** `''` is "defer to the stage policy". `unset` is "send no effort key", which means the
 *  MODEL's own default — so it is labelled with that default where the source publishes one,
 *  because an operator reading a bare "unset" reasonably assumes the model will not reason. */
function reasoningOptionLabel(
  effort: ReasoningEffortName,
  entry: AdminCatalogEntry,
  t: TFunction<['models', 'plans']>,
): string {
  if (effort === '') return t('catalog.reasoningDefault')
  if (effort === 'unset' && entry.reasoning.defaultEffort !== '') {
    return t('catalog.reasoningUnsetWithDefault', { effort: entry.reasoning.defaultEffort })
  }
  return effort
}
