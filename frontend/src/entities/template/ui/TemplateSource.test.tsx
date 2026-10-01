import { afterEach, describe, expect, it, vi } from 'vitest'
import { useState, type ReactElement } from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import i18next from 'i18next'
import { GetFormatGuideResponseSchema, TemplateService } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import {
  FAKE_FORMAT_GUIDE,
  registerTemplateService,
  type FakeTemplatesOptions,
} from '@/test/templates'
import { TEMPLATE_LIMITS } from '../model/types'
import { TemplateSource } from './TemplateSource'

const originalClipboard = navigator.clipboard

function setClipboard(value: Pick<Clipboard, 'writeText'> | Clipboard | undefined) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

afterEach(async () => {
  setClipboard(originalClipboard)
  vi.restoreAllMocks()
  await i18next.changeLanguage('ko')
})

/** The guide is the backend's (TMPL-41), so every render carries a transport that serves it. */
function renderSource(ui: ReactElement, options: FakeTemplatesOptions = {}) {
  const transport = createRouterTransport((router) => registerTemplateService(router, options))
  return render(ui, { wrapper: withProviders(transport, createTestQueryClient()) })
}

/** The copy is enabled once the served guide has arrived. */
async function guideLoaded() {
  await waitFor(() => expect(screen.getByRole('button', { name: '형식 안내 복사' })).toBeEnabled())
}

/** Controlled, through a real parent, so what the assertions read is what a caller would save. */
function Source({ initial = '' }: { initial?: string }) {
  const [body, setBody] = useState(initial)
  return (
    <>
      <TemplateSource value={body} onChange={setBody} failure={null} />
      <output data-testid="body">{body}</output>
    </>
  )
}

const body = () => screen.getByTestId('body').textContent ?? ''

describe('the source editor', () => {
  // TMPL-42, TMPL-8, TMPL-19: what is pasted is what is stored. No trim, no normalization, on input or
  // on the way out — an outside AI's body has to survive this field byte for byte.
  it('keeps what is typed byte for byte, outer whitespace included', async () => {
    const user = userEvent.setup()
    renderSource(<Source />)

    const field = screen.getByLabelText('원문')
    await user.click(field)
    // Leading spaces and a trailing newline: exactly what a paste from a chat window carries.
    await user.paste('  <write>인트로</write>\n')

    expect(body()).toBe('  <write>인트로</write>\n')
    expect(field).toHaveValue('  <write>인트로</write>\n')
  })

  it('shows the failure at its line, in words, and marks the field invalid', () => {
    renderSource(
      <TemplateSource
        value="<write>닫히지 않음"
        onChange={() => {}}
        failure={{ line: 1, reason: 'unclosed_tag', area: 'body' }}
      />,
    )

    const field = screen.getByLabelText('원문')
    expect(field).toHaveAttribute('aria-invalid', 'true')
    const message = screen.getByRole('alert')
    expect(message).toHaveTextContent('1번째 줄: 닫히지 않았어요')
    expect(field).toHaveAttribute('aria-describedby', message.id)
  })

  it('shows no failure and no invalid state when the body parses', () => {
    renderSource(
      <TemplateSource value="<write>인트로</write>" onChange={() => {}} failure={null} />,
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByLabelText('원문')).not.toHaveAttribute('aria-invalid')
  })

  it('copies the body and confirms it', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    renderSource(<Source initial="<write>인트로</write>" />)

    await user.click(screen.getByRole('button', { name: '원문 복사' }))
    expect(writeText).toHaveBeenCalledWith('<write>인트로</write>')
    expect(await screen.findByText('복사했어요')).toBeInTheDocument()
  })

  // TMPL-41: the guide is the thing an outside AI is handed, so copying it is the point.
  it('copies the format guide and confirms it', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    renderSource(<Source />)
    await guideLoaded()

    await user.click(screen.getByRole('button', { name: '형식 안내 복사' }))
    expect(writeText).toHaveBeenCalledWith(FAKE_FORMAT_GUIDE.ko)
    expect(await screen.findByText('복사했어요')).toBeInTheDocument()
  })

  // A clipboard the browser refuses is exactly when the user needs the text selected instead.
  it('falls back to selecting the body when the clipboard is unavailable', async () => {
    const user = userEvent.setup()
    setClipboard(undefined)
    renderSource(<Source initial="<write>인트로</write>" />)

    await user.click(screen.getByRole('button', { name: '원문 복사' }))

    const field = screen.getByLabelText('원문')
    expect(field).toHaveFocus()
    expect((field as HTMLTextAreaElement).selectionEnd).toBe('<write>인트로</write>'.length)
    expect(await screen.findByText(/복사가 막혀 있어요/)).toBeInTheDocument()
    expect(screen.queryByText('복사했어요')).not.toBeInTheDocument()
  })

  // The guide's fallback field lives inside a `<details>`, and a selection inside a collapsed
  // element is a selection nobody can see — so the disclosure opens with it.
  it('opens the disclosure and selects the guide when the clipboard is unavailable', async () => {
    const user = userEvent.setup()
    setClipboard(undefined)
    renderSource(<Source />)
    await guideLoaded()

    await user.click(screen.getByRole('button', { name: '형식 안내 복사' }))

    const guideField = screen.getByLabelText('형식 안내 보기')
    expect(guideField).toHaveFocus()
    expect(guideField.closest('details')).toHaveAttribute('open')
    expect(await screen.findByText(/복사가 막혀 있어요/)).toBeInTheDocument()
  })

  // What the disclosure shows and what the button copies are one string.
  it('shows the same guide it copies', async () => {
    renderSource(<Source />)
    await guideLoaded()
    expect(screen.getByLabelText('형식 안내 보기')).toHaveValue(FAKE_FORMAT_GUIDE.ko)
  })

  // The clipboard write must be the first thing the click awaits, so the guide is read before any
  // click — and until it has arrived there is nothing true to copy.
  it('keeps the guide copy disabled until the guide has been read', async () => {
    let serve: () => void = () => {}
    const served = new Promise<void>((resolve) => {
      serve = resolve
    })
    const transport = createRouterTransport(({ rpc }) => {
      rpc(TemplateService.method.getFormatGuide, async () => {
        await served
        return create(GetFormatGuideResponseSchema, { text: FAKE_FORMAT_GUIDE.ko })
      })
    })
    render(<Source />, { wrapper: withProviders(transport, createTestQueryClient()) })

    expect(screen.getByRole('button', { name: '형식 안내 복사' })).toBeDisabled()
    await act(async () => serve())
    await guideLoaded()
  })

  // A guide that could not be read is said so with a retry, never copied empty.
  it('says the guide failed to load and reads it again on retry', async () => {
    const user = userEvent.setup()
    const options: FakeTemplatesOptions = { guideFails: true, calls: [] }
    renderSource(<Source />, options)

    expect(await screen.findByText('형식 안내를 불러오지 못했어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '형식 안내 복사' })).toBeDisabled()

    options.guideFails = false
    await user.click(screen.getByRole('button', { name: '다시 시도' }))
    await guideLoaded()
    expect(screen.queryByText('형식 안내를 불러오지 못했어요.')).not.toBeInTheDocument()
    expect(options.calls?.filter((call) => call === 'GetFormatGuide')).toHaveLength(2)
  })

  // The guide follows the reader's language (TMPL-41): switching the UI reads that language's guide.
  it('reads the guide again in the language the UI switches to', async () => {
    renderSource(<Source />)
    await guideLoaded()
    expect(screen.getByLabelText('형식 안내 보기')).toHaveValue(FAKE_FORMAT_GUIDE.ko)

    await act(async () => {
      await i18next.changeLanguage('en')
    })
    await waitFor(() =>
      expect(screen.getByLabelText('Show format guide')).toHaveValue(FAKE_FORMAT_GUIDE.en),
    )
  })

  // The same counter the name and description fields carry, over the body's own ceiling.
  it('counts what is left against the body ceiling', async () => {
    const user = userEvent.setup()
    renderSource(<Source />)

    expect(screen.getByText(`${TEMPLATE_LIMITS.body}자 남음`)).toBeInTheDocument()
    await user.click(screen.getByLabelText('원문'))
    await user.paste('가나다')
    expect(screen.getByText(`${TEMPLATE_LIMITS.body - 3}자 남음`)).toBeInTheDocument()
  })
})

