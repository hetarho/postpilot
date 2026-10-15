import type { StageDraft, StageRole } from '../model/wizard-machine'
import { STAGE_ROLES } from '../model/wizard-machine'

export function subjectOf(path: readonly string[]): string {
  return path[0] === 'product' ? (path[1] ?? 'product') : (path.at(-1) ?? path[0] ?? 'product')
}
/** The seven-stage arc is the same at every leaf; its topic words come from that leaf. */
export function clipStageSeeds(
  labels: Record<StageRole, { name: string; intent: string }>,
  fieldLabel: string,
): StageDraft[] {
  return STAGE_ROLES.map((role) => ({
    id: role,
    role,
    name: labels[role].name,
    heading: labels[role].name,
    intent: labels[role].intent,
    checked: true,
    asks: role === 'info',
    fieldLabel,
  }))
}
