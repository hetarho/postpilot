import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { GenerationOptions } from './GenerationOptions'

afterEach(cleanup)

function renderOptions(
  overrides: Partial<Parameters<typeof GenerationOptions>[0]> = {},
  posts: { calls: string[]; saves: Array<number | undefined> } = { calls: [], saves: [] },
) {
  const transport = createFakeAuthTransport({
    posts: {
      posts: [{ slug: 'post-a' }],
      calls: posts.calls,
      generationOptionSaves: posts.saves,
    },
  })
  const onSaved = vi.fn()
  const onClose = vi.fn()
  render(
    <GenerationOptions
      slug="post-a"
      tagCount={4}
      disabled={false}
      onSaved={onSaved}
      onClose={onClose}
      {...overrides}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { onSaved, onClose, ...posts }
}

describe('GenerationOptions', () => {
  // POST-63: no enabling tick for the count — the field is always there, holding the post's value.
  it('shows the tag count beside the length, prefilled from the post', () => {
    renderOptions({ tagCount: 4 })
    expect(screen.getByLabelText('목표 글자 수 사용')).not.toBeChecked()
    expect(screen.getByLabelText('태그 개수')).toHaveValue(4)
    expect(screen.queryByLabelText('목표 글자 수')).not.toBeInTheDocument()
  })

  it('refuses a count outside 1–10 and says the range', async () => {
    const user = userEvent.setup()
    renderOptions()
    const field = screen.getByLabelText('태그 개수')
    await user.clear(field)
    await user.type(field, '11')
    expect(screen.getByText('1–10개로 입력해 주세요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.clear(field)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.type(field, '10')
    expect(screen.queryByText('1–10개로 입력해 주세요.')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
  })

  // One 저장 carries both numbers; the length stays natural when the box was never ticked.
  it('saves both values in one call and reports them back', async () => {
    const user = userEvent.setup()
    const { onSaved, onClose, calls } = renderOptions()
    const field = screen.getByLabelText('태그 개수')
    await user.clear(field)
    await user.type(field, '7')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('SavePostGenerationOptions'))
    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({ targetLength: undefined, tagCount: 7 }),
    )
    expect(onClose).toHaveBeenCalled()
  })

  it('greys the count with the length while a job runs', () => {
    renderOptions({ disabled: true })
    expect(screen.getByLabelText('태그 개수')).toBeDisabled()
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeDisabled()
  })
})
