import { modelChoiceIssue, savedChoiceIssue } from './access'
import {
  filterForStage,
  sameRef,
  type CatalogModel,
  type ComparisonPair,
  type ModelRef,
  type StageName,
} from './types'

/** Saved A/B is a seed for a new binary test; retained lab extras never enter that seed. */
export function eligibleTestPair(
  pairs: readonly ComparisonPair[],
  models: readonly CatalogModel[],
  stage: StageName,
): [ModelRef, ModelRef] | undefined {
  const pair = pairs.find((value) => value.stage === stage)
  const left = pair?.candidateA
  const right = pair?.candidateB
  if (!left || !right || sameRef(left.ref, right.ref)) return undefined
  const candidates = filterForStage(models, stage)
  if (
    [left, right].some((selection) => {
      if (selection.missing || selection.unavailableReason || savedChoiceIssue(selection))
        return true
      const model = candidates.find((candidate) => sameRef(candidate.ref, selection.ref))
      // A saved choice is independent from balance; explicit test admission checks its quote.
      return (
        !model || model.disabled || Boolean(modelChoiceIssue({ ...model, affordable: true }, stage))
      )
    })
  )
    return undefined
  return [{ ...left.ref }, { ...right.ref }]
}
