import { screen, waitFor } from '@testing-library/react'
import { expect } from 'vitest'
import type userEvent from '@testing-library/user-event'

export async function publishAuthoringDraft(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  await user.click(await screen.findByRole('button', { name: '저장할 내용 확인하기' }))
  await user.click(await screen.findByRole('button', { name: /저장하기$/ }))
}

/** The conversation layout's secondary path: ⋯ → 직접 고치기. */
export async function openConversationDirect(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '더 보기' }))
  await user.click(await screen.findByRole('menuitem', { name: '직접 고치기' }))
}

/** Keeps the direct edit, then saves from the conversation's heading row in one press. */
export async function saveConversationDraft(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  const save = await screen.findByRole('button', { name: /^(저장|변경 저장)$/ })
  await waitFor(() => expect(save).toBeEnabled())
  await user.click(save)
}
