import { afterEach, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toNaver } from '@/features/export-naver'
import { BlockType } from '@/shared/api'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { ExportPanel } from './ExportPanel'

const originalClipboard = navigator.clipboard

function setClipboard(value: Pick<Clipboard, 'writeText'> | Clipboard | undefined) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

afterEach(() => {
  setClipboard(originalClipboard)
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderPanel() {
  render(
    <ExportPanel
      content={POST_CONTENT_FIXTURE}
      images={POST_IMAGES_FIXTURE}
      createdAt="2026-08-29T03:04:05Z"
      contentLanguage="ko"
    />,
  )
}

it('switches four synchronous outputs with their guidance and keeps the Naver title separate', async () => {
  const user = userEvent.setup()
  const fetchSpy = vi.spyOn(globalThis, 'fetch')
  renderPanel()

  expect(screen.getByLabelText('네이버 제목')).toHaveValue(POST_CONTENT_FIXTURE.title)
  expect(
    screen.getByText('본문을 붙여넣은 뒤, 미리보기의 사진을 복사해 [사진 …] 자리에 붙여넣으세요'),
  ).toBeInTheDocument()

  await user.click(screen.getByRole('tab', { name: '티스토리' }))
  expect(
    screen.getByText('HTML 모드에 붙여넣고 사진 업로드 후 src를 교체하세요'),
  ).toBeInTheDocument()
  expect(screen.getByLabelText<HTMLTextAreaElement>('내보내기 결과').value).toContain(
    '<p class="summary">',
  )

  await user.click(screen.getByRole('tab', { name: '자체 사이트' }))
  expect(screen.getByText('그대로 .html로 저장하고 사진 파일을 옆에 두세요')).toBeInTheDocument()
  await user.click(screen.getByRole('tab', { name: '마크다운' }))
  expect(
    screen.getByText('Hugo · Jekyll · Obsidian에 맞는 형식이에요. 사진 파일을 같은 폴더에 두세요'),
  ).toBeInTheDocument()
  expect(fetchSpy).not.toHaveBeenCalled()
})

// ── The rendered Naver preview (change 18) ────────────────────────────────────────────────────
//
// The Naver tab shows the POST, not the wire text: the `[사진 …]` markers exist only in what the
// copy button puts on the clipboard, and each photo renders inline at its marker position with
// its own copy control.

it('renders the Naver tab as the post — no marker text, no raw field, photos inline in marker order', async () => {
  const user = userEvent.setup()
  renderPanel()

  const preview = screen.getByRole('article', { name: '네이버 미리보기' })
  // The markers are the copied text's business, not the preview's.
  expect(preview.textContent).not.toContain('[사진')
  expect(screen.queryByLabelText('내보내기 결과')).not.toBeInTheDocument()
  // The body renders as the reading view does, header excluded: the body copy does not paste the
  // title/summary/tags, and the title has its own field above.
  expect(within(preview).getByText('비가 그치기를 기다렸다.')).toBeInTheDocument()
  expect(within(preview).getByRole('heading', { name: '바닷가로' })).toBeInTheDocument()
  expect(within(preview).queryByText(POST_CONTENT_FIXTURE.summary)).not.toBeInTheDocument()

  // One copy control per `[사진 …]` marker, in marker order — asserted through the controls'
  // accessible names, which bind each control to the marker it copies for.
  const markers = [
    ...toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko').matchAll(/\[사진 ([^\]:]+)/g),
  ].map((match) => match[1])
  expect(markers.length).toBeGreaterThan(1)
  expect(
    within(preview)
      .getAllByRole('button')
      .map((button) => button.getAttribute('aria-label')),
  ).toEqual(markers.map((file) => `${file} 사진 복사`))
  expect(within(preview).getAllByRole('img')).toHaveLength(markers.length)

  // The other three formats are source to be read, not a post to be seen.
  for (const format of ['티스토리', '자체 사이트', '마크다운']) {
    await user.click(screen.getByRole('tab', { name: format }))
    expect(screen.queryByRole('article', { name: '네이버 미리보기' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('내보내기 결과')).toBeInTheDocument()
  }
})

it('renders no photo controls, and no photo guidance, for a post with no photos', () => {
  render(
    <ExportPanel
      content={{
        ...POST_CONTENT_FIXTURE,
        blocks: POST_CONTENT_FIXTURE.blocks.filter((block) => block.type !== BlockType.IMAGE),
      }}
      images={[]}
      createdAt="2026-08-29T03:04:05Z"
      contentLanguage="ko"
    />,
  )
  expect(screen.queryByRole('button', { name: /사진 복사$/ })).not.toBeInTheDocument()
  expect(screen.getByText('본문을 그대로 붙여넣으세요')).toBeInTheDocument()
})

it('copies the exact rendered output and shows transient success', async () => {
  const user = userEvent.setup()
  const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
  setClipboard({ writeText })
  renderPanel()
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko')

  await user.click(screen.getByRole('button', { name: '복사' }))

  await waitFor(() => expect(writeText).toHaveBeenCalledWith(output))
  // The confirmation is the docked status line, not a label swap on the button: the button keeps
  // its size and its name under the thumb that pressed it.
  expect(await screen.findByText('복사됨')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '복사' })).toBeInTheDocument()
  // A successful copy leaves the preview in place.
  expect(screen.queryByLabelText('내보내기 결과')).not.toBeInTheDocument()
})

it('reveals, selects and explains the raw text when the Clipboard API is unavailable', async () => {
  const user = userEvent.setup()
  setClipboard(undefined)
  const select = vi.spyOn(HTMLTextAreaElement.prototype, 'select')
  renderPanel()

  await user.click(screen.getByRole('button', { name: '복사' }))

  // The raw marker text mounts only for the fallback: a selection must be SEEN to be long-pressed.
  await waitFor(() => expect(select).toHaveBeenCalledOnce())
  const raw = screen.getByLabelText<HTMLTextAreaElement>('내보내기 결과')
  expect(raw).toHaveFocus()
  expect(raw.value).toBe(toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko'))
  expect(
    screen.getByText('자동 복사가 막혀 있어요. 선택된 텍스트를 길게 눌러 복사하세요'),
  ).toBeInTheDocument()
  expect(screen.queryByRole('article', { name: '네이버 미리보기' })).not.toBeInTheDocument()

  // Leaving and returning dismisses the fallback back to the preview.
  await user.click(screen.getByRole('tab', { name: '티스토리' }))
  await user.click(screen.getByRole('tab', { name: '네이버 블로그' }))
  expect(screen.getByRole('article', { name: '네이버 미리보기' })).toBeInTheDocument()
  expect(screen.queryByLabelText('내보내기 결과')).not.toBeInTheDocument()

  // A copy that succeeds afterwards also returns to the preview.
  const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
  await user.click(screen.getByRole('button', { name: '복사' }))
  await waitFor(() => screen.getByLabelText('내보내기 결과'))
  setClipboard({ writeText })
  await user.click(screen.getByRole('button', { name: '복사' }))
  await waitFor(() => expect(writeText).toHaveBeenCalledOnce())
  expect(screen.queryByLabelText('내보내기 결과')).not.toBeInTheDocument()
})

it('dismisses the revealed fallback when the content changes under it', async () => {
  const user = userEvent.setup()
  setClipboard(undefined)
  const view = render(
    <ExportPanel
      content={POST_CONTENT_FIXTURE}
      images={POST_IMAGES_FIXTURE}
      createdAt="2026-08-29T03:04:05Z"
      contentLanguage="ko"
    />,
  )
  await user.click(screen.getByRole('button', { name: '복사' }))
  await waitFor(() => screen.getByLabelText('내보내기 결과'))

  // The selection described the previous text; keeping it selected would hand SmartEditor a body
  // the post no longer contains.
  view.rerender(
    <ExportPanel
      content={{
        ...POST_CONTENT_FIXTURE,
        blocks: POST_CONTENT_FIXTURE.blocks.filter((block) => block.type !== BlockType.QUOTE),
      }}
      images={POST_IMAGES_FIXTURE}
      createdAt="2026-08-29T03:04:05Z"
      contentLanguage="ko"
    />,
  )
  expect(screen.queryByLabelText('내보내기 결과')).not.toBeInTheDocument()
  expect(screen.getByRole('article', { name: '네이버 미리보기' })).toBeInTheDocument()
})

it('ignores a stale clipboard rejection after the format changes', async () => {
  const user = userEvent.setup()
  let rejectWrite: (reason?: unknown) => void = () => undefined
  const writeText = vi.fn(
    () =>
      new Promise<void>((_resolve, reject) => {
        rejectWrite = reject
      }),
  )
  setClipboard({
    writeText,
  })
  const select = vi.spyOn(HTMLTextAreaElement.prototype, 'select')
  renderPanel()

  await user.click(screen.getByRole('button', { name: '복사' }))
  await waitFor(() => expect(writeText).toHaveBeenCalledOnce())
  await user.click(screen.getByRole('tab', { name: '티스토리' }))
  rejectWrite(new DOMException('blocked', 'NotAllowedError'))
  await new Promise((resolve) => window.setTimeout(resolve, 0))

  await waitFor(() =>
    expect(screen.getByRole('tab', { name: '티스토리' })).toHaveAttribute('aria-selected', 'true'),
  )
  expect(select).not.toHaveBeenCalled()
  expect(
    screen.queryByText('자동 복사가 막혀 있어요. 선택된 텍스트를 길게 눌러 복사하세요'),
  ).not.toBeInTheDocument()
  expect(screen.queryByText('복사됨')).not.toBeInTheDocument()
})

// ── The photo copy (change 17, presentation moved inline by change 18) ─────────────────────────
//
// The clipboard PAYLOAD SHAPE is the part that must never regress: SmartEditor ONE accepts
// exactly one `ClipboardItem` carrying `image/png` and nothing else. jsdom has neither
// `ClipboardItem` nor `createImageBitmap`, so both are stubbed and the assertion stays on the
// shape rather than on real pixels.

interface StubbedClipboard {
  write: ReturnType<typeof vi.fn>
  items: Array<Record<string, Promise<Blob>>>
}

function stubImageClipboard({ write }: { write?: () => Promise<void> } = {}): StubbedClipboard {
  const items: Array<Record<string, Promise<Blob>>> = []
  class FakeClipboardItem {
    constructor(readonly types: Record<string, Promise<Blob>>) {
      items.push(types)
    }
  }
  // Awaits the item's promises the way the real API does — see `copyImage`'s own test.
  const awaitItems = async (given: unknown[]) => {
    for (const item of given as FakeClipboardItem[]) await Promise.all(Object.values(item.types))
  }
  vi.stubGlobal('ClipboardItem', FakeClipboardItem)
  vi.stubGlobal(
    'createImageBitmap',
    vi.fn().mockResolvedValue({ width: 4, height: 3, close: vi.fn() }),
  )
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response(new Blob([new Uint8Array([1, 2, 3])], { type: 'image/jpeg' })),
  )
  // The canvas is the other thing jsdom does not have. Only the encode's OUTPUT matters here.
  vi.stubGlobal(
    'OffscreenCanvas',
    class {
      constructor(
        readonly width: number,
        readonly height: number,
      ) {}
      getContext() {
        return { drawImage: vi.fn() }
      }
      convertToBlob({ type }: { type: string }) {
        return Promise.resolve(new Blob([new Uint8Array([9])], { type }))
      }
    },
  )
  const spy = vi.fn(async (given: unknown[]) => {
    await awaitItems(given)
    return (write ?? (() => Promise.resolve()))()
  })
  setClipboard({ writeText: vi.fn(), write: spy } as unknown as Clipboard)
  return { write: spy, items }
}

it('writes exactly one ClipboardItem carrying image/png and nothing else', async () => {
  const user = userEvent.setup()
  const clipboard = stubImageClipboard()
  renderPanel()

  await user.click(screen.getAllByRole('button', { name: /사진 복사$/ })[0])

  await waitFor(() => expect(clipboard.write).toHaveBeenCalledOnce())
  // ONE item, ONE flavor. A `text/html` flavor beside it makes the editor render a broken image,
  // and a second item is silently ignored.
  const written = clipboard.write.mock.calls[0][0] as unknown[]
  expect(written).toHaveLength(1)
  expect(clipboard.items).toHaveLength(1)
  expect(Object.keys(clipboard.items[0])).toEqual(['image/png'])
  // The value is a PROMISE of the blob: `write` is called before the bytes arrive so the user
  // activation is not spent on the fetch (see `copyImage`).
  expect(await clipboard.items[0]['image/png']).toBeInstanceOf(Blob)
  expect((await clipboard.items[0]['image/png']).type).toBe('image/png')
  expect(await screen.findByText(/사진이 복사됐어요/)).toBeInTheDocument()
})

it('names the failure on the photo that failed, with no text flavor substituted', async () => {
  const user = userEvent.setup()
  const clipboard = stubImageClipboard({
    write: () => Promise.reject(new DOMException('blocked', 'NotAllowedError')),
  })
  renderPanel()

  await user.click(screen.getAllByRole('button', { name: /사진 복사$/ })[0])

  expect(await screen.findByText('사진 복사가 막혔어요. 다시 시도해 주세요.')).toBeInTheDocument()
  expect(clipboard.items).toHaveLength(1)
  expect(Object.keys(clipboard.items[0])).toEqual(['image/png'])
})

it('says so and offers no copy when the photo cannot be read', async () => {
  const user = userEvent.setup()
  stubImageClipboard()
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 403 }))
  renderPanel()

  await user.click(screen.getAllByRole('button', { name: /사진 복사$/ })[0])

  expect(
    await screen.findByText('사진을 읽지 못했어요. 글을 다시 불러오면 사진 주소가 새로 발급돼요.'),
  ).toBeInTheDocument()
})

