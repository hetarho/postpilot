import { screen, waitFor } from '@testing-library/react'
import { expect } from 'vitest'
import type userEvent from '@testing-library/user-event'

export async function publishAuthoringDraft(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  await user.click(await screen.findByRole('button', { name: '저장할 내용 확인하기' }))
  await user.click(await screen.findByRole('button', { name: /저장하기$/ }))
}

/** A template opens on its conversation (owner 2026-11-17); editing its text by hand is behind
 *  the heading row's ⋯. */
export async function openTemplateDirect(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '더 보기' }))
  await user.click(await screen.findByRole('menuitem', { name: '직접 고치기' }))
  await screen.findByLabelText('이름')
}

/** Keeps the direct edit, returns to the conversation and saves from its heading row. */
export async function publishTemplateConversation(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
  const save = await screen.findByRole('button', { name: '저장' })
  await waitFor(() => expect(save).toBeEnabled())
  await user.click(save)
  await user.click(
    await screen.findByRole('button', { name: /^(변경사항 저장|새 템플릿으로 저장)$/ }),
  )
}
