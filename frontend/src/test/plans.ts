// Shared fake PlanService / AdminService for tests.
//
// It models the two server rules the frontend depends on: every displayed number comes from
// GetMyPlan (so a test can change a limit without touching the client), and the last master
// cannot be demoted (so the admin screen must render the server's refusal rather than predict
// it).
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  AdminService,
  GetMyPlanResponseSchema,
  ListUsersResponseSchema,
  PlanService,
  ProtoPlan,
  SetUserPlanResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakePlanUsage {
  jobsStartedToday?: number
  costTodayMicrousd?: bigint
  costMonthMicrousd?: bigint
  dayResetsAt?: string
  monthResetsAt?: string
}

export interface FakePlansOptions {
  /** The caller's own tier. Defaults to `master`, matching the session fake. */
  plan?: ProtoPlan
  /** The three limits, in the server's units. Zero means unlimited. */
  limits?: { dailyJobStarts?: number; dailyBudgetMicrousd?: bigint; monthlyBudgetMicrousd?: bigint }
  usage?: FakePlanUsage
  /** Make GetMyPlan fail. */
  planFails?: boolean
  /** The accounts the admin screen lists. */
  accounts?: Array<{ id: string; plan: ProtoPlan; createdAt?: string }>
  /** Refuse SetUserPlan the way the last-master guard does. */
  setPlanFails?: boolean
  listUsersFails?: boolean
  calls?: string[]
}

export function registerPlanServices(router: ConnectRouter, options: FakePlansOptions = {}) {
  const { rpc } = router
  const { calls } = options
  let accounts = [...(options.accounts ?? [])]

  rpc(PlanService.method.getMyPlan, () => {
    calls?.push('GetMyPlan')
    if (options.planFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(GetMyPlanResponseSchema, {
      plan: options.plan ?? ProtoPlan.MASTER,
      limits: {
        dailyJobStarts: options.limits?.dailyJobStarts ?? 0,
        dailyBudgetMicrousd: options.limits?.dailyBudgetMicrousd ?? 0n,
        monthlyBudgetMicrousd: options.limits?.monthlyBudgetMicrousd ?? 0n,
      },
      usage: {
        jobsStartedToday: options.usage?.jobsStartedToday ?? 0,
        costTodayMicrousd: options.usage?.costTodayMicrousd ?? 0n,
        costMonthMicrousd: options.usage?.costMonthMicrousd ?? 0n,
        dayResetsAt: options.usage?.dayResetsAt ?? '',
        monthResetsAt: options.usage?.monthResetsAt ?? '',
      },
    })
  })

  rpc(AdminService.method.listUsers, () => {
    calls?.push('ListUsers')
    if (options.listUsersFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListUsersResponseSchema, {
      users: accounts.map((account) => ({
        id: account.id,
        plan: account.plan,
        createdAt: account.createdAt ?? '2026-08-01T00:00:00Z',
      })),
    })
  })

  rpc(AdminService.method.setUserPlan, (req) => {
    calls?.push('SetUserPlan')
    if (options.setPlanFails) throw connectAppError('LAST_MASTER', Code.FailedPrecondition)
    accounts = accounts.map((account) =>
      account.id === req.userId ? { ...account, plan: req.plan } : account,
    )
    return create(SetUserPlanResponseSchema, {
      user: { id: req.userId, plan: req.plan },
    })
  })
}
