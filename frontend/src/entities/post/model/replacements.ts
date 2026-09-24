// Where a write's replacement candidates still stand in the content on screen (GEN-53, POST-79).
// Pure on purpose (ARCH-18): the stored candidates are never edited, so every stale one is dropped
// here, at render, by the same containment rules the server applies when it keeps them (T330).
import { BlockType, type PostContent } from '@/shared/api'
import { REPLACEMENT_PHRASES_MAX } from '../config'
import { blockWith, postContentWith } from './content'

export type ReplacementSurface = 'title' | 'tag' | 'body'

/** One span a write offered phrases for, as stored: where it stood when the write returned it. */
export interface ReplacementCandidate {
  surface: ReplacementSurface
  /** The tag's index for a tag, the block's for the body, and 0 for the title. */
  index: number
  source: string
  phrases: string[]
}

/** One piece of rendered text a mark can stand in. `item` is set only for a LIST block. */
export type TextAt =
  | { surface: 'title' }
  | { surface: 'tag'; index: number }
  | { surface: 'body'; index: number; item?: number }

/** A candidate that still stands: its text's place, the first occurrence of its source there, and
 *  the phrases it may still be replaced with. */
export interface ReplacementSpan {
  at: TextAt
  start: number
  end: number
  source: string
  phrases: string[]
}

/** The server's tag identity (`ValidateContent`): runs of whitespace collapsed, leading `#`s
 *  dropped, case kept. A take that made two tags equal by it would be refused. */
export function canonicalTag(tag: string): string {
  return tag.split(/\s+/).filter(Boolean).join(' ').replace(/^#+/, '').trim()
}

function samePlace(a: TextAt, b: TextAt): boolean {
  if (a.surface !== b.surface) return false
  if (a.surface === 'title' || b.surface === 'title') return true
  if (a.index !== b.index) return false
  return a.surface !== 'body' || b.surface !== 'body' || a.item === b.item
}

/** The text a candidate's place holds, or undefined for a place no mark stands on: a title index
 *  other than 0, an index out of range, a slot, or a block with no text of its own. A LIST marks
 *  the lowest-numbered item that contains the source. */
function textOf(
  content: PostContent,
  candidate: ReplacementCandidate,
): { at: TextAt; text: string } | undefined {
  switch (candidate.surface) {
    case 'title':
      return candidate.index === 0 ? { at: { surface: 'title' }, text: content.title } : undefined
    case 'tag': {
      const tag = content.tags[candidate.index]
      return tag === undefined
        ? undefined
        : { at: { surface: 'tag', index: candidate.index }, text: tag }
    }
    case 'body': {
      const index = candidate.index
      const block = content.blocks[index]
      if (!block) return undefined
      switch (block.type) {
        case BlockType.TEXT:
          return block.slot ? undefined : { at: { surface: 'body', index }, text: block.content }
        case BlockType.HEADING:
        case BlockType.QUOTE:
          return { at: { surface: 'body', index }, text: block.content }
        case BlockType.LIST: {
          const item = block.items.findIndex((text) => text.includes(candidate.source))
          return item === -1
            ? undefined
            : { at: { surface: 'body', index, item }, text: block.items[item] }
        }
        default:
          return undefined
      }
    }
  }
}

/** The candidates that still stand in `content`, in answer order: each marks the first occurrence
 *  of its source at its place, offers at most three phrases — none blank, none its own source, and
 *  for a tag none that would make it canonically empty or equal to another tag — and yields to an
 *  earlier one it overlaps. Anything else renders nothing and says nothing (GEN-53). */
export function visibleSpans(
  content: PostContent,
  candidates: readonly ReplacementCandidate[],
): ReplacementSpan[] {
  const spans: ReplacementSpan[] = []
  for (const candidate of candidates) {
    if (candidate.source.trim() === '') continue
    const place = textOf(content, candidate)
    if (!place) continue
    // Case-sensitive, like the server's containment.
    const start = place.text.indexOf(candidate.source)
    if (start === -1) continue
    const end = start + candidate.source.length
    let phrases = [...new Set(candidate.phrases)].filter(
      (phrase) => phrase.trim() !== '' && phrase !== candidate.source,
    )
    const at = place.at
    if (at.surface === 'tag') {
      const others = new Set(
        content.tags.filter((_, index) => index !== at.index).map(canonicalTag),
      )
      phrases = phrases.filter((phrase) => {
        const taken = canonicalTag(place.text.slice(0, start) + phrase + place.text.slice(end))
        return taken !== '' && !others.has(taken)
      })
    }
    phrases = phrases.slice(0, REPLACEMENT_PHRASES_MAX)
    if (phrases.length === 0) continue
    if (spans.some((span) => samePlace(span.at, at) && span.start < end && start < span.end))
      continue
    spans.push({ at, start, end, source: candidate.source, phrases })
  }
  return spans
}

/** The spans standing in one piece of rendered text, in reading order. */
export function spansAt(spans: readonly ReplacementSpan[], at: TextAt): ReplacementSpan[] {
  return spans.filter((span) => samePlace(span.at, at)).sort((a, b) => a.start - b.start)
}

/** The content with `phrase` spliced over the span — an ordinary edit, which the content queue
 *  saves like any other (POST-80). The input is never mutated. */
export function applyReplacement(
  content: PostContent,
  span: ReplacementSpan,
  phrase: string,
): PostContent {
  const splice = (text: string) => text.slice(0, span.start) + phrase + text.slice(span.end)
  const at = span.at
  switch (at.surface) {
    case 'title':
      return postContentWith(content, { title: splice(content.title) })
    case 'tag':
      return postContentWith(content, {
        tags: content.tags.map((tag, index) => (index === at.index ? splice(tag) : tag)),
      })
    case 'body':
      return postContentWith(content, {
        blocks: content.blocks.map((block, index) => {
          if (index !== at.index) return block
          if (at.item === undefined) return blockWith(block, { content: splice(block.content) })
          return blockWith(block, {
            items: block.items.map((item, itemIndex) =>
              itemIndex === at.item ? splice(item) : item,
            ),
          })
        }),
      })
  }
}
