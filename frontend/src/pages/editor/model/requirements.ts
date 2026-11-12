import { keywordCoverage } from '@/entities/advertiser-guide'

/** Only enabled, selected-template answers count as material for the next run. Each value is
 *  searched separately, so two partial answers cannot accidentally form one required keyword. */
export function draftKeywordCoverage(
  keywords: readonly string[],
  material: { title: string; memo: string; answers: readonly { text: string; enabled: boolean }[] },
): Array<{ keyword: string; present: boolean }> {
  return keywordCoverage(keywords, [
    material.title,
    material.memo,
    ...material.answers.filter((answer) => answer.enabled).map((answer) => answer.text),
  ]).map(({ keyword, held }) => ({ keyword, present: held }))
}
