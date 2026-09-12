import { CLIP_RAPID, CLIP_TIMING } from '@/shared/config'
import type { ClipCaption, ClipEditCut } from './edit-plan'

export function copyChars(text: string): number {
  return Array.from(text).filter((c) => !/[\s\p{P}\p{S}]/u.test(c)).length
}

export function isRapidCut(cut: ClipEditCut): boolean {
  return cut.copies.length > 0 && cut.copies.every((c) => c.pace === 'rapid')
}

export function rapidPhrases(text: string): string[] {
  const words: string[] = []
  const segmenter = new Intl.Segmenter('ko', { granularity: 'grapheme' })
  for (const word of text.trim().split(/\s+/u).filter(Boolean)) {
    let part = ''
    for (const { segment } of segmenter.segment(word)) {
      if (copyChars(part + segment) > CLIP_RAPID.max_chars) {
        words.push(part)
        part = ''
      }
      part += segment
    }
    if (part) words.push(part)
  }
  const phrases: string[] = []
  for (const word of words) {
    const last = phrases.at(-1)
    if (
      last &&
      !(phrases.length === 1 && copyChars(last) <= CLIP_RAPID.short_chars) &&
      copyChars(last + word) <= CLIP_RAPID.target_chars &&
      !/[.!?,;:]$/u.test(last)
    )
      phrases[phrases.length - 1] = `${last} ${word}`
    else phrases.push(word)
  }
  return phrases
}

/** Word-preserving editorial timing. This does not imply speech alignment. */
export function splitRapid(seed: ClipCaption, start: number, end: number): ClipCaption[] | null {
  const phrases = rapidPhrases(seed.text)
  const minimum = phrases.length * CLIP_RAPID.min_ms
  const room = end - start
  if (!phrases.length || phrases.length > CLIP_RAPID.max_per_cut || start < 0 || room < minimum)
    return null
  const durations = phrases.map((text) => {
    const chars = copyChars(text)
    return chars <= CLIP_RAPID.short_chars
      ? CLIP_RAPID.short_ms
      : chars <= CLIP_RAPID.target_chars
        ? CLIP_RAPID.medium_ms
        : CLIP_RAPID.long_ms
  })
  const total = durations.reduce((sum, value) => sum + value, 0)
  return phrases.map((text, i) => {
    const duration =
      total <= room
        ? durations[i]!
        : CLIP_RAPID.min_ms +
          Math.floor(((durations[i]! - CLIP_RAPID.min_ms) * (room - minimum)) / (total - minimum))
    const copy: ClipCaption = {
      ...seed,
      text,
      pace: 'rapid',
      keyword: text.includes(seed.keyword) ? seed.keyword : '',
      startMs: start,
      endMs: start + duration,
    }
    start += duration
    return copy
  })
}

export function canSplitRapid(cut: ClipEditCut): boolean {
  const seed = cut.copies[0]
  return (
    !!seed &&
    splitRapid(
      { ...seed, text: cut.copies.map((c) => c.text.trim()).join(' ') },
      CLIP_TIMING.copy_lead_ms,
      cut.endMs - cut.startMs - CLIP_TIMING.copy_lead_ms,
    ) !== null
  )
}

export function canAddRapid(cut: ClipEditCut): boolean {
  const last = cut.copies.at(-1)
  return (
    isRapidCut(cut) &&
    cut.copies.length < CLIP_RAPID.max_per_cut &&
    !!last &&
    cut.endMs - cut.startMs - CLIP_TIMING.copy_lead_ms - last.endMs >= CLIP_RAPID.min_ms
  )
}
