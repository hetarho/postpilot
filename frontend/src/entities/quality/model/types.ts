/** The four metrics, by the stable ASCII ids the product names them with, in the wire's order
 *  (QUAL-5). M1 is an account metric with no per-post value, so ② never shows it (QUAL-36). */
export const QUALITY_METRICS = [
  'title_saturation',
  'cross_post_phrases',
  'in_post_repetition',
  'composition',
] as const

export type QualityMetricId = (typeof QUALITY_METRICS)[number]

/** The four row states (QUAL-12). `absent` is a value that could not be computed: it neither
 *  passes nor warns (QUAL-40). */
export type QualityVerdict = 'over_band' | 'within_band' | 'below_minimum' | 'absent'

/** One metric's numbers, discriminated by the metric they belong to. A value the server left
 *  unset is `undefined` — never 0, which is a real measurement (QUAL-40). Each band edge is the
 *  server's: the client mirrors none of them and computes no verdict. */
export type QualityValues =
  | { metric: 'title_saturation'; share: number | undefined; shareWarnAbove: number }
  | { metric: 'cross_post_phrases'; share: number | undefined; shareWarnAbove: number }
  | {
      metric: 'in_post_repetition'
      repetitionShare: number | undefined
      titleRelevance: number | undefined
      repetitionShareWarnAbove: number
      titleRelevanceWarnBelow: number
    }
  | {
      metric: 'composition'
      charCount: number | undefined
      photoCount: number | undefined
      distinctBlockTypes: number | undefined
      averageSentenceLength: number | undefined
      distinctBlockTypesWarnAtOrBelow: number
    }

export interface QualityReading {
  metric: QualityMetricId
  verdict: QualityVerdict
  /** The 발행됨 count the metric needs and the account's own count. On a post's measurement they
   *  mean something on M2 only, the one per-post metric that reads the published window. */
  minimum: number
  publishedCount: number
  /** The text ticking the metric adds to the write prompt; set only on an account reading. */
  ruleText: string
  values: QualityValues | undefined
}

/** The account aggregate over its 발행됨 posts, all four metrics in wire order (POST-81). An
 *  over-band reading carries the rule text ticking it adds, in the post's target language. */
export interface AccountQuality {
  readings: QualityReading[]
}

/** A post's own M2, M3 and M4 at the content revision they describe (QUAL-3, QUAL-36). */
export interface PostMeasurement {
  contentRevision: bigint
  readings: QualityReading[]
}

/** A reading's values when they are the named metric's, narrowed to that metric's shape. */
export function qualityValuesOf<M extends QualityMetricId>(
  reading: QualityReading,
  metric: M,
): Extract<QualityValues, { metric: M }> | undefined {
  const values = reading.values
  return values?.metric === metric ? (values as Extract<QualityValues, { metric: M }>) : undefined
}