// TMPL-50: the title area in source is its own short field with its own failure and counter.
// Copying, the format guide and the guide's copy belong to the body alone (TMPL-26, TMPL-42).
describe('the title area source', () => {
  it('is labelled 제목 원문 and counts against the title ceiling', async () => {
    const user = userEvent.setup()
    function TitleSource() {
      const [title, setTitle] = useState('')
      return <TemplateSource area="title_area" value={title} onChange={setTitle} failure={null} />
    }
    render(<TitleSource />)

    const field = screen.getByLabelText('제목 원문')
    expect(field).toHaveAttribute('id', 'template-title-source')
    expect(screen.getByText(`${TEMPLATE_LIMITS.titleArea}자 남음`)).toBeInTheDocument()
    await user.type(field, '방문 후기')
    expect(screen.getByText(`${TEMPLATE_LIMITS.titleArea - 5}자 남음`)).toBeInTheDocument()
  })

  it('shows its own failure at its line and marks itself invalid', () => {
    renderSource(
      <TemplateSource
        area="title_area"
        value={'<slot kind="photo"/>'}
        onChange={() => {}}
        failure={{ line: 1, reason: 'not_in_title', area: 'title_area' }}
      />,
    )
    const field = screen.getByLabelText('제목 원문')
    const message = screen.getByRole('alert')
    expect(message).toHaveTextContent('1번째 줄: 사진·사진마다 반복은 제목에 넣을 수 없어요')
    expect(message).toHaveAttribute('id', 'template-title-source-error')
    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(field).toHaveAttribute('aria-describedby', message.id)
  })

  it('offers no copy and no guide', () => {
    renderSource(<TemplateSource area="title_area" value="" onChange={() => {}} failure={null} />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByText('형식 안내 보기')).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })
})
