/** What `clip-plan` exposes to `clip-preview` (ARCH-13 @x). */
export { clipPlanToProto } from '../api/edit-plan'
export {
  cutOutputMs,
  cutRate,
  outputToSourceMs,
  sourceAudioEnabled,
  sourceToOutputMs,
  timelineCuts,
} from '../model/edit-plan'
export type { ClipEditPlan, ClipTimelineCut, RetainedClipSource } from '../model/edit-plan'
export { clipSourceSound, textInterval } from '../model/timeline'
