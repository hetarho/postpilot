/** What a 체험단 campaign asks of a post or a clip (POST-118, CLIP-210): the guide as pasted and
 *  the items the product can count. An absent minimum is "not asked". */
export interface AdvertiserGuide {
  text: string
  keywords: string[]
  minChars?: number
  minPhotos?: number
  hashtags: string[]
}

/** Which counted items a host offers: a clip takes keywords and hashtags only (CLIP-210). */
export type AdvertiserGuideItem = 'keywords' | 'minChars' | 'minPhotos' | 'hashtags'

export const EMPTY_GUIDE: AdvertiserGuide = Object.freeze({
  text: '',
  keywords: [],
  hashtags: [],
}) as AdvertiserGuide

export function isEmptyGuide(guide: AdvertiserGuide): boolean {
  return (
    guide.text.trim() === '' &&
    guide.keywords.length === 0 &&
    guide.hashtags.length === 0 &&
    guide.minChars === undefined &&
    guide.minPhotos === undefined
  )
}

export function sameGuide(a: AdvertiserGuide, b: AdvertiserGuide): boolean {
  return (
    a.text === b.text &&
    a.minChars === b.minChars &&
    a.minPhotos === b.minPhotos &&
    a.keywords.length === b.keywords.length &&
    a.keywords.every((keyword, index) => keyword === b.keywords[index]) &&
    a.hashtags.length === b.hashtags.length &&
    a.hashtags.every((hashtag, index) => hashtag === b.hashtags[index])
  )
}

export function copyGuide(guide: AdvertiserGuide): AdvertiserGuide {
  return { ...guide, keywords: [...guide.keywords], hashtags: [...guide.hashtags] }
}

/** The free-text lines the checklist offers to tick (POST-119): every non-empty line, trimmed. */
export function guideLines(guide: AdvertiserGuide): string[] {
  return guide.text
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
}
