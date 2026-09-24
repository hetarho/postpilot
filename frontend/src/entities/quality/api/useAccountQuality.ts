import { useEffect } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { ContentLanguage } from '@/shared/api'
import type { AccountQuality } from '../model/types'
import { accountQualityQuery } from './quality-queries'

/** The account aggregate for one post's brief (POST-81). A read and nothing else: it calls no
 *  provider and costs no credit (QUAL-16). */
export function useAccountQuality(
  ownerId: string,
  slug: string,
  targetLanguage: ContentLanguage,
): {
  quality: AccountQuality | undefined
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    ...accountQualityQuery(transport, ownerId, slug, targetLanguage),
    enabled: ownerId !== '' && slug !== '',
  })
  return {
    quality: query.data,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}

/** Starts the aggregate read as soon as ①'s dock renders, so the rows are there when the brief
 *  opens. A draft with no post yet has nothing to read. */
export function usePrefetchAccountQuality(
  ownerId: string,
  slug: string,
  targetLanguage: ContentLanguage,
): void {
  const transport = useTransport()
  const queryClient = useQueryClient()
  useEffect(() => {
    if (ownerId === '' || slug === '') return
    void queryClient.prefetchQuery(accountQualityQuery(transport, ownerId, slug, targetLanguage))
  }, [queryClient, transport, ownerId, slug, targetLanguage])
}
