import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipAnalysisEligibility, ListClipAnalysisEligibilityResponseSchema } from '@/shared/api'
import { toClipEligibility } from './eligibility'

describe('clip eligibility mapper', () => {
  it('maps the five statuses and drops what carries none', () => {
    const rows = toClipEligibility(
      create(ListClipAnalysisEligibilityResponseSchema, {
        models: [
          { model: { providerId: 'p', modelId: 'a' }, status: ClipAnalysisEligibility.ELIGIBLE },
          {
            model: { providerId: 'p', modelId: 'b' },
            status: ClipAnalysisEligibility.VIDEO_INPUT_ABSENT,
          },
          {
            model: { providerId: 'p', modelId: 'c' },
            status: ClipAnalysisEligibility.INLINE_ENDPOINT_UNAVAILABLE,
          },
          {
            model: { providerId: 'p', modelId: 'd' },
            status: ClipAnalysisEligibility.REQUIRED_PARAMETERS_UNSUPPORTED,
          },
          {
            model: { providerId: 'p', modelId: 'e' },
            status: ClipAnalysisEligibility.PRICE_CEILING_UNAVAILABLE,
          },
          // Unspecified is a missing value, never eligible; so is a row with no model.
          {
            model: { providerId: 'p', modelId: 'f' },
            status: ClipAnalysisEligibility.UNSPECIFIED,
          },
          { status: ClipAnalysisEligibility.ELIGIBLE },
          { model: { providerId: 'p', modelId: 'g' }, status: 99 as ClipAnalysisEligibility },
        ],
      }),
    )
    expect(rows.map((r) => `${r.ref.modelId}:${r.status}`)).toEqual([
      'a:eligible',
      'b:video_input_absent',
      'c:inline_endpoint_unavailable',
      'd:required_parameters_unsupported',
      'e:price_ceiling_unavailable',
    ])
  })
})
