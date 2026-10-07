import { sameRef, useStageSelection } from '@/entities/model-catalog'
import {
  briefIssues,
  type BriefIssues,
  type GenerationMode,
  type GenerationModelSelection,
} from './preconditions'

/** Both ordinary actions read the active observer and writer; tests own their entrants. */
export function useGenerationSelections() {
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  return {
    observe: resolveSelection(observe.models, observe.selected),
    write: resolveSelection(write.models, write.selected),
    isPending: observe.isPending || write.isPending,
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
  return briefIssues(mode, photoCount, videoCount, selections.observe, selections.write)
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
