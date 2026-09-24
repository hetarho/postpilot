import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  GetAccountQualityResponseSchema,
  GetPostMeasurementResponseSchema,
  ProtoQualityMetric,
  ProtoQualityVerdict,
  QualityCompositionSchema,
  QualityCrossPostPhrasesSchema,
  QualityReadingSchema,
  QualityTitleSaturationSchema,
} from '@/shared/api'
import { QUALITY_METRICS } from '../model/types'
import {
  qualityMetricFromProto,
  qualityMetricToProto,
  qualityVerdictFromProto,
  qualityVerdictToProto,
  requireQualityMetric,
  requireQualityVerdict,
  toAccountQuality,
  toPostMeasurement,
  toQualityReading,
} from './quality-mappers'

/** Every number the generated enum names, UNSPECIFIED aside. */
function wireValues<E extends number>(enumObject: Record<string, string | number>, unspecified: E) {
  return Object.values(enumObject).filter(
    (value): value is E => typeof value === 'number' && value !== unspecified,
  )
}

// ARCH-3: the hand-kept mirror is pinned against the generated enum, so a metric or a verdict added
// to the contract without a name here fails this file rather than a screen.
describe('the quality enum mirrors', () => {
  it('map every generated metric both ways, in the wire order', () => {
    const wire = wireValues(ProtoQualityMetric, ProtoQualityMetric.UNSPECIFIED)
    expect(wire.map((value) => qualityMetricFromProto(value))).toEqual([...QUALITY_METRICS])
    for (const value of wire) expect(qualityMetricToProto(requireQualityMetric(value))).toBe(value)
  })

  it('map every generated verdict both ways, each to its own name', () => {
    const wire = wireValues(ProtoQualityVerdict, ProtoQualityVerdict.UNSPECIFIED)
    // By name, not only round-trip: two swapped verdicts would still round-trip.
    expect(wire.map((value) => qualityVerdictFromProto(value))).toEqual([
      'over_band',
      'within_band',
      'below_minimum',
      'absent',
    ])
    for (const value of wire) {
      const verdict = qualityVerdictFromProto(value)
      expect(verdict, `wire value ${value} has no name`).toBeDefined()
      expect(qualityVerdictToProto(requireQualityVerdict(value))).toBe(value)
    }
  })

  it.each([
    ['UNSPECIFIED', 0],
    ['an unknown number', 9_999],
  ])('reads %s as no metric and no verdict, never a guess', (_label, value) => {
    expect(qualityMetricFromProto(value as ProtoQualityMetric)).toBeUndefined()
    expect(qualityVerdictFromProto(value as ProtoQualityVerdict)).toBeUndefined()
    expect(() => requireQualityMetric(value as ProtoQualityMetric)).toThrow(
      `unsupported quality metric enum: ${value}`,
    )
    expect(() => requireQualityVerdict(value as ProtoQualityVerdict)).toThrow(
      `unsupported quality verdict enum: ${value}`,
    )
  })
})

