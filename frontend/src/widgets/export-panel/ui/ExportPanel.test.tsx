import { afterEach, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toNaver } from '@/features/export-naver'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { ExportPanel } from './ExportPanel'

const originalClipboard = navigator.clipboard

function setClipboard(value: Pick<Clipboard, 'writeText'> | undefined) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

afterEach(() => {
  setClipboard(originalClipboard)
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
  expect(screen.getByText('붙여넣고 표시된 자리에 사진을 드래그하세요')).toBeInTheDocument()

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
})

it('selects the preview and explains manual copy when the Clipboard API is unavailable', async () => {
  const user = userEvent.setup()
  setClipboard(undefined)
  const select = vi.spyOn(HTMLTextAreaElement.prototype, 'select')
  renderPanel()

  await user.click(screen.getByRole('button', { name: '복사' }))

  await waitFor(() => expect(select).toHaveBeenCalledOnce())
  expect(screen.getByLabelText('내보내기 결과')).toHaveFocus()
  expect(
    screen.getByText('자동 복사가 막혀 있어요. 선택된 텍스트를 길게 눌러 복사하세요'),
  ).toBeInTheDocument()
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
