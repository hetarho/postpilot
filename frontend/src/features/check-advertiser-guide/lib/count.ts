import { guideLines, type AdvertiserGuide } from '@/entities/advertiser-guide'
import { BlockType, type PostContent } from '@/shared/api'
import { blockPhotos, walkBlocks } from '@/shared/lib'

type CountedContent = Pick<PostContent, 'title' | 'tags' | 'blocks'>

/** The body's human-readable texts the checklist reads (POST-119): TEXT, HEADING, QUOTE and LIST
 *  text and photo captions. Never the title, the tags or a photo marker — those are not what a
 *  reader counts as the post's writing. */
export function bodyTexts(content: Pick<PostContent, 'blocks'>): string[] {
  return walkBlocks(content, (block) => {
    switch (block.type) {
      case BlockType.TEXT:
      case BlockType.HEADING:
      case BlockType.QUOTE:
        return [block.content]
      case BlockType.LIST:
        return block.items
      case BlockType.IMAGE:
      case BlockType.GALLERY:
        return [block.caption]
      default:
        return []
    }
  })
    .flat()
    .filter((text) => text !== '')
}

/** A keyword match ignores case and every space on both sides (POST-119). */
const folded = (text: string) => text.toLowerCase().replace(/\s+/g, '')

/** Whether a keyword stands in the title or in one of the body's texts. Each text is searched on
 *  its own, so a keyword never matches across two blocks. */
export function keywordPresent(content: CountedContent, keyword: string): boolean {
  const needle = folded(keyword)
  if (needle === '') return false
  return [content.title, ...bodyTexts(content)].some((text) => folded(text).includes(needle))
}

export interface CharacterCount {
  withSpaces: number
  withoutSpaces: number
}

/** The body's characters with and without whitespace, counted as Unicode code points like the
 *  server counts them. */
export function countCharacters(content: Pick<PostContent, 'blocks'>): CharacterCount {
  let withSpaces = 0
  let withoutSpaces = 0
  for (const text of bodyTexts(content)) {
    withSpaces += [...text].length
    withoutSpaces += [...text.replace(/\s+/g, '')].length
  }
  return { withSpaces, withoutSpaces }
}

/** Every photo placed in the content, alone or inside a group; a marker with no file is not a
 *  photo the reader sees. */
export function countPhotos(content: Pick<PostContent, 'blocks'>): number {
  return walkBlocks(content, blockPhotos)
    .flat()
    .filter((file) => file !== '').length
}

/** A hashtag matches a tag ignoring a leading '#' (POST-119), case and spaces. */
const hashtagKey = (tag: string) => folded(tag.trim().replace(/^#/, ''))

export function hashtagPresent(tags: readonly string[], hashtag: string): boolean {
  const key = hashtagKey(hashtag)
  return key !== '' && tags.some((tag) => hashtagKey(tag) === key)
}

export type CountedItem =
  | { kind: 'keyword'; keyword: string; met: boolean }
  | { kind: 'characters'; minimum: number; count: CharacterCount; met: boolean }
  | { kind: 'photos'; minimum: number; count: number; met: boolean }
  | { kind: 'hashtag'; hashtag: string; met: boolean }

/** The items the guide asks for that the product can count, in the guide form's order. The
 *  character minimum is met by the count without spaces — the stricter of the two the checklist
 *  shows, so a met mark holds whichever way the campaign counts. */
export function countedItems(content: CountedContent, guide: AdvertiserGuide): CountedItem[] {
  const items: CountedItem[] = guide.keywords.map((keyword) => ({
    kind: 'keyword',
    keyword,
    met: keywordPresent(content, keyword),
  }))
  if (guide.minChars !== undefined) {
    const count = countCharacters(content)
    items.push({
      kind: 'characters',
      minimum: guide.minChars,
      count,
      met: count.withoutSpaces >= guide.minChars,
    })
  }
  if (guide.minPhotos !== undefined) {
    const count = countPhotos(content)
    items.push({ kind: 'photos', minimum: guide.minPhotos, count, met: count >= guide.minPhotos })
  }
  for (const hashtag of guide.hashtags)
    items.push({ kind: 'hashtag', hashtag, met: hashtagPresent(content.tags, hashtag) })
  return items
}

/** The free-text lines with their ticks (POST-119). A tick is the line's text, so an edited line
 *  starts unticked. */
export function tickedLines(
  guide: AdvertiserGuide,
  ticks: readonly string[],
): Array<{ line: string; ticked: boolean }> {
  const ticked = new Set(ticks)
  return guideLines(guide).map((line) => ({ line, ticked: ticked.has(line) }))
}

/** The ticks after one line changes: only lines the guide still holds are kept, so a tick of a
 *  line edited away is not carried forever. */
export function toggleTick(
  guide: AdvertiserGuide,
  ticks: readonly string[],
  line: string,
  on: boolean,
): string[] {
  const ticked = new Set(ticks)
  if (on) ticked.add(line)
  else ticked.delete(line)
  return guideLines(guide).filter((candidate, index, lines) => {
    return ticked.has(candidate) && lines.indexOf(candidate) === index
  })
}
