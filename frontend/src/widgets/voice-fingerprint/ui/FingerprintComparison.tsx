import i18next from 'i18next'
import { useTranslation } from 'react-i18next'
import type { FingerprintComparisonItem, FingerprintFacet, FingerprintItem } from '@/entities/voice'
import { Disclosure, Typography, typographyStyles } from '@/shared/ui'

/** The server's facet keys whose wording key differs from the key itself; every other key is
 *  its own wording key, and an adverb's key is the word (VOICE-24 ⑥). */
const FACET_KEYS: Partial<Record<FingerprintItem, Record<string, string>>> = {
  endings: { 다: 'da', 해요: 'haeyo', 습니다: 'seumnida', 기타: 'other' },
  emoji: { ㅎㅎ: 'hh', ㅋㅋ: 'kk', ㅠㅠ: 'tears' },
  person: { 저: 'jeo', 우리: 'uri', 나: 'na' },
}

// The facet keys are the server's and vary per item, which i18next's per-key typing cannot
// express for one helper; the catalogue's parity test checks both locales hold every key.
const translate = i18next.t.bind(i18next) as unknown as (
  key: string,
  options: Record<string, unknown>,
) => string
const t = (key: string, options?: Record<string, string | number>) =>
  translate(`comparison.${key}`, { ns: 'voices', ...options })

function facetLabel(item: FingerprintItem, key: string): string {
  if (item === 'adverbs') return t('facet.adverbs.word', { word: key })
  const wording = FACET_KEYS[item]?.[key] ?? key
  const fullKey = `comparison.facet.${item}.${wording}`
  return i18next.exists(fullKey, { ns: 'voices' }) ? t(`facet.${item}.${wording}`) : key
}

function numberText(unit: Exclude<FingerprintFacet['unit'], 'text'>, value: number): string {
  switch (unit) {
    case 'share':
      return t('unit.share', { value: Math.round(value * 100) })
    case 'per_hundred':
      return t('unit.per_hundred', { value: Math.round(value) })
    case 'chars':
      return t('unit.chars', { value: Math.round(value) })
    case 'sentences':
      return t('unit.sentences', { value: Math.round(value * 10) / 10 })
  }
}

function termsText(key: string, terms: string[]): string {
  if (terms.length === 0) return t('unit.none')
  return terms.map((term) => (key === 'suffixes' ? `~${term}` : `‘${term}’`)).join(', ')
}

/** One facet's two sides, each in the facet's own unit. */
function sides(facet: FingerprintFacet): { voice: string; text: string } {
  return facet.unit === 'text'
    ? { voice: termsText(facet.key, facet.voice), text: termsText(facet.key, facet.text) }
    : { voice: numberText(facet.unit, facet.voice), text: numberText(facet.unit, facet.text) }
}

/** The fingerprint comparison (VOICE-62): each counted item by its name, in the order given —
 *  farthest from the voice first — with its headline facet as 내 말투 · the text's value, every
 *  facet on demand, and 알 수 없음 where the text is too short to show the item. */
export function FingerprintComparison({
  items,
  textLabel,
}: {
  items: FingerprintComparisonItem[]
  /** What the compared text is called beside 내 말투 (`이 글`, `AI가 쓴 글`). */
  textLabel: string
}) {
  const { t: tr } = useTranslation('voices')
  const line = (facet: FingerprintFacet) =>
    tr('comparison.headline', { ...sides(facet), label: textLabel })
  return (
    <ul className="mt-2 flex flex-col">
      {items.map((item) => {
        const label = tr(`fingerprint.label.${item.item}`)
        if (item.unknown)
          return (
            // Indented past the chevron a known item's name follows, so the names line up.
            <li key={item.item} className="flex min-h-11 flex-wrap items-center gap-x-2 ps-6">
              <Typography variant="body" as="span" className="text-content-primary">
                {label}
              </Typography>
              <Typography variant="body" as="span" className="text-content-secondary">
                {tr('fingerprint.unknown')}
              </Typography>
            </li>
          )
        const headline = item.facets.find((facet) => facet.key === item.headline)
        return (
          <li key={item.item}>
            <Disclosure
              size="row"
              headingLevel={4}
              title={label}
              lead={
                headline && (
                  <p className={typographyStyles({ variant: 'body', className: 'ms-6' })}>
                    <span className="text-content-secondary">
                      {facetLabel(item.item, headline.key)}
                    </span>{' '}
                    <span className="text-content-primary tabular-nums">{line(headline)}</span>
                  </p>
                )
              }
            >
              <dl className="ms-6 mt-1 flex flex-col gap-1">
                {item.facets.map((facet) => (
                  <div key={facet.key} className="flex flex-wrap gap-x-2">
                    <dt
                      className={typographyStyles({
                        variant: 'body',
                        className: 'text-content-secondary',
                      })}
                    >
                      {facetLabel(item.item, facet.key)}
                    </dt>
                    <dd
                      className={typographyStyles({ variant: 'body', className: 'tabular-nums' })}
                    >
                      {line(facet)}
                    </dd>
                  </div>
                ))}
              </dl>
            </Disclosure>
          </li>
        )
      })}
    </ul>
  )
}
