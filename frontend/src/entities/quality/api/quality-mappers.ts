import {
  ProtoQualityMetric,
  ProtoQualityVerdict,
  type ProtoPostMeasurement,
  type ProtoQualityReading,
} from '@/shared/api'
import type {
  PostMeasurement,
  QualityMetricId,
  QualityReading,
  QualityValues,
  QualityVerdict,
} from '../model/types'

// Closed both ways (ARCH-3): a number this build does not know is undefined from the `FromProto`
// pair and a thrown error from the `require` pair, never a guess. The query then fails and the
// row says so, rather than showing a reading it cannot name.

const METRIC_TO_PROTO: Record<QualityMetricId, ProtoQualityMetric> = {
  title_saturation: ProtoQualityMetric.TITLE_SATURATION,
  cross_post_phrases: ProtoQualityMetric.CROSS_POST_PHRASES,
  in_post_repetition: ProtoQualityMetric.IN_POST_REPETITION,
  composition: ProtoQualityMetric.COMPOSITION,
}

const METRIC_FROM_PROTO = new Map<ProtoQualityMetric, QualityMetricId>(
  Object.entries(METRIC_TO_PROTO).map(([id, wire]) => [wire, id as QualityMetricId]),
)

const VERDICT_TO_PROTO: Record<QualityVerdict, ProtoQualityVerdict> = {
  over_band: ProtoQualityVerdict.OVER_BAND,
  within_band: ProtoQualityVerdict.WITHIN_BAND,
  below_minimum: ProtoQualityVerdict.BELOW_MINIMUM,
  absent: ProtoQualityVerdict.ABSENT,
}

const VERDICT_FROM_PROTO = new Map<ProtoQualityVerdict, QualityVerdict>(
  Object.entries(VERDICT_TO_PROTO).map(([id, wire]) => [wire, id as QualityVerdict]),
)

export function qualityMetricFromProto(value: ProtoQualityMetric): QualityMetricId | undefined {
  return METRIC_FROM_PROTO.get(value)
}

export function requireQualityMetric(value: ProtoQualityMetric): QualityMetricId {
  const metric = qualityMetricFromProto(value)
  if (!metric) throw new Error(`unsupported quality metric enum: ${String(value)}`)
  return metric
}

export function qualityMetricToProto(id: QualityMetricId): ProtoQualityMetric {
  return METRIC_TO_PROTO[id]
}

export function qualityVerdictFromProto(value: ProtoQualityVerdict): QualityVerdict | undefined {
  return VERDICT_FROM_PROTO.get(value)
}

export function requireQualityVerdict(value: ProtoQualityVerdict): QualityVerdict {
  const verdict = qualityVerdictFromProto(value)
  if (!verdict) throw new Error(`unsupported quality verdict enum: ${String(value)}`)
  return verdict
}

export function qualityVerdictToProto(verdict: QualityVerdict): ProtoQualityVerdict {
  return VERDICT_TO_PROTO[verdict]
}

/** The values a reading carries, which must be the member set its metric names. */
function toQualityValues(
  metric: QualityMetricId,
  values: ProtoQualityReading['values'],
): QualityValues | undefined {
  const matching = (expected: QualityMetricId) => {
    if (metric !== expected)
      throw new Error(`quality reading for ${metric} carries ${String(values.case)} values`)
  }
  switch (values.case) {
    case undefined:
      return undefined
    case 'titleSaturation':
      matching('title_saturation')
      return {
        metric: 'title_saturation',
        share: values.value.share,
        shareWarnAbove: values.value.shareWarnAbove,
      }
    case 'crossPostPhrases':
      matching('cross_post_phrases')
      return {
        metric: 'cross_post_phrases',
        share: values.value.share,
        shareWarnAbove: values.value.shareWarnAbove,
      }
    case 'inPostRepetition':
      matching('in_post_repetition')
      return {
        metric: 'in_post_repetition',
        repetitionShare: values.value.repetitionShare,
        titleRelevance: values.value.titleRelevance,
        repetitionShareWarnAbove: values.value.repetitionShareWarnAbove,
        titleRelevanceWarnBelow: values.value.titleRelevanceWarnBelow,
      }
    case 'composition':
      matching('composition')
      return {
        metric: 'composition',
        charCount: values.value.charCount,
        photoCount: values.value.photoCount,
        distinctBlockTypes: values.value.distinctBlockTypes,
        averageSentenceLength: values.value.averageSentenceLength,
        distinctBlockTypesWarnAtOrBelow: values.value.distinctBlockTypesWarnAtOrBelow,
      }
  }
}

export function toQualityReading(reading: ProtoQualityReading): QualityReading {
  const metric = requireQualityMetric(reading.metric)
  return {
    metric,
    verdict: requireQualityVerdict(reading.verdict),
    minimum: reading.minimum,
    publishedCount: reading.publishedCount,
    ruleText: reading.ruleText,
    values: toQualityValues(metric, reading.values),
  }
}

export function toPostMeasurement(response: ProtoPostMeasurement): PostMeasurement {
  return {
    contentRevision: response.contentRevision,
    readings: response.readings.map(toQualityReading),
  }
}
