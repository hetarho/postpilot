import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import {
  FingerprintItemComparisonSchema,
  ProtoFingerprintItem,
  ProtoFingerprintVerdict,
} from '@/shared/api'
import { toComparisons } from './fingerprint-comparison'

const wire = (item: ProtoFingerprintItem, verdict: ProtoFingerprintVerdict, unknown = false) =>
  create(FingerprintItemComparisonSchema, { item, verdict, unknown, headline: 'x' })

// VOICE-62: the verdict rides each item through as similar, different or none, and the server's
// order (다름 first, then 비슷함, then unjudged) is kept as it came.
it('maps each verdict and keeps the server order', () => {
  const items = toComparisons([
    wire(ProtoFingerprintItem.MARKS, ProtoFingerprintVerdict.DIFFERENT),
    wire(ProtoFingerprintItem.ENDINGS, ProtoFingerprintVerdict.SIMILAR),
    wire(ProtoFingerprintItem.ADVERBS, ProtoFingerprintVerdict.UNSPECIFIED),
    wire(ProtoFingerprintItem.EMOJI, ProtoFingerprintVerdict.UNSPECIFIED, true),
  ])

  expect(items.map((item) => item.item)).toEqual(['marks', 'endings', 'adverbs', 'emoji'])
  expect(items.map((item) => item.verdict)).toEqual(['different', 'similar', undefined, undefined])
})
