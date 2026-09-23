import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { QualityService } from '@/shared/api'
import type { PostMeasurement } from '../model/types'
import { toPostMeasurement } from './quality-mappers'
import { postMeasurementQueryKey } from './quality-queries'

/** This post's own M2, M3 and M4 at `revision` (QUAL-36). The previous revision's reading stays
 *  on screen while the next one loads, so an autosave does not flash the row back to its loading
 *  line. A reading this build cannot name fails the query rather than rendering a guess. */
export function usePostMeasurement(
  ownerId: string,
  slug: string,
  revision: bigint,
): {
  measurement: PostMeasurement | undefined
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: postMeasurementQueryKey(transport, ownerId, slug, revision),
    queryFn: () =>
      createClient(QualityService, transport).getPostMeasurement({ slug }).then(toPostMeasurement),
    enabled: ownerId !== '' && slug !== '',
    placeholderData: keepPreviousData,
  })
  return {
    measurement: query.data,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
