import { describe, expect, it, vi } from 'vitest'
import { Code } from '@connectrpc/connect'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { VoiceFeedbackReason } from '@/shared/api'
import { LONG_PRESS_MS } from '@/shared/config'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { connectAppError } from '@/test/app-error'
import { SentenceFeedback } from './SentenceFeedback'

const FIRST = '첫 문장입니다.'
const SECOND = '둘째 문장입니다.'
const EDITED = '고쳐 쓴 문장입니다.'

function renderFeedback(text: string) {
  const sentenceFeedback: Array<{ sentenceRef: string; authoredText: string }> = []
  const transport = createFakeAuthTransport({ voice: { sentenceFeedback } })
  const view = render(
    <SentenceFeedback postSlug="post" text={text} beforeSubmit={() => Promise.resolve()} />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { sentenceFeedback, view }
}

async function submit() {
  await userEvent.click(screen.getByRole('button', { name: '문장 의견' }))
  await userEvent.click(await screen.findByRole('button', { name: '의견 남기기' }))
}

describe('SentenceFeedback', () => {
  it('submits a sentence from the current text after the block was edited', async () => {
    const { sentenceFeedback, view } = renderFeedback(`${FIRST} ${SECOND}`)

    // The block is edited while the control stays mounted; nothing in the dropdown was touched.
    view.rerender(
      <SentenceFeedback postSlug="post" text={EDITED} beforeSubmit={() => Promise.resolve()} />,
    )
    await submit()

    await waitFor(() => expect(sentenceFeedback).toHaveLength(1))
    expect(sentenceFeedback[0].sentenceRef).toBe(EDITED)
    expect(sentenceFeedback[0].authoredText).toBe(EDITED)
  })

  it('keeps an explicitly chosen sentence while the text still contains it', async () => {
    const { sentenceFeedback, view } = renderFeedback(`${FIRST} ${SECOND}`)
    await userEvent.click(screen.getByRole('button', { name: '문장 의견' }))
    await userEvent.selectOptions(await screen.findByLabelText('문장'), SECOND)

    // A later edit appends a sentence but leaves the chosen one in place.
    view.rerender(
      <SentenceFeedback
        postSlug="post"
        text={`${FIRST} ${SECOND} ${EDITED}`}
        beforeSubmit={() => Promise.resolve()}
      />,
    )
    await userEvent.click(await screen.findByRole('button', { name: '의견 남기기' }))

    await waitFor(() => expect(sentenceFeedback).toHaveLength(1))
    expect(sentenceFeedback[0].sentenceRef).toBe(SECOND)
  })

  it('survives an edit without losing the open dialog or the chosen reason', async () => {
    const { view } = renderFeedback(`${FIRST} ${SECOND}`)
    await userEvent.click(screen.getByRole('button', { name: '문장 의견' }))
    await userEvent.selectOptions(await screen.findByLabelText('이유'), '종결어미')

    view.rerender(
      <SentenceFeedback
        postSlug="post"
        text={`${FIRST} ${EDITED}`}
        beforeSubmit={() => Promise.resolve()}
      />,
    )

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByLabelText('이유')).toHaveValue(String(VoiceFeedbackReason.ENDING))
  })

  it('clears the long-press timer when it unmounts mid-press', async () => {
    const set = vi.spyOn(window, 'setTimeout')
    const clear = vi.spyOn(window, 'clearTimeout')
    const { view } = renderFeedback(`${FIRST} ${SECOND}`)
    const button = screen.getByRole('button', { name: '문장 의견' })

    // Pointer-down starts the timeout; the block is removed before pointer-up ever arrives.
    await userEvent.pointer({ target: button, keys: '[MouseLeft>]' })
    const longPress = set.mock.calls.findIndex(([, delay]) => delay === LONG_PRESS_MS)
    expect(longPress).toBeGreaterThanOrEqual(0)
    view.unmount()

    expect(clear).toHaveBeenCalledWith(set.mock.results[longPress].value)
    set.mockRestore()
    clear.mockRestore()
  })

  it('shows the structured reason when saving the edited sentence prerequisite fails', async () => {
    const transport = createFakeAuthTransport()
    render(
      <SentenceFeedback
        postSlug="post"
        text={FIRST}
        beforeSubmit={() =>
          Promise.reject(connectAppError('POST_CONTENT_INVALID', Code.InvalidArgument))
        }
      />,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )

    await submit()

    expect(await screen.findByRole('alert')).toHaveTextContent('글 내용을 확인해 주세요.')
    expect(screen.queryByText('private backend prose')).not.toBeInTheDocument()
  })
})