describe('a quality reading', () => {
  it('keeps an unset value undefined and a stored 0 as 0 (QUAL-40)', () => {
    const reading = toQualityReading(
      create(QualityReadingSchema, {
        metric: ProtoQualityMetric.COMPOSITION,
        verdict: ProtoQualityVerdict.WITHIN_BAND,
        values: {
          case: 'composition',
          value: create(QualityCompositionSchema, {
            charCount: 0,
            photoCount: 2,
            distinctBlockTypes: 3,
            distinctBlockTypesWarnAtOrBelow: 2,
          }),
        },
      }),
    )
    expect(reading).toEqual({
      metric: 'composition',
      verdict: 'within_band',
      minimum: 0,
      publishedCount: 0,
      ruleText: '',
      values: {
        metric: 'composition',
        charCount: 0,
        photoCount: 2,
        distinctBlockTypes: 3,
        averageSentenceLength: undefined,
        distinctBlockTypesWarnAtOrBelow: 2,
      },
    })
    expect(reading.values && 'averageSentenceLength' in reading.values).toBe(true)
  })

  it('carries a reading with no values as none', () => {
    const reading = toQualityReading(
      create(QualityReadingSchema, {
        metric: ProtoQualityMetric.IN_POST_REPETITION,
        verdict: ProtoQualityVerdict.ABSENT,
      }),
    )
    expect(reading.values).toBeUndefined()
    expect(reading.verdict).toBe('absent')
  })

  it('refuses values that belong to another metric', () => {
    expect(() =>
      toQualityReading(
        create(QualityReadingSchema, {
          metric: ProtoQualityMetric.COMPOSITION,
          verdict: ProtoQualityVerdict.OVER_BAND,
          values: {
            case: 'crossPostPhrases',
            value: create(QualityCrossPostPhrasesSchema, { share: 0.2, shareWarnAbove: 0.1 }),
          },
        }),
      ),
    ).toThrow('quality reading for composition carries crossPostPhrases values')
  })

  it('reads a measurement in the order the server sent it', () => {
    const measurement = toPostMeasurement(
      create(GetPostMeasurementResponseSchema, {
        contentRevision: 7n,
        readings: [
          create(QualityReadingSchema, {
            metric: ProtoQualityMetric.CROSS_POST_PHRASES,
            verdict: ProtoQualityVerdict.BELOW_MINIMUM,
            minimum: 3,
            publishedCount: 1,
            values: {
              case: 'crossPostPhrases',
              value: create(QualityCrossPostPhrasesSchema, { shareWarnAbove: 0.1 }),
            },
          }),
          create(QualityReadingSchema, {
            metric: ProtoQualityMetric.COMPOSITION,
            verdict: ProtoQualityVerdict.ABSENT,
          }),
        ],
      }),
    )
    expect(measurement.contentRevision).toBe(7n)
    expect(measurement.readings.map((reading) => [reading.metric, reading.verdict])).toEqual([
      ['cross_post_phrases', 'below_minimum'],
      ['composition', 'absent'],
    ])
    expect(measurement.readings[0]).toMatchObject({ minimum: 3, publishedCount: 1 })
    expect(measurement.readings[0].values).toEqual({
      metric: 'cross_post_phrases',
      share: undefined,
      shareWarnAbove: 0.1,
    })
  })

  it('maps the account aggregate with its rule text', () => {
    const quality = toAccountQuality(
      create(GetAccountQualityResponseSchema, {
        readings: [
          create(QualityReadingSchema, {
            metric: ProtoQualityMetric.TITLE_SATURATION,
            verdict: ProtoQualityVerdict.OVER_BAND,
            minimum: 10,
            publishedCount: 12,
            ruleText: '제목에 “성수”를 매번 넣지 마세요.',
            values: {
              case: 'titleSaturation',
              value: create(QualityTitleSaturationSchema, { share: 0.42, shareWarnAbove: 0.3 }),
            },
          }),
          create(QualityReadingSchema, {
            metric: ProtoQualityMetric.COMPOSITION,
            verdict: ProtoQualityVerdict.WITHIN_BAND,
          }),
        ],
      }),
    )
    expect(quality.readings).toEqual([
      {
        metric: 'title_saturation',
        verdict: 'over_band',
        minimum: 10,
        publishedCount: 12,
        ruleText: '제목에 “성수”를 매번 넣지 마세요.',
        values: { metric: 'title_saturation', share: 0.42, shareWarnAbove: 0.3 },
      },
      {
        metric: 'composition',
        verdict: 'within_band',
        minimum: 0,
        publishedCount: 0,
        ruleText: '',
        values: undefined,
      },
    ])
  })

  it('fails the whole measurement on a reading it cannot name', () => {
    expect(() =>
      toPostMeasurement(
        create(GetPostMeasurementResponseSchema, {
          readings: [create(QualityReadingSchema, { verdict: ProtoQualityVerdict.ABSENT })],
        }),
      ),
    ).toThrow('unsupported quality metric enum: 0')
  })
})