// The bucket-CORS case. An `<img>` needs no CORS allow, so the photo renders while its bytes are
// unreachable from this origin — and the reload that remints an expired URL mints one blocked in
// exactly the same way. Telling the user to reload here is a loop with no exit, so the assertion
// that matters is the NEGATIVE one.
it('does not tell the user to reload when the photo bytes never arrive', async () => {
  const user = userEvent.setup()
  stubImageClipboard()
  vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))
  renderPanel()

  await user.click(screen.getAllByRole('button', { name: /사진 복사$/ })[0])

  expect(
    await screen.findByText(
      '지금은 사진을 복사할 수 없어요. 본문만 붙여넣고 사진은 직접 올려 주세요.',
    ),
  ).toBeInTheDocument()
  expect(
    screen.queryByText('사진을 읽지 못했어요. 글을 다시 불러오면 사진 주소가 새로 발급돼요.'),
  ).not.toBeInTheDocument()
})

it('says so when the browser has no image clipboard at all', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('ClipboardItem', undefined)
  setClipboard({ writeText: vi.fn() } as unknown as Clipboard)
  renderPanel()

  await user.click(screen.getAllByRole('button', { name: /사진 복사$/ })[0])

  expect(
    await screen.findByText(
      '이 브라우저는 사진 복사를 지원하지 않아요. 본문만 붙여넣고 사진은 직접 올려 주세요.',
    ),
  ).toBeInTheDocument()
})

