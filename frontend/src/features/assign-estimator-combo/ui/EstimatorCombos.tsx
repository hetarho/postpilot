import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAdminCatalog, type AdminCatalogEntry } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  FieldLabel,
  Listbox,
  Notice,
  Typography,
  type ListboxOption,
} from '@/shared/ui'
import { useAssignEstimatorCombo } from '../api/useAssignEstimatorCombo'

/** The four estimator combos, richest first. The names are the product's (QUOTA-39) — the
 *  operator names the models behind each tier, not the tier itself. */
const COMBOS = ['quality', 'balanced', 'value', 'cheapest'] as const

/** Which models a comparison screen's post counts are priced with (QUOTA-39).
 *
 *  Assigning a price tier IS curation: the two models it names must be registered to the stage
 *  they serve on the 모델 관리 tab, and the pickers here offer only what that tab registered.
 *  It has its own tab under `/admin` because the catalog runs to several hundred rows and a
 *  section beneath it was out of sight; the assignments still ride the same `ListCatalog` read,
 *  so the tab costs the same one request the catalog does.
 *
 *  A combo needs BOTH models before it means anything, so a half-chosen pair is not sent: the
 *  call carries the complete assignment or waits. */
export function EstimatorCombos() {
  const { t } = useTranslation('models')
  const titleId = useId()
  // Any purpose serves: every entry carries its own registrations, and the purpose only
  // decides which stage's reasoning annotations come with the row.
  const { catalog, isPending, isError } = useAdminCatalog('photo-analysis')
  const assign = useAssignEstimatorCombo()

  const observers = catalog.entries.filter((entry) => entry.purposes.includes('photo-analysis'))
  const writers = catalog.entries.filter((entry) => entry.purposes.includes('writing'))

  return (
    // Named so the section is a landmark an operator jumping by region can reach directly.
    <section aria-labelledby={titleId} className="mt-8 grid gap-4">
      <div className="grid gap-1">
        <Typography variant="title" as="h2" id={titleId}>
          {t('combos.title')}
        </Typography>
        <Typography variant="body" className="text-content-secondary max-w-measure">
          {t('combos.description')}
        </Typography>
      </div>

      {isError && (
        <Notice tone="danger" role="alert">
          {t('combos.loadFailed')}
        </Notice>
      )}
      {assign.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={assign.failure} />
        </Notice>
      )}

      {!isError && (
        <div className="grid gap-4">
          {COMBOS.map((combo) => {
            const assigned = catalog.estimatorCombos.find((entry) => entry.combo === combo)
            return (
              // The key carries the assignment, so a row re-seeds from the server after
              // every write rather than holding a draft that the store has moved past.
              <ComboRow
                key={`${combo}:${assigned?.observeModelId ?? ''}:${assigned?.writeModelId ?? ''}`}
                combo={combo}
                observeModelId={assigned?.observeModelId ?? ''}
                writeModelId={assigned?.writeModelId ?? ''}
                observers={observers}
                writers={writers}
                saving={assign.isPending}
                loading={isPending}
                onAssign={assign.assign}
              />
            )
          })}
        </div>
      )}
    </section>
  )
}

function ComboRow({
  combo,
  observeModelId,
  writeModelId,
  observers,
  writers,
  saving,
  loading,
  onAssign,
}: {
  combo: (typeof COMBOS)[number]
  observeModelId: string
  writeModelId: string
  observers: readonly AdminCatalogEntry[]
  writers: readonly AdminCatalogEntry[]
  saving: boolean
  loading: boolean
  onAssign: (combo: string, observeModelId: string, writeModelId: string) => void
}) {
  const { t } = useTranslation('models')
  const id = useId()
  // The pair is drafted here and sent only once both halves are chosen: the server takes a
  // complete assignment, so sending the first choice alone would be a refusal the operator
  // caused by picking in the obvious order.
  const [observe, setObserve] = useState(observeModelId)
  const [write, setWrite] = useState(writeModelId)

  const choose = (next: { observe?: string; write?: string }) => {
    const chosenObserve = next.observe ?? observe
    const chosenWrite = next.write ?? write
    setObserve(chosenObserve)
    setWrite(chosenWrite)
    if (chosenObserve !== '' && chosenWrite !== '') {
      onAssign(combo, chosenObserve, chosenWrite)
    }
  }

  const complete = observe !== '' && write !== ''
  const headingId = `${id}-heading`

  return (
    // A labelled pair of fields is a group, not a list item: the tier names the two controls
    // inside it, and the page's one list is the catalog above.
    <div role="group" aria-labelledby={headingId} className="bg-surface-raised rounded-lg p-4">
      <Typography variant="fieldTitle" as="h3" id={headingId}>
        {t(`combos.name.${combo}`)}
      </Typography>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <ComboPicker
          id={`${id}-observe`}
          label={t('combos.observe')}
          value={observe}
          entries={observers}
          disabled={saving || loading}
          onChange={(modelId) => choose({ observe: modelId })}
        />
        <ComboPicker
          id={`${id}-write`}
          label={t('combos.write')}
          value={write}
          entries={writers}
          disabled={saving || loading}
          onChange={(modelId) => choose({ write: modelId })}
        />
      </div>
      {!complete && (
        <Typography variant="meta" className="text-content-tertiary mt-2 block">
          {t('combos.unassigned')}
        </Typography>
      )}
    </div>
  )
}

function ComboPicker({
  id,
  label,
  value,
  entries,
  disabled,
  onChange,
}: {
  id: string
  label: string
  value: string
  entries: readonly AdminCatalogEntry[]
  disabled: boolean
  onChange: (modelId: string) => void
}) {
  const { t } = useTranslation('models')
  const labelId = `${id}-label`
  const known = entries.some((entry) => entry.modelId === value)
  const options: ListboxOption<string>[] = [
    { value: '', label: t('combos.none') },
    ...entries.map((entry) => ({ value: entry.modelId, label: entry.label })),
  ]
  // A model that has since lost its registration is still shown as the current value, greyed:
  // the operator must be able to see WHY the comparison dropped this tier.
  if (value !== '' && !known) {
    options.push({ value, label: t('combos.retired', { model: value }), disabled: true })
  }

  return (
    <div>
      <FieldLabel id={labelId} htmlFor={id}>
        {label}
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={labelId}
        className="mt-1"
        value={value}
        options={options}
        disabled={disabled}
        onChange={onChange}
      />
    </div>
  )
}
