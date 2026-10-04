import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import type { PostContent } from '@/shared/api'
import {
  POST_CONTENT_FIXTURE,
  POST_CONTENT_WITH_GROUPS_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import { useExportPanel } from './useExportPanel'

const CREATED_AT = '2026-08-29T03:04:05Z'
const NAVER = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko')
const TAGS = '#제주 #산책 #여행'
const originalClipboard = navigator.clipboard

function setClipboard(value: Pick<Clipboard, 'writeText'> | undefined) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

function setup(content: PostContent = POST_CONTENT_FIXTURE) {
  return renderHook(
    ({ content }: { content: PostContent }) =>
      useExportPanel({
        content,
        images: POST_IMAGES_FIXTURE,
        createdAt: CREATED_AT,
        contentLanguage: 'ko',
      }),
    { initialProps: { content } },
  )
}

/** The Naver tab's raw field and its two copy buttons, mounted and attached as the panel attaches
 *  them. The field holds other text than the body, so a body copy is handed no fallback element —
 *  exactly as on the Naver tab, where the field is not mounted until the copy falls back. */
function attachNaverControls(panel: ReturnType<typeof setup>['result']) {
  const field = document.createElement('textarea')
  field.readOnly = true
  const copyButton = document.createElement('button')
  const copyWithTagsButton = document.createElement('button')
  document.body.append(field, copyButton, copyWithTagsButton)
  panel.current.outputRef.current = field
  panel.current.copyButtonRef.current = copyButton
  panel.current.copyWithTagsButtonRef.current = copyWithTagsButton
  return { field, copyButton, copyWithTagsButton }
}

afterEach(() => {
  setClipboard(originalClipboard)
  vi.restoreAllMocks()
  document.body.replaceChildren()
})

describe('the four outputs', () => {
  it('opens on the Naver tab and switches between four synchronous derivations', () => {
    const { result } = setup()
    expect(result.current.format).toBe('naver')
    expect(result.current.output).toBe(NAVER)
    expect(result.current.rawFieldVisible).toBe(false)

    const expected = {
      tistory: toTistory(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko'),
      site: toSite(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, CREATED_AT, 'ko'),
      markdown: toMarkdown(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, CREATED_AT, 'ko'),
    }
    for (const format of ['tistory', 'site', 'markdown'] as const) {
      act(() => result.current.selectFormat(format))
      expect(result.current.output).toBe(expected[format])
      // Markup is read as source, so these tabs keep the raw field always.
      expect(result.current.rawFieldVisible).toBe(true)
      expect(result.current.fallbackOutput).toBe(expected[format])
    }
  })

  // EXPORT-20: the tags as one paste-ready string, and the Naver body with them at its end.
  it('offers the tags, and the Naver body with them, only when the post has tags', () => {
    const { result, rerender } = setup()
    expect(result.current.hashtags).toBe(TAGS)
    expect(result.current.outputWithTags).toBe(`${NAVER}\n\n${TAGS}`)

    rerender({ content: { ...POST_CONTENT_FIXTURE, tags: [] } })
    expect(result.current.hashtags).toBe('')
    expect(result.current.outputWithTags).toBe('')
  })

  // EXPORT-5, EXPORT-26: one number per photo, alone or in a group, counted in marker order.
  it('numbers every photo from 0 in marker order, a group holding one per photo', () => {
    const { result } = setup(POST_CONTENT_WITH_GROUPS_FIXTURE)
    expect([...result.current.photoNumbersByBlock]).toEqual([
      [1, [0]],
      [2, [1, 2]],
      [3, [3, 4]],
    ])
    expect(result.current.hasPhotos).toBe(true)
    expect(result.current.imagesByFilename.get('IMG_1.jpg')).toBe(POST_IMAGES_FIXTURE[0])
  })

  it('knows a post with no photos has none', () => {
    const { result } = setup({ ...POST_CONTENT_FIXTURE, blocks: [] })
    expect(result.current.hasPhotos).toBe(false)
  })
})

describe('the copies', () => {
  it('reports each copy on its own line', async () => {
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    const { result } = setup()

    await act(() => result.current.copyTitle())
    expect(writeText).toHaveBeenLastCalledWith(POST_CONTENT_FIXTURE.title)
    expect(result.current.status).toMatchObject({ title: 'copied', tags: undefined })

    await act(() => result.current.copyTags())
    expect(writeText).toHaveBeenLastCalledWith(TAGS)
    expect(result.current.status).toMatchObject({ title: undefined, tags: 'copied' })

    await act(() => result.current.copyOutputWithTags())
    expect(writeText).toHaveBeenLastCalledWith(`${NAVER}\n\n${TAGS}`)
    expect(result.current.status).toMatchObject({ outputWithTags: 'copied', output: undefined })

    await act(() => result.current.copyCaption(0, '비 뒤의 바다'))
    expect(writeText).toHaveBeenLastCalledWith('비 뒤의 바다')
    expect(result.current.captionCopy(0, '비 뒤의 바다').status).toBe('copied')
    expect(result.current.captionCopy(1, '비 뒤의 바다').status).toBeUndefined()
  })

  it('names how a photo copy failed on that photo alone', async () => {
    // No image clipboard at all: `copyImage` answers `unsupported` without touching the element.
    setClipboard({ writeText: vi.fn<Clipboard['writeText']>() })
    const { result } = setup()

    await act(() => result.current.copyPhoto('photo:0:IMG_1.jpg', document.createElement('img'), 0))
    expect(result.current.photoFailure('photo:0:IMG_1.jpg')).toBe('unsupported')
    expect(result.current.photoFailure('photo:1:IMG_2.jpg')).toBeUndefined()
    expect(result.current.photoCopied('photo:0:IMG_1.jpg')).toBe(false)
  })

  it('drops every confirmation, and a copy still in flight, when the format changes', async () => {
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    const { result } = setup()
    await act(() => result.current.copyTags())
    expect(result.current.status.tags).toBe('copied')

    act(() => result.current.selectFormat('tistory'))
    expect(result.current.status.tags).toBeUndefined()

    // A refusal that settles after a switch lands nowhere — not even once the user is back on the
    // tab whose text it was copying.
    let reject: (reason: unknown) => void = () => undefined
    writeText.mockImplementationOnce(() => new Promise<void>((_resolve, no) => (reject = no)))
    let copy: Promise<void> = Promise.resolve()
    await act(async () => {
      copy = result.current.copyOutput()
    })
    act(() => result.current.selectFormat('naver'))
    act(() => result.current.selectFormat('tistory'))
    reject(new DOMException('blocked', 'NotAllowedError'))
    await act(() => copy)

    expect(result.current.status.output).toBeUndefined()
  })
})

// EXPORT-9: the Naver tab shows the post, so a refused body copy has to REVEAL a field to select.
describe("the Naver body's fallback", () => {
  it('reveals the body field, focused and selected, once the copy is refused', async () => {
    setClipboard(undefined)
    const { result } = setup()
    const { field } = attachNaverControls(result)
    const select = vi.spyOn(field, 'select')

    await act(() => result.current.copyOutput())

    expect(result.current.rawFieldVisible).toBe(true)
    expect(result.current.fallbackOutput).toBe(NAVER)
    expect(result.current.status.output).toBe('manual')
    expect(field).toHaveFocus()
    expect(select).toHaveBeenCalledOnce()
  })

  it('reveals the combined body when that is the copy refused', async () => {
    setClipboard(undefined)
    const { result } = setup()
    const { field } = attachNaverControls(result)

    await act(() => result.current.copyOutputWithTags())

    expect(result.current.fallbackOutput).toBe(`${NAVER}\n\n${TAGS}`)
    expect(result.current.status.outputWithTags).toBe('manual')
    expect(field).toHaveFocus()
  })

  it('hands the focus back to the button pressed when a content change dissolves it', async () => {
    setClipboard(undefined)
    const { result, rerender } = setup()
    const { field, copyWithTagsButton } = attachNaverControls(result)
    await act(() => result.current.copyOutputWithTags())

    // The content changed under the selection: the field unmounts and the keyboard falls to <body>.
    field.remove()
    rerender({ content: { ...POST_CONTENT_FIXTURE, tags: ['다른태그'] } })

    expect(result.current.rawFieldVisible).toBe(false)
    expect(copyWithTagsButton).toHaveFocus()
  })

  it('returns the body copy its own button the same way', async () => {
    setClipboard(undefined)
    const { result, rerender } = setup()
    const { field, copyButton } = attachNaverControls(result)
    await act(() => result.current.copyOutput())

    field.remove()
    rerender({ content: { ...POST_CONTENT_FIXTURE, blocks: POST_CONTENT_FIXTURE.blocks.slice(1) } })

    expect(copyButton).toHaveFocus()
  })

  it('leaves the focus where the user put it when a tab switch dismisses it', async () => {
    setClipboard(undefined)
    const { result } = setup()
    const { copyButton } = attachNaverControls(result)
    await act(() => result.current.copyOutput())
    const tab = document.createElement('button')
    document.body.append(tab)
    tab.focus()

    act(() => result.current.selectFormat('tistory'))

    expect(tab).toHaveFocus()
    expect(copyButton).not.toHaveFocus()
  })
})
