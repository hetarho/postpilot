import type { PostStorylineParagraph } from '@/entities/post'

/** The order the large view walks (POST-100): paragraph 1's attachments in their order, then
 *  paragraph 2's, …, then the taken-out ones. Only an attachment `canView` accepts enters it — one
 *  still converting, or gone, has nothing to show — and a file appears once however it is listed. */
export function storylineViewOrder(
  paragraphs: readonly PostStorylineParagraph[],
  takenOut: readonly string[],
  canView: (file: string) => boolean,
): string[] {
  const files = [...paragraphs.flatMap((paragraph) => paragraph.files), ...takenOut]
  return [...new Set(files)].filter(canView)
}
