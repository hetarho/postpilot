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
  GetExchangeRateResponseSchema,
  GetMyPlanResponseSchema,
  ListUsersResponseSchema,
  PlanService,
  ProtoPlan,
  SetUserPlanResponseSchema,
  PostCreditsBasis,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeCreditLot {
  kind?: string
  granted?: number
  remaining?: number
  expiresAt?: string
  issuanceCause?: string
}

export interface FakeCreditBalance {
  credits?: number
  unlimited?: boolean
  lots?: FakeCreditLot[]
  renewsAt?: string
  dailyGrant?: number
  monthlyBonus?: number
  dailyResetsAt?: string
  bonusResetsAt?: string
}

export interface FakePlansOptions {
  /** The caller's own tier. Defaults to `master`, matching the session fake. */
  plan?: ProtoPlan
  /** What the account may spend. `unlimited` is the operator tier's shape. */
  balance?: FakeCreditBalance
  /** The rungs the comparison screen lists. */
  offers?: Array<{
    plan: ProtoPlan
    monthlyKrw?: number
    annualKrw?: number
    dailyCredits?: number
    monthlyBonus?: number
    monthlyServerExports?: number
    modelCeiling?: string
    recommended?: boolean
  }>
  /** The priced combos the estimator publishes. Defaults to one assigned tier so a screen
   *  test renders a post count; pass `[]` for the state an operator has not set up. */
  estimatorCombos?: Array<{
    combo: string
    /** One post's credits on the level's pair; defaults to 15 from recent usage. */
    postCredits?: number
    postCreditsBasis?: PostCreditsBasis
    clipRates?: {
      perSourceMilli: number
      perOutputSecondMilli: number
      perClipBaseMilli: number
    } | null
  }>
  clipSourceSeconds?: number
  serverExportWindow?: {
    coverageId: string
    startsAt?: string
    endsAt?: string
    allowance?: number
    used?: number
    reserved?: number
    remaining?: number
  }
  creditPacks?: Array<{ id: string; priceKrw: number; credits: number }>
  fxUnavailable?: boolean
  /** Make GetMyPlan fail. */
  planFails?: boolean
  /** The accounts the admin screen lists. */
  accounts?: Array<{ id: string; plan: ProtoPlan; createdAt?: string }>
  /** Refuse SetUserPlan the way the last-master guard does. */
  setPlanFails?: boolean
  listUsersFails?: boolean
  /** The rate /admin's 비용·환율 tab reads: a confirmed one by default, `'unavailable'` for none. */
  exchangeRate?:
    | {
        source: string
        publicationDate: string
        referenceE4: bigint
        appliedE4: bigint
        temporary?: boolean
      }
    | 'unavailable'
  /** How many GetExchangeRate calls fail before one succeeds. */
  exchangeRateFailures?: number
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
      clipSourceSeconds: options.clipSourceSeconds ?? 60,
      serverExportWindow: options.serverExportWindow,
      creditPacks: options.creditPacks ?? [
        { id: 'pack-1000', priceKrw: 3000, credits: 1000 },
        { id: 'pack-3000', priceKrw: 9000, credits: 3000 },
        { id: 'pack-10000', priceKrw: 30000, credits: 10000 },
      ],
      fxUnavailable: options.fxUnavailable ?? false,
      balance: {
        credits: options.balance?.credits ?? 0,
        // The session fake signs in as master, whose balance is not a number at all.
        unlimited:
          options.balance?.unlimited ?? (options.plan ?? ProtoPlan.MASTER) === ProtoPlan.MASTER,
        lots: (options.balance?.lots ?? []).map((lot) => ({
          kind: lot.kind ?? 'monthly',
          granted: lot.granted ?? 0,
          remaining: lot.remaining ?? 0,
          expiresAt: lot.expiresAt ?? '',
          issuanceCause: lot.issuanceCause ?? '',
        })),
        renewsAt: options.balance?.renewsAt ?? '',
        dailyGrant: options.balance?.dailyGrant ?? 0,
        monthlyBonus: options.balance?.monthlyBonus ?? 0,
        dailyResetsAt: options.balance?.dailyResetsAt ?? '',
        bonusResetsAt: options.balance?.bonusResetsAt ?? '',
      },
      // The shipped ladder, so a test that does not care about the figures still renders
      // what the server would actually send.
      offers: options.offers ?? [
        {
          plan: ProtoPlan.FREE,
          monthlyKrw: 0,
          annualKrw: 0,
          modelCeiling: 'none',
        },
        {
          plan: ProtoPlan.LIGHT,
          monthlyKrw: 1900,
          annualKrw: 19000,
          dailyCredits: 15,
          monthlyBonus: 290,
          modelCeiling: 'value',
          monthlyServerExports: 2,
        },
        {
          plan: ProtoPlan.BASIC,
          monthlyKrw: 4900,
          annualKrw: 49000,
          dailyCredits: 45,
          monthlyBonus: 510,
          monthlyServerExports: 6,
          modelCeiling: 'balanced',
        },
        {
          plan: ProtoPlan.PRO,
          monthlyKrw: 9900,
          annualKrw: 99000,
          dailyCredits: 85,
          monthlyBonus: 1070,
          monthlyServerExports: 15,
          modelCeiling: 'premium',
          recommended: true,
        },
        {
          plan: ProtoPlan.MAX,
          monthlyKrw: 29900,
          annualKrw: 299000,
          dailyCredits: 235,
          monthlyBonus: 3170,
          monthlyServerExports: 60,
          modelCeiling: 'top',
        },
      ],
      // One post's credits per level (QUOTA-64); clip rates stay the ones plan_test pins.
      estimatorCombos: (options.estimatorCombos ?? [{ combo: 'value' }, { combo: 'balanced' }]).map(
        (combo) => ({
          combo: combo.combo,
          postCredits: combo.postCredits ?? 15,
          postCreditsBasis: combo.postCreditsBasis ?? PostCreditsBasis.RECENT_USAGE,
          clipRates:
            combo.clipRates === null
              ? undefined
              : (combo.clipRates ?? {
                  perSourceMilli: 8750,
                  perOutputSecondMilli: 360,
                  perClipBaseMilli: 11200,
                }),
        }),
      ),
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

  let exchangeRateFailures = options.exchangeRateFailures ?? 0
  rpc(AdminService.method.getExchangeRate, () => {
    calls?.push('GetExchangeRate')
    if (exchangeRateFailures > 0) {
      exchangeRateFailures -= 1
      throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    }
    const rate = options.exchangeRate ?? {
      source: 'korea-eximbank',
      publicationDate: '2026-09-30',
      referenceE4: 13_584_000n,
      appliedE4: 13_600_000n,
    }
    return create(
      GetExchangeRateResponseSchema,
      rate === 'unavailable' ? { unavailable: true } : { rate: { temporary: false, ...rate } },
    )
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
