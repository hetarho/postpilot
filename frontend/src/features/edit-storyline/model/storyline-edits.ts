import type { PostStoryline, PostStorylineParagraph } from '@/entities/post'

/** The owner's own storyline edits (POST-96), as pure functions over the whole paragraph list:
 *  the list is what the draft queue saves, and the server validates every save. None of them adds
 *  or removes a paragraph — the count is the storyline job's. */

export function withText(
  paragraphs: readonly PostStorylineParagraph[],
  index: number,
  text: string,
): PostStorylineParagraph[] {
  return paragraphs.map((paragraph, at) => (at === index ? { ...paragraph, text } : paragraph))
}

/** Takes `file` out of whichever paragraph holds it. */
export function withoutFile(
  paragraphs: readonly PostStorylineParagraph[],
  file: string,
): PostStorylineParagraph[] {
  return paragraphs.map((paragraph) =>
    paragraph.files.includes(file)
      ? { ...paragraph, files: paragraph.files.filter((name) => name !== file) }
      : paragraph,
  )
}

/** Puts `file` at the end of paragraph `to`, taking it out of wherever it was — a move and a put
 *  back are the same edit. Moving a file to the paragraph that holds it changes nothing. */
export function withFileIn(
  paragraphs: readonly PostStorylineParagraph[],
  file: string,
  to: number,
): PostStorylineParagraph[] {
  if (paragraphs[to]?.files.includes(file)) return [...paragraphs]
  return withoutFile(paragraphs, file).map((paragraph, at) =>
    at === to ? { ...paragraph, files: [...paragraph.files, file] } : paragraph,
  )
}

/** The attachments the storyline was made with that no paragraph of `paragraphs` holds — the ones
 *  the owner can put back. Read against the server's storyline, whose held files and taken-out
 *  files together are what it was made with and is still attached; an attachment added later is
 *  never among them (POST-99). */
export function takenOutFiles(
  server: Pick<PostStoryline, 'paragraphs' | 'takenOutFiles'>,
  paragraphs: readonly PostStorylineParagraph[],
): string[] {
  const held = new Set(paragraphs.flatMap((paragraph) => paragraph.files))
  const candidates = [
    ...server.paragraphs.flatMap((paragraph) => paragraph.files),
    ...server.takenOutFiles,
  ]
  return [...new Set(candidates)].filter((file) => !held.has(file))
}
