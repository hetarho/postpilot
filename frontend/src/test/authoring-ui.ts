import { screen } from '@testing-library/react'
import type userEvent from '@testing-library/user-event'

export async function publishAuthoringDraft(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  await user.click(await screen.findByRole('button', { name: '저장할 내용 확인하기' }))
  await user.click(await screen.findByRole('button', { name: /저장하기$/ }))
}
