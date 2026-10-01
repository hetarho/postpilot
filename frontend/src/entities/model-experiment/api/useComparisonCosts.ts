import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { toModelRef } from '@/entities/model-catalog/@x/model-experiment'
import { AdminService, CostSource, LeaderboardWindow, Stage } from '@/shared/api'
import type { ExperimentStageName, LeaderboardWindowName } from '../model/types'

export type ComparisonCostQuality = 'reported' | 'estimated' | 'unavailable' | 'mixed'

export interface ComparisonCostRow {
  model: ReturnType<typeof toModelRef>
  modelLabel: string
  evaluatedComparisons: number
  totalCostMicrousd: bigint
  costQuality: ComparisonCostQuality
}

export function useComparisonCosts(stage: ExperimentStageName, window: LeaderboardWindowName) {
  const query = useQuery(AdminService.method.listComparisonCosts, {
    stage: stage === 'write' ? Stage.WRITE : Stage.OBSERVE,
    window:
      window === 'day'
        ? LeaderboardWindow.DAY
        : window === 'month'
          ? LeaderboardWindow.MONTH
          : LeaderboardWindow.WEEK,
  })
  const rows = useMemo(
    () =>
      query.data?.rows.map((row): ComparisonCostRow => ({
        model: toModelRef(row.model),
        modelLabel: row.modelLabel,
        evaluatedComparisons: row.evaluatedComparisons,
        totalCostMicrousd: row.totalCostMicrousd,
        costQuality: costQuality(row.costQuality),
      })) ?? [],
    [query.data],
  )
  return { ...query, rows }
}

function costQuality(value: CostSource): ComparisonCostQuality {
  switch (value) {
    case CostSource.REPORTED:
      return 'reported'
    case CostSource.ESTIMATED:
      return 'estimated'
    case CostSource.MIXED:
      return 'mixed'
    default:
      return 'unavailable'
  }
}
