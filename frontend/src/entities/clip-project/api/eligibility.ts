import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import {
  ClipAnalysisEligibility,
  ClipService,
  type ProtoClipAnalysisEligibilityList,
} from '@/shared/api'
import { type ClipEligibilityStatus, type ClipModelEligibility } from '../model/eligibility'

export const clipEligibilityKey = (transport: Transport, ownerId: string) =>
  ['clip-analysis-eligibility', transport, ownerId] as const

const STATUS: Partial<Record<ClipAnalysisEligibility, ClipEligibilityStatus>> = {
  [ClipAnalysisEligibility.ELIGIBLE]: 'eligible',
  [ClipAnalysisEligibility.VIDEO_INPUT_ABSENT]: 'video_input_absent',
  [ClipAnalysisEligibility.INLINE_ENDPOINT_UNAVAILABLE]: 'inline_endpoint_unavailable',
  [ClipAnalysisEligibility.REQUIRED_PARAMETERS_UNSUPPORTED]: 'required_parameters_unsupported',
  [ClipAnalysisEligibility.PRICE_CEILING_UNAVAILABLE]: 'price_ceiling_unavailable',
}

/** The wire answer as domain rows. `UNSPECIFIED`, an unknown enum value or a row without a
 *  model ref carries no status and is DROPPED, so a lookup for that model comes back
 *  unresolved rather than eligible (T111's domain rule, mirrored here). */
export function toClipEligibility(
  response: ProtoClipAnalysisEligibilityList,
): ClipModelEligibility[] {
  const out: ClipModelEligibility[] = []
  for (const row of response.models) {
    const status = STATUS[row.status]
    if (!row.model || !row.model.providerId || !row.model.modelId || !status) continue
    out.push({ ref: { providerId: row.model.providerId, modelId: row.model.modelId }, status })
  }
  return out
}

/** Every registered observe model's live clip-analysis eligibility (CLIP-44). Read-only on the
 *  server; a failed read is a surface state the caller shows and lets the user retry, never a
 *  reason attributed to a model. */
export function useClipAnalysisEligibility(ownerId: string) {
  const transport = useTransport()
  return useQuery({
    queryKey: clipEligibilityKey(transport, ownerId),
    queryFn: async ({ signal }) =>
      toClipEligibility(
        await createClient(ClipService, transport).listClipAnalysisEligibility({}, { signal }),
      ),
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  })
}
