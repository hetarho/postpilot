import {
  ADVERTISER_HASHTAG_MAX,
  ADVERTISER_ITEM_MAX_CHARS,
  ADVERTISER_KEYWORD_MAX,
  ADVERTISER_MIN_CHARS_RANGE,
  ADVERTISER_MIN_PHOTOS_RANGE,
  type AdvertiserGuide,
} from '@/entities/advertiser-guide'

export type RequirementSuggestion =
  | { id: string; kind: 'keywords'; value: string }
  | { id: string; kind: 'hashtags'; value: string }
  | { id: string; kind: 'minChars'; value: number }
  | { id: string; kind: 'minPhotos'; value: number }

const folded = (text: string) => text.replace(/^#/, '').replace(/\s+/gu, '').toLowerCase()

/** A conservative local scan: candidates only, never a guide mutation or a model call. */
export function suggestRequirements(text: string): RequirementSuggestion[] {
  const found: RequirementSuggestion[] = []
  const add = (kind: RequirementSuggestion['kind'], raw: string) => {
    const suggestion = editSuggestion(kind, raw)
    if (suggestion && !found.some((item) => item.id === suggestion.id)) found.push(suggestion)
  }
  for (const match of text.matchAll(/#([\p{L}\p{N}_]+)/gu)) add('hashtags', match[1])
  const number = '[0-9]+(?:,[0-9]{3})*'
  const scans: Array<[RequirementSuggestion['kind'], RegExp]> = [
    [
      'minChars',
      new RegExp(
        `(?:최소\\s*(${number})\\s*자|(${number})\\s*자\\s*이상|(?:at least|minimum|over)\\s*(${number})\\s*(?:characters|chars))`,
        'gi',
      ),
    ],
    [
      'minPhotos',
      new RegExp(
        `(?:사진\\s*(?:최소\\s*)?(${number})\\s*장\\s*이상|최소\\s*(${number})\\s*장(?:의)?\\s*사진|사진\\s*최소\\s*(${number})\\s*장|(?:at least|minimum)\\s*(${number})\\s*(?:photos|images))`,
        'gi',
      ),
    ],
  ]
  for (const [kind, pattern] of scans) {
    for (const match of text.matchAll(pattern)) add(kind, match.slice(1).find(Boolean) ?? '')
  }
  for (const match of text.matchAll(
    /(?:필수\s*키워드|제목\s*키워드|본문\s*키워드|키워드|(?:required\s+)?keywords?)\s*[:：]\s*([^\n]+)/gi,
  )) {
    for (const phrase of match[1].split(/[,，/|·]/u))
      add('keywords', phrase.replace(/^["'“‘]|["'”’]$/gu, ''))
  }
  for (const match of text.matchAll(
    /["“‘]([^"”’\n]+)["”’]\s*(?:를|을)?\s*(?:꼭\s*)?(?:포함|넣어)/gu,
  ))
    add('keywords', match[1])
  return found
}

/** The same bounds guard both scanned and owner-edited suggestions. */
export function editSuggestion(
  kind: RequirementSuggestion['kind'],
  raw: string,
): RequirementSuggestion | undefined {
  if (kind === 'minChars' || kind === 'minPhotos') {
    const value = Number(raw.replace(/,/g, '').trim())
    const range = kind === 'minChars' ? ADVERTISER_MIN_CHARS_RANGE : ADVERTISER_MIN_PHOTOS_RANGE
    return Number.isInteger(value) && value >= range.min && value <= range.max
      ? { kind, value, id: `${kind}:${value}` }
      : undefined
  }
  const value = raw.trim().replace(/^#/, '').trim()
  if (
    !value ||
    [...value].length > ADVERTISER_ITEM_MAX_CHARS ||
    (kind === 'hashtags' && /\s/u.test(value))
  )
    return undefined
  return { kind, value, id: `${kind}:${folded(value)}` }
}

/** Confirmation is the only route from a candidate into the persisted counted items. */
export function acceptRequirement(
  guide: AdvertiserGuide,
  suggestion: RequirementSuggestion,
): AdvertiserGuide {
  if (suggestion.kind === 'minChars' || suggestion.kind === 'minPhotos') {
    return { ...guide, [suggestion.kind]: suggestion.value }
  }
  const needle = folded(suggestion.value)
  const values = guide[suggestion.kind]
  const max = suggestion.kind === 'keywords' ? ADVERTISER_KEYWORD_MAX : ADVERTISER_HASHTAG_MAX
  if (values.length >= max || values.some((value) => folded(value) === needle)) return guide
  return { ...guide, [suggestion.kind]: [...values, suggestion.value] }
}

export function suggestionAlreadyAdded(
  guide: AdvertiserGuide,
  suggestion: RequirementSuggestion,
): boolean {
  if (suggestion.kind === 'minChars' || suggestion.kind === 'minPhotos')
    return guide[suggestion.kind] === suggestion.value
  const needle = folded(suggestion.value)
  return guide[suggestion.kind].some((value) => folded(value) === needle)
}

/** Only enabled, selected-template answers count as material for the next run. Each value is
 *  searched separately, so two partial answers cannot accidentally form one required keyword. */
export function draftKeywordCoverage(
  keywords: readonly string[],
  material: { title: string; memo: string; answers: readonly { text: string; enabled: boolean }[] },
): Array<{ keyword: string; present: boolean }> {
  const texts = [
    material.title,
    material.memo,
    ...material.answers.filter((answer) => answer.enabled).map((answer) => answer.text),
  ]
  return keywords.map((keyword) => ({
    keyword,
    present:
      Boolean(folded(keyword)) && texts.some((text) => folded(text).includes(folded(keyword))),
  }))
}
