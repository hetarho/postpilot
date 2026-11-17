import { screen } from '@testing-library/react'
import type userEvent from '@testing-library/user-event'

/** Keeps a direct edit and saves it, in either authoring layout: the step layout's 저장할 내용
 *  확인하기 → 저장하기, or the conversation's one 저장 in its heading row. */
export async function publishAuthoringDraft(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  const next = await screen.findByRole('button', { name: /^(저장할 내용 확인하기|저장)$/ })
  await user.click(next)
  if (next.textContent?.trim() === '저장') return
  await user.click(await screen.findByRole('button', { name: /저장하기$/ }))
}
