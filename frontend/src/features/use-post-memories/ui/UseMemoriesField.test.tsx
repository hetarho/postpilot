import { describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { FakePostsOptions } from '@/test/posts'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { UseMemoriesField } from './UseMemoriesField'

function renderField(useMemory: boolean, posts: FakePostsOptions = {}) {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    posts: { posts: [{ slug: 'draft' }], ...posts },
  })
  return render(<UseMemoriesField slug="draft" useMemory={useMemory} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
}

describe('기억 사용', () => {
  // POST-71/MEM-18: default off, and the label says what turning it on does.
  it('reads the post and starts off', () => {
    renderField(false)
    expect(screen.getByRole('checkbox', { name: '기억 사용' })).not.toBeChecked()
    expect(
      screen.getByText('이 글을 쓸 때 저장해 둔 기억 중 관련 있는 것을 함께 참고해요.'),
    ).toBeInTheDocument()
  })

  it('shows a post that opted in as checked', () => {
    renderField(true)
    expect(screen.getByRole('checkbox', { name: '기억 사용' })).toBeChecked()
  })

  // It autosaves on the toggle — there is nothing to confirm, because the flag is an option of
  // the next RUN and saving it moves no status, revision or baseline.
  it('autosaves the flag on the toggle and sends nothing else', async () => {
    const user = userEvent.setup()
    const memoryOptionSaves: Array<boolean | undefined> = []
    const generationOptionSaves: Array<number | undefined> = []
    renderField(false, { memoryOptionSaves, generationOptionSaves })

    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))

    await waitFor(() => expect(memoryOptionSaves).toEqual([true]))
    // Presence is the edit unit: the two numbers the brief owns were not part of this save.
    expect(generationOptionSaves).toEqual([undefined])
    expect(screen.getByRole('checkbox', { name: '기억 사용' })).toBeChecked()
  })

  it('turns it back off', async () => {
    const user = userEvent.setup()
    const memoryOptionSaves: Array<boolean | undefined> = []
    renderField(true, { memoryOptionSaves })

    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))

    await waitFor(() => expect(memoryOptionSaves).toEqual([false]))
  })

  // A refused save puts the box back where the post says it is: nothing was saved, so the
  // screen must not claim otherwise.
  it('reverts the box and says so when the save is refused', async () => {
    const user = userEvent.setup()
    renderField(false, { optionSaveFails: true })

    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))

    expect(
      await screen.findByText('설정을 저장하지 못했어요. 다시 눌러 주세요.'),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole('checkbox', { name: '기억 사용' })).not.toBeChecked(),
    )
  })
})
