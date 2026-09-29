import { sameRef, useStageSelection } from '@/entities/model-catalog'

/** The account's write selection as 검증 needs it: the ref, whether it reads images (a photo
 *  prompt needs `vision`, VOICE-43), and whether none is usable (VOICE-55). */
export function useWriteSelection() {
  const { selected, models, isPending } = useStageSelection('write')
  const vision = selected
    ? (models.find((model) => sameRef(model.ref, selected))?.vision ?? false)
    : false
  return { selected, vision, isPending, missing: !isPending && !selected }
}
