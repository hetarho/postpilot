import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { ProviderService } from '@/shared/api'
import { MODEL_CATALOG_STALE_MS } from '@/shared/config'
import type { CatalogModel } from '../model/types'
import { toCatalogModel } from './catalog-mappers'

const NONE: CatalogModel[] = []

/** The registry snapshot, in the yaml's order. */
export function useModels(): {
  models: readonly CatalogModel[]
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(
    ProviderService.method.listModels,
    {},
    { staleTime: MODEL_CATALOG_STALE_MS },
  )
  const models = useMemo(() => data?.models.map(toCatalogModel) ?? NONE, [data])
  return { models, isPending, isError }
}
