import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { useTranslation } from 'react-i18next'
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
import { modelChoiceIssue, savedChoiceIssue } from '../model/access'
import { useModels } from './useModels'

export type SelectionsByStage = Partial<Record<StageName, StageSelection>>

/** The acting user's saved choice per stage, as the server reports it. What one post costs is
 *  each model's per-stage figure on the model list (QUOTA-64). */
export function useSelections(): {
  selections: SelectionsByStage
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const { data, isPending, isError, refetch } = useQuery(ProviderService.method.getSelections, {})
  const selections = useMemo(() => {
    const byStage: SelectionsByStage = {}
    for (const selection of data?.selections ?? []) {
      const mapped = toStageSelection(selection)
      if (mapped) byStage[mapped.stage] = mapped
    }
    return byStage
  }, [data])
  return {
    selections,
    isPending,
    isError,
    refetch: () => {
      void refetch()
    },
  }
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
 *  `selected` is null until an eligible saved choice exists, including a prepared
 *  recommended default. A saved choice that has vanished from the registry, whose provider
 *  lost its key, or that was deregistered from the stage's purpose is not usable: it
 *  comes back as `unavailable` with the reason, for the dropdown to grey out.
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
      // `missing` now means one thing only: the model is gone or unusable for the stage, and
      // the server preserves its identity until the owner explicitly replaces it.
      // Temporary credit restrictions likewise preserve the saved choice.
      return {
        ...base,
        selected: null,
        unavailable: { ref: saved.ref, reason: t('vanished') },
      }
    }
    if (catalogPending || isError) return { ...base, selected: null, unavailable: undefined }
    if (saved.unavailableReason) {
      return {
        ...base,
        selected: null,
        unavailable: { ref: saved.ref, reason: savedChoiceIssue(saved) || t('unsuitable') },
      }
    }

    const model = catalog.find((candidate) => sameRef(candidate.ref, saved.ref))
    if (!model) {
      // The selections answer predates a registry change the catalog already reflects.
      return { ...base, selected: null, unavailable: { ref: saved.ref, reason: t('vanished') } }
    }
    if (model.disabled) {
      return {
        ...base,
        selected: null,
        unavailable: { ref: saved.ref, reason: modelChoiceIssue(model, stage) },
      }
    }
    if (!models.includes(model)) {
      return { ...base, selected: null, unavailable: { ref: saved.ref, reason: t('unsuitable') } }
    }
    if (model.access?.[stage] && modelChoiceIssue(model, stage)) {
      return {
        ...base,
        selected: null,
        unavailable: {
          ref: saved.ref,
          reason: modelChoiceIssue(model, stage),
        },
      }
    }
    return { ...base, selected: saved.ref, unavailable: undefined }
  }, [saved, catalog, stage, catalogPending, selectionsPending, isError, t])
}
