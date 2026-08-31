import i18next from 'i18next'

/** The authorization ladder, in order. Every gate in the app asks the same question of it:
 *  is the acting tier at least this floor? */
export const PLANS = ['free', 'basic', 'max', 'master'] as const

export type PlanName = (typeof PLANS)[number]

export function isPlanName(value: unknown): value is PlanName {
  return typeof value === 'string' && (PLANS as readonly string[]).includes(value)
}

/** The tier's own name for a badge or a picker. An unknown tier is named as unknown rather
 *  than as `free`: the client must never invent an authority it was not told about. */
export function planLabel(plan: PlanName | undefined): string {
  return i18next.t(`tier.${plan ?? 'unknown'}`, { ns: 'plans' })
}

/** Whether an acting tier satisfies a floor. Display only — the server refuses on its own,
 *  whatever the client rendered. */
export function planAllows(acting: PlanName | undefined, floor: PlanName | undefined): boolean {
  if (!acting || !floor) return false
  return PLANS.indexOf(acting) >= PLANS.indexOf(floor)
}

/** Zero on any axis means unlimited (the operator tier), never a zero allowance. */
export interface PlanLimits {
  dailyJobStarts: number
  dailyBudgetMicrousd: number
  monthlyBudgetMicrousd: number
}

export interface PlanUsage {
  jobsStartedToday: number
  costTodayMicrousd: number
  costMonthMicrousd: number
  /** RFC3339; the instant each calendar window reopens, computed by the server. */
  dayResetsAt: string
  monthResetsAt: string
}

export interface MyPlan {
  plan: PlanName | undefined
  limits: PlanLimits
  usage: PlanUsage
}

/** One account as the operator screen sees it. */
export interface PlanAccount {
  id: string
  plan: PlanName | undefined
  createdAt: string
}
