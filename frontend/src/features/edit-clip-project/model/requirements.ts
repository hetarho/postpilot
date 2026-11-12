import { keywordCoverage } from '@/entities/advertiser-guide'

/** A keyword is held when the title, the instruction or a switched-on answer of the selected
 *  template contains it, each text searched on its own (CLIP-228). Generated content is never
 *  read. */
export function clipKeywordCoverage(
  title: string,
  instruction: string,
  switchedOnAnswers: readonly string[],
  keywords: readonly string[],
): Array<{ keyword: string; held: boolean }> {
  return keywordCoverage(keywords, [title, instruction, ...switchedOnAnswers])
}
