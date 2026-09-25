import { sameRef, useModelSetup, useStageSelection } from '@/entities/model-catalog'
import {
  briefIssues,
  type BriefIssues,
  type GenerationMode,
  type GenerationModelSelection,
} from './preconditions'

/** The selections 글 생성's two runs are given: the active observe and write models and the write
 *  A/B pair, each resolved against the catalog so its capabilities can be checked. */
export function useGenerationSelections() {
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const setup = useModelSetup()
  const pair = setup.pairs.find((value) => value.stage === 'write')
  return {
    observe: resolveSelection(observe.models, observe.selected),
    write: resolveSelection(write.models, write.selected),
    writeA: resolveSelection(write.models, pair?.candidateA?.ref ?? null),
    writeB: resolveSelection(write.models, pair?.candidateB?.ref ?? null),
    isPending: observe.isPending || write.isPending || setup.isPending,
  }
}

/** What the writing brief has to mark for a run of `mode` that was refused for its setup, read
 *  LIVE from the selections: a field the user fixes in the brief stops being marked the moment its
 *  save lands. Empty while nothing was refused, and while the catalog is still answering, when
 *  every selection reads as missing without being so. */
export function useBriefIssues(
  mode: GenerationMode | undefined,
  photoCount: number,
  videoCount: number,
): BriefIssues {
  const selections = useGenerationSelections()
  if (!mode || selections.isPending) return {}
  return briefIssues(
    mode,
    photoCount,
    videoCount,
    selections.observe,
    selections.write,
    selections.writeA,
    selections.writeB,
  )
}

function resolveSelection(
  models: ReturnType<typeof useStageSelection>['models'],
  selected: ReturnType<typeof useStageSelection>['selected'],
): GenerationModelSelection | undefined {
  if (!selected) return undefined
  const model = models.find((candidate) => sameRef(candidate.ref, selected))
  return model
    ? {
        ref: selected,
        vision: model.vision,
        videoInput: model.videoInput,
        signedVideoUrl: model.signedVideoUrl,
      }
    : undefined
}
