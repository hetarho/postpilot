import i18next from 'i18next'
import type { VoiceExample, VoiceFingerprint } from './types'

/** The eight counted items in VOICE-24's order. */
export const FINGERPRINT_ITEMS = [
  'endings',
  'marks',
  'emoji',
  'shape',
  'openings',
  'adverbs',
  'person',
  'headings',
] as const

export type FingerprintItem = (typeof FINGERPRINT_ITEMS)[number]

/** How a facet's two values read (VOICE-62). */
export type FingerprintFacetUnit = 'share' | 'per_hundred' | 'chars' | 'sentences' | 'text'

/** One value of a counted item on the voice and on a text: numbers in their unit, or a text
 *  facet's terms (possibly none). The facet key is the server's; the widget words it. */
export type FingerprintFacet =
  | { key: string; unit: Exclude<FingerprintFacetUnit, 'text'>; voice: number; text: number }
  | { key: string; unit: 'text'; voice: string[]; text: string[] }

/** One counted item measured on the voice and on a text, as the server orders them: farthest
 *  from the voice first, unknown last (VOICE-62). */
export interface FingerprintComparisonItem {
  item: FingerprintItem
  unknown: boolean
  distance: number
  /** The key of the facet with the largest gap. */
  headline: string
  facets: FingerprintFacet[]
}

/** ②'s reading of a post against its voice (POST-102); not applicable for 말투 없음, no content,
 *  or a deleted or unmade voice. */
export interface PostFingerprint {
  applicable: boolean
  revision: bigint
  items: FingerprintComparisonItem[]
}

/** One counted item as a reader sees it: its name, one plain sentence with its numbers — or
 *  알 수 없음 — and the example sentence it was counted from (VOICE-63). */
export interface FingerprintRow {
  item: FingerprintItem
  label: string
  sentence: string
  unknown: boolean
  example?: VoiceExample
}

const pct = (share: number) => Math.round(share * 100)
const count = (value: number) => Math.round(value)
type SentenceKey =
  | 'endings'
  | 'suffixes'
  | 'marks'
  | 'emoji'
  | 'shapeOwnLine'
  | 'shapeRunOn'
  | 'openings'
  | 'closings'
  | 'adverbs'
  | 'adverbRate'
  | 'adverbsNone'
  | 'person'
  | 'personNone'
  | 'headings'
  | 'lists'
type FingerprintKey = 'unknown' | `label.${FingerprintItem}` | `sentence.${SentenceKey}`
// The keys are checked by the union above and by the catalogue's parity test; the interpolation
// values vary per key, which i18next's per-key typing cannot express for one helper.
const translate = i18next.t.bind(i18next) as unknown as (
  key: string,
  options: Record<string, unknown>,
) => string
const t = (key: FingerprintKey, options?: Record<string, string | number>) =>
  translate(`fingerprint.${key}`, { ns: 'voices', ...options })

/** The sentence an item reads as, in the active locale. */
export function fingerprintSentence(fingerprint: VoiceFingerprint, item: FingerprintItem): string {
  switch (item) {
    case 'endings': {
      const e = fingerprint.endings
      const base = t('sentence.endings', {
        da: pct(e.da),
        haeyo: pct(e.haeyo),
        seumnida: pct(e.seumnida),
      })
      if (e.suffixes.length === 0) return base
      return `${base} ${t('sentence.suffixes', { suffixes: e.suffixes.map((suffix) => `~${suffix.text}`).join(', ') })}`
    }
    case 'marks': {
      const m = fingerprint.marks
      return t('sentence.marks', {
        exclaim: pct(m.exclaim),
        question: pct(m.question),
        tilde: pct(m.tilde),
        repeat: pct(m.repeat),
      })
    }
    case 'emoji': {
      const e = fingerprint.emoji
      return t('sentence.emoji', {
        emoji: count(e.emoji),
        hh: count(e.hh),
        kk: count(e.kk),
        tears: count(e.tears),
      })
    }
    case 'shape': {
      const s = fingerprint.shape
      const range =
        s.paragraphMin === s.paragraphMax
          ? `${s.paragraphMin}`
          : `${s.paragraphMin}~${s.paragraphMax}`
      return t(s.ownLine ? 'sentence.shapeOwnLine' : 'sentence.shapeRunOn', {
        average: count(s.averageChars),
        range,
      })
    }
    case 'openings': {
      const o = fingerprint.openings
      const parts = []
      if (o.openings.length > 0)
        parts.push(
          t('sentence.openings', { lines: o.openings.map((line) => `“${line}”`).join(', ') }),
        )
      if (o.closings.length > 0)
        parts.push(
          t('sentence.closings', { lines: o.closings.map((line) => `“${line}”`).join(', ') }),
        )
      return parts.join(' · ')
    }
    case 'adverbs': {
      const a = fingerprint.adverbs
      if (a.none || a.words.length === 0) return t('sentence.adverbsNone')
      return t('sentence.adverbs', {
        words: a.words
          .map((word) =>
            t('sentence.adverbRate', { word: word.word, count: count(word.perHundred) }),
          )
          .join(', '),
      })
    }
    case 'person': {
      const p = fingerprint.person
      if (!p.dominant) return t('sentence.personNone')
      const rate = p.dominant === '저' ? p.jeo : p.dominant === '우리' ? p.uri : p.na
      return t('sentence.person', { form: p.dominant, count: count(rate) })
    }
    case 'headings': {
      const h = fingerprint.headings
      const parts = []
      if (h.count > 0)
        parts.push(
          t('sentence.headings', { emoji: pct(h.emojiShare), question: pct(h.questionShare) }),
        )
      if (h.marker) parts.push(t('sentence.lists', { marker: h.marker }))
      return parts.join(' · ')
    }
  }
}

/** The eight measured analysis rows, 알 수 없음 where an item is unknown. */
export function fingerprintRows(fingerprint: VoiceFingerprint): FingerprintRow[] {
  return FINGERPRINT_ITEMS.map((item) => {
    const unknown = fingerprint[item].unknown
    return {
      item,
      label: t(`label.${item}`),
      sentence: unknown ? t('unknown') : fingerprintSentence(fingerprint, item),
      unknown,
      ...(unknown || !fingerprint[item].example ? {} : { example: fingerprint[item].example }),
    }
  })
}
