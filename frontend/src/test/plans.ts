// Shared fake PlanService / AdminService for tests.
//
// It models the two server rules the frontend depends on: every displayed number comes from
// GetMyPlan (so a test can change a grant without touching the client), and the last master
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

export interface FakeCreditLot {
  kind?: string
  granted?: number
  remaining?: number
  expiresAt?: string
}

export interface FakeCreditBalance {
  credits?: number
  unlimited?: boolean
  lots?: FakeCreditLot[]
  renewsAt?: string
  monthlyGrant?: number
}

export interface FakePlansOptions {
  /** The caller's own tier. Defaults to `master`, matching the session fake. */
  plan?: ProtoPlan
  /** What the account may spend. `unlimited` is the operator tier's shape. */
  balance?: FakeCreditBalance
  /** The rungs the comparison screen lists. */
  offers?: Array<{ plan: ProtoPlan; monthlyCredits: number; priceUsdCents: number }>
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
      balance: {
        credits: options.balance?.credits ?? 0,
        // The session fake signs in as master, whose balance is not a number at all.
        unlimited: options.balance?.unlimited ?? true,
        lots: (options.balance?.lots ?? []).map((lot) => ({
          kind: lot.kind ?? 'monthly',
          granted: lot.granted ?? 0,
          remaining: lot.remaining ?? 0,
          expiresAt: lot.expiresAt ?? '',
        })),
        renewsAt: options.balance?.renewsAt ?? '',
        monthlyGrant: options.balance?.monthlyGrant ?? 0,
      },
      offers: options.offers ?? [
        { plan: ProtoPlan.FREE, monthlyCredits: 50, priceUsdCents: 0 },
        { plan: ProtoPlan.BASIC, monthlyCredits: 200, priceUsdCents: 200 },
        { plan: ProtoPlan.PRO, monthlyCredits: 500, priceUsdCents: 500 },
        { plan: ProtoPlan.MAX, monthlyCredits: 1000, priceUsdCents: 1000 },
      ],
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
