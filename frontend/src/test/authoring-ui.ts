import { screen, waitFor, within } from '@testing-library/react'
import type userEvent from '@testing-library/user-event'

/** Keeps a direct edit and publishes it, in whichever layout the editor uses: the step layout
 *  reviews and then saves; the conversation saves from its heading row after one confirmation. */
export async function publishAuthoringDraft(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  const next = await screen.findByRole('button', {
    name: /^(저장할 내용 확인하기|저장|템플릿으로 저장)$/,
  })
  if (next.textContent === '저장할 내용 확인하기') {
    await user.click(next)
    await user.click(await screen.findByRole('button', { name: /저장하기$/ }))
    return
  }
  await waitFor(() => expect(next).toBeEnabled())
  await user.click(next)
  const dialog = within(await screen.findByRole('dialog'))
  await user.click(dialog.getByRole('button', { name: '저장' }))
}
