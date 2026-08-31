// Proto ↔ domain for the plan ladder. The Plan enum crosses here and nowhere else.
import {
  type GetMyPlanResponse,
  ProtoPlan,
  type ProtoPlanUser,
  type ProtoPlanLimits,
  type ProtoPlanUsage,
} from '@/shared/api'
import type { MyPlan, PlanAccount, PlanLimits, PlanName, PlanUsage } from '../model/types'

const PLAN_TO_PROTO: Record<PlanName, ProtoPlan> = {
  free: ProtoPlan.FREE,
  basic: ProtoPlan.BASIC,
  max: ProtoPlan.MAX,
  master: ProtoPlan.MASTER,
}

export function planToProto(plan: PlanName): ProtoPlan {
  return PLAN_TO_PROTO[plan]
}

/** Undefined for UNSPECIFIED and for a tier this build does not know — the caller then has
 *  no authority to show, which is the safe reading of a value it cannot interpret. */
export function planFromProto(plan: ProtoPlan): PlanName | undefined {
  for (const [name, wire] of Object.entries(PLAN_TO_PROTO) as [PlanName, ProtoPlan][]) {
    if (wire === plan) return name
  }
  return undefined
}

function toLimits(limits: ProtoPlanLimits | undefined): PlanLimits {
  return {
    dailyJobStarts: limits?.dailyJobStarts ?? 0,
    dailyBudgetMicrousd: Number(limits?.dailyBudgetMicrousd ?? 0n),
    monthlyBudgetMicrousd: Number(limits?.monthlyBudgetMicrousd ?? 0n),
  }
}

function toUsage(usage: ProtoPlanUsage | undefined): PlanUsage {
  return {
    jobsStartedToday: usage?.jobsStartedToday ?? 0,
    costTodayMicrousd: Number(usage?.costTodayMicrousd ?? 0n),
    costMonthMicrousd: Number(usage?.costMonthMicrousd ?? 0n),
    dayResetsAt: usage?.dayResetsAt ?? '',
    monthResetsAt: usage?.monthResetsAt ?? '',
  }
}

export function toMyPlan(response: GetMyPlanResponse | undefined): MyPlan | undefined {
  if (!response) return undefined
  return {
    plan: planFromProto(response.plan),
    limits: toLimits(response.limits),
    usage: toUsage(response.usage),
  }
}

export function toPlanAccount(user: ProtoPlanUser): PlanAccount {
  return { id: user.id, plan: planFromProto(user.plan), createdAt: user.createdAt }
}
