import { Code, createRouterTransport } from '@connectrpc/connect'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import {
  GetPostMeasurementResponseSchema,
  QualityCompositionSchema,
  QualityCrossPostPhrasesSchema,
  QualityInPostRepetitionSchema,
  QualityReadingSchema,
  QualityService,
  QualityTitleSaturationSchema,
} from '@/shared/api'
import {
  qualityMetricToProto,
  qualityVerdictToProto,
  type QualityMetricId,
  type QualityVerdict,
} from '@/entities/quality'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

/** One reading as a test states it. `values` is keyed by the value message's member names
 *  (`share`, `shareWarnAbove`, …); the fake builds the oneof member the metric names. Band edges
 *  here are fixture data, not the product's constants. */
export interface FakeQualityReading {
  metric: QualityMetricId
  verdict: QualityVerdict
  minimum?: number
  publishedCount?: number
  ruleText?: string
  values?: Record<string, number | undefined>
}

export interface FakeQualityOptions {
  /** Each post's M2, M3 and M4 by slug. A slug not listed answers all three as absent with no
   *  values, the way a post with nothing countable would. */
  measurements?: Record<string, FakeQualityReading[]>
  /** Make GetPostMeasurement fail. */
  measurementFails?: boolean
  calls?: string[]
}

const ABSENT: FakeQualityReading[] = [
  { metric: 'cross_post_phrases', verdict: 'absent' },
  { metric: 'in_post_repetition', verdict: 'absent' },
  { metric: 'composition', verdict: 'absent' },
]

type ReadingInit = MessageInitShape<typeof QualityReadingSchema>

function valuesOf(reading: FakeQualityReading): ReadingInit['values'] {
  if (!reading.values) return undefined
  const values = reading.values
  switch (reading.metric) {
    case 'title_saturation':
      return { case: 'titleSaturation', value: create(QualityTitleSaturationSchema, values) }
    case 'cross_post_phrases':
      return { case: 'crossPostPhrases', value: create(QualityCrossPostPhrasesSchema, values) }
    case 'in_post_repetition':
      return { case: 'inPostRepetition', value: create(QualityInPostRepetitionSchema, values) }
    case 'composition':
      return { case: 'composition', value: create(QualityCompositionSchema, values) }
  }
}

export function toFakeProtoReading(reading: FakeQualityReading) {
  return create(QualityReadingSchema, {
    metric: qualityMetricToProto(reading.metric),
    verdict: qualityVerdictToProto(reading.verdict),
    minimum: reading.minimum ?? 0,
    publishedCount: reading.publishedCount ?? 0,
    ruleText: reading.ruleText ?? '',
    values: valuesOf(reading),
  })
}

export function registerQualityService(router: ConnectRouter, options: FakeQualityOptions = {}) {
  const { rpc } = router
  const { calls } = options

  rpc(QualityService.method.getPostMeasurement, (req) => {
    calls?.push('GetPostMeasurement')
    if (options.measurementFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const readings = options.measurements?.[req.slug] ?? ABSENT
    return create(GetPostMeasurementResponseSchema, {
      readings: readings.map(toFakeProtoReading),
    })
  })
}

export function createFakeQualityTransport(options: FakeQualityOptions = {}) {
  return createRouterTransport((router) => registerQualityService(router, options))
}
