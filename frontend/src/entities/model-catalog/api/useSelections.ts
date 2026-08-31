import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { useTranslation } from 'react-i18next'
import { planLabel } from '@/entities/plan/@x/model-catalog'
import { ProviderService } from '@/shared/api'
import {
  type CatalogModel,
  type ModelRef,
  type StageName,
  type StageSelection,
  filterForStage,
  sameRef,
} from '../model/types'
import { toStageSelection } from './catalog-mappers'
import { useModels } from './useModels'

export type SelectionsByStage = Partial<Record<StageName, StageSelection>>

/** The acting user's saved choice per stage, as the server reports it. */
export function useSelections(): {
  selections: SelectionsByStage
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(ProviderService.method.getSelections, {})
  const selections = useMemo(() => {
    const byStage: SelectionsByStage = {}
    for (const selection of data?.selections ?? []) {
      const mapped = toStageSelection(selection)
      if (mapped) byStage[mapped.stage] = mapped
    }
    return byStage
  }, [data])
  return { selections, isPending, isError }
}

/** Why a saved choice cannot be used right now, for the dropdown to say so. */
export interface UnavailableSelection {
  ref: ModelRef
  reason: string
}

export interface StageSelectionState {
  /** The models this stage may list, in catalog order. */
  models: readonly CatalogModel[]
  /** The usable saved choice, or null until the user picks one. */
  selected: ModelRef | null
  /** A saved choice that cannot be used, with the reason to show greyed. */
  unavailable: UnavailableSelection | undefined
  isPending: boolean
  /** The catalog could not be loaded. */
  isError: boolean
}

/** What a stage has chosen, resolved against the catalog.
 *
 *  `selected` is null until the user picks a usable model — the callers of the
 *  generation and analysis actions block on exactly that (plan 04 AC7: no default
 *  pairing, [I3]). A saved choice that has vanished from the registry, whose provider
 *  lost its key, or that the stage can no longer use (observe needs vision) is not
 *  usable: it comes back as `unavailable` with the reason, for the dropdown to grey out.
 *
 *  While the catalog is still loading or failed to load, nothing is judged: a valid
 *  choice must not be called "vanished" because the list it would be found in is not
 *  here. */
export function useStageSelection(stage: StageName): StageSelectionState {
  const { t } = useTranslation('models')
  const { models: catalog, isPending: catalogPending, isError } = useModels()
  const { selections, isPending: selectionsPending } = useSelections()
  const saved = selections[stage]

  return useMemo(() => {
    const models = filterForStage(catalog, stage)
    const base = { models, isError, isPending: catalogPending || selectionsPending }
    if (!saved) return { ...base, selected: null, unavailable: undefined }
    if (saved.missing) {
      // The server reports a plan-locked choice as missing too — but its row is kept, and an
      // upgrade restores it. Saying "no longer registered" there would send the user off to
      // pick a replacement they do not need, so the catalog's own `locked` decides the reason.
      const locked = catalog.find(
        (candidate) => sameRef(candidate.ref, saved.ref) && candidate.locked,
      )
      return {
        ...base,
        selected: null,
        unavailable: {
          ref: saved.ref,
          reason: locked
            ? t('selectField.locked', { plan: planLabel(locked.minPlan) })
            : t('vanished'),
        },
      }
    }
    if (catalogPending || isError) return { ...base, selected: null, unavailable: undefined }

    const model = catalog.find((candidate) => sameRef(candidate.ref, saved.ref))
    if (!model) {
      // The selections answer predates a registry change the catalog already reflects.
      return { ...base, selected: null, unavailable: { ref: saved.ref, reason: t('vanished') } }
    }
    if (model.disabled) {
      return {
        ...base,
        selected: null,
        unavailable: { ref: saved.ref, reason: model.disabledReason },
      }
    }
    if (!models.includes(model)) {
      return { ...base, selected: null, unavailable: { ref: saved.ref, reason: t('unsuitable') } }
    }
    return { ...base, selected: saved.ref, unavailable: undefined }
  }, [saved, catalog, stage, catalogPending, selectionsPending, isError, t])
}