it('refuses to copy a photo that is still a local upload preview, but keeps its pixels inline', () => {
  render(
    <ExportPanel
      content={POST_CONTENT_FIXTURE}
      images={POST_IMAGES_FIXTURE.map((image) => ({ ...image, viewUrl: 'blob:local' }))}
      createdAt="2026-08-29T03:04:05Z"
      contentLanguage="ko"
    />,
  )
  const buttons = screen.getAllByRole('button', { name: /사진 복사$/ })
  expect(buttons.length).toBeGreaterThan(0)
  for (const button of buttons) {
    expect(button).toBeDisabled()
  }
  // The preview still shows the photo the reader will see; only the copy is refused.
  const preview = screen.getByRole('article', { name: '네이버 미리보기' })
  expect(within(preview).getAllByRole('img').length).toBeGreaterThan(0)
  expect(
    screen.getAllByText('사진을 읽지 못했어요. 글을 다시 불러오면 사진 주소가 새로 발급돼요.')
      .length,
  ).toBeGreaterThan(0)
})

it('holds the marker position of a photo missing from the post', () => {
  render(
    <ExportPanel
      content={POST_CONTENT_FIXTURE}
      images={[]}
      createdAt="2026-08-29T03:04:05Z"
      contentLanguage="ko"
    />,
  )
  // The filename says which file belongs at the marker the copied text still carries; a dropped
  // entry would shift every later photo against its marker.
  const preview = screen.getByRole('article', { name: '네이버 미리보기' })
  expect(within(preview).getByText('IMG_1.jpg')).toBeInTheDocument()
  expect(screen.getAllByText('이 표시에 해당하는 사진을 찾지 못했어요.').length).toBeGreaterThan(0)
})
