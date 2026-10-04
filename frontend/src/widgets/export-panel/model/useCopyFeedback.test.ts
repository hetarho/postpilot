import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyImage } from '@/shared/lib'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import { useCopyFeedback } from './useCopyFeedback'

vi.mock('@/shared/lib', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/shared/lib')>()),
  copyImage: vi.fn(),
}))

const CONTENT = POST_CONTENT_FIXTURE
const originalClipboard = navigator.clipboard

function setClipboard(value: Pick<Clipboard, 'writeText'> | undefined) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

/** A clipboard whose writes settle only when the test says so, one per call. */
function deferredClipboard() {
  const settle: Array<{ resolve: () => void; reject: (reason: unknown) => void }> = []
  const writeText = vi.fn(
    () => new Promise<void>((resolve, reject) => settle.push({ resolve, reject })),
  )
  setClipboard({ writeText })
  return { writeText, settle }
}

/** A read-only field in the document holding `value`, as a mounted fallback field is. */
function field(value: string) {
  const input = document.createElement('input')
  input.readOnly = true
  input.value = value
  document.body.append(input)
  return input
}

function setup() {
  return renderHook(() => useCopyFeedback(CONTENT))
}

afterEach(() => {
  setClipboard(originalClipboard)
  vi.useRealTimers()
  vi.mocked(copyImage).mockReset()
  vi.restoreAllMocks()
  document.body.replaceChildren()
})

describe('a text copy', () => {
  it('confirms the value that reached the clipboard for COPY_FEEDBACK_MS', async () => {
    vi.useFakeTimers()
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    const { result } = setup()

    await act(() => result.current.copy('title', CONTENT.title, null))
    expect(writeText).toHaveBeenCalledWith(CONTENT.title)
    expect(result.current.copied).toEqual({ target: 'title', value: CONTENT.title })
    expect(result.current.manualCopy).toBeUndefined()

    act(() => vi.advanceTimersByTime(COPY_FEEDBACK_MS - 1))
    expect(result.current.copied).toBeDefined()
    act(() => vi.advanceTimersByTime(1))
    expect(result.current.copied).toBeUndefined()
  })

  it('leaves a refused copy selected in its field, recorded against the content it came from', async () => {
    setClipboard(undefined)
    const { result } = setup()
    const tags = field('#제주 #산책 #여행')
    const select = vi.spyOn(tags, 'select')
    result.current.tagsRef.current = tags

    await act(() => result.current.copy('tags', '#제주 #산책 #여행', tags))

    expect(select).toHaveBeenCalledOnce()
    expect(tags).toHaveFocus()
    expect(result.current.copied).toBeUndefined()
    expect(result.current.manualCopy).toEqual({
      target: 'tags',
      value: '#제주 #산책 #여행',
      source: CONTENT,
    })
  })

  // A retry from the revealed field must not unmount it for the whole in-flight wait.
  it('keeps the fallback while a retry is in flight, and drops it once the retry lands', async () => {
    setClipboard(undefined)
    const { result } = setup()
    await act(() => result.current.copy('output', 'body', null))
    expect(result.current.manualCopy?.target).toBe('output')

    const clipboard = deferredClipboard()
    let retry: Promise<void> = Promise.resolve()
    await act(async () => {
      retry = result.current.copy('output', 'body', null)
    })
    expect(result.current.manualCopy?.target).toBe('output')

    clipboard.settle[0].resolve()
    await act(() => retry)
    expect(result.current.manualCopy).toBeUndefined()
    expect(result.current.copied).toEqual({ target: 'output', value: 'body' })
  })

  // The tags field mounts only while there are tags: a result settling after it is gone must be
  // compared against the field as it stands NOW, not the one the copy started from.
  it('drops a result whose field unmounted before it settled', async () => {
    const clipboard = deferredClipboard()
    const { result } = setup()
    const tags = field('#제주 #산책 #여행')
    result.current.tagsRef.current = tags

    let copy: Promise<void> = Promise.resolve()
    await act(async () => {
      copy = result.current.copy('tags', '#제주 #산책 #여행', tags)
    })
    result.current.tagsRef.current = null
    clipboard.settle[0].resolve()
    await act(() => copy)

    expect(result.current.copied).toBeUndefined()
  })

  // A third text target must not compare against another target's field.
  it('drops a result whose field is a different copy target', async () => {
    setClipboard(undefined)
    const { result } = setup()
    const title = field(CONTENT.title)
    const select = vi.spyOn(title, 'select')
    result.current.titleRef.current = title

    // A tags copy handed the TITLE field: it is not the tags field, so its outcome is stale.
    await act(() => result.current.copy('tags', CONTENT.title, title))

    expect(select).not.toHaveBeenCalled()
    expect(result.current.manualCopy).toBeUndefined()
  })

  it('drops a result overtaken by an invalidation', async () => {
    const clipboard = deferredClipboard()
    const { result } = setup()

    let copy: Promise<void> = Promise.resolve()
    await act(async () => {
      copy = result.current.copy('output', 'body', null)
    })
    act(() => result.current.invalidate())
    clipboard.settle[0].reject(new DOMException('blocked', 'NotAllowedError'))
    await act(() => copy)

    expect(result.current.manualCopy).toBeUndefined()
    expect(result.current.copied).toBeUndefined()
  })

  it('queues two presses, so only the later one reports', async () => {
    const clipboard = deferredClipboard()
    const { result } = setup()

    let first: Promise<void> = Promise.resolve()
    let second: Promise<void> = Promise.resolve()
    await act(async () => {
      first = result.current.copy('title', CONTENT.title, null)
    })
    await act(async () => {
      second = result.current.copy('tags', '#제주 #산책 #여행', null)
    })
    // The second write waits for the first to settle rather than racing it for the clipboard.
    expect(clipboard.writeText).toHaveBeenCalledOnce()

    clipboard.settle[0].resolve()
    await act(() => first)
    expect(clipboard.writeText).toHaveBeenCalledTimes(2)
    expect(clipboard.writeText).toHaveBeenLastCalledWith('#제주 #산책 #여행')
    expect(result.current.copied).toBeUndefined()

    clipboard.settle[1].resolve()
    await act(() => second)
    expect(result.current.copied).toEqual({ target: 'tags', value: '#제주 #산책 #여행' })
  })

  it('compares a caption copy against that caption’s registered field', async () => {
    setClipboard(undefined)
    const { result } = setup()
    const caption = field('비 뒤의 바다')
    act(() => result.current.registerCaptionField(0, caption))
    expect(result.current.captionField(0)).toBe(caption)

    await act(() => result.current.copy('caption:0', '비 뒤의 바다', caption))
    expect(result.current.manualCopy).toMatchObject({ target: 'caption:0', value: '비 뒤의 바다' })

    act(() => result.current.registerCaptionField(0, null))
    expect(result.current.captionField(0)).toBeNull()
  })
})

describe('a photo copy', () => {
  const image = () => document.createElement('img')

  it('confirms the photo it copied, turned as shown, for COPY_FEEDBACK_MS', async () => {
    vi.useFakeTimers()
    vi.mocked(copyImage).mockResolvedValue({ kind: 'copied' })
    const { result } = setup()
    const photo = image()

    await act(() => result.current.copyPhoto('photo:0:IMG_1.jpg', photo, 90))
    expect(copyImage).toHaveBeenCalledWith(photo, 90)
    expect(result.current.copied).toEqual({ target: 'photo:0:IMG_1.jpg' })

    act(() => vi.advanceTimersByTime(COPY_FEEDBACK_MS))
    expect(result.current.copied).toBeUndefined()
  })

  it('names how it failed on the photo that failed, and a throw is unreadable', async () => {
    vi.mocked(copyImage).mockResolvedValueOnce({ kind: 'refused' })
    const { result } = setup()

    await act(() => result.current.copyPhoto('photo:1:IMG_2.jpg', image(), 0))
    expect(result.current.photoFailure).toEqual({ target: 'photo:1:IMG_2.jpg', kind: 'refused' })

    vi.mocked(copyImage).mockRejectedValueOnce(new Error('encode failed'))
    await act(() => result.current.copyPhoto('photo:0:IMG_1.jpg', image(), 0))
    expect(result.current.photoFailure).toEqual({ target: 'photo:0:IMG_1.jpg', kind: 'unreadable' })
  })

  it('clears every other feedback when pressed', async () => {
    setClipboard(undefined)
    vi.mocked(copyImage).mockResolvedValue({ kind: 'blocked' })
    const { result } = setup()
    await act(() => result.current.copy('output', 'body', null))
    expect(result.current.manualCopy).toBeDefined()

    await act(() => result.current.copyPhoto('photo:0:IMG_1.jpg', image(), 0))
    expect(result.current.manualCopy).toBeUndefined()
    expect(result.current.photoFailure?.kind).toBe('blocked')

    // And a text copy clears the photo's failure in turn.
    await act(() => result.current.copy('output', 'body', null))
    expect(result.current.photoFailure).toBeUndefined()
  })
})

describe('the end of the panel', () => {
  it('drops the pending confirmation timer on unmount', async () => {
    vi.useFakeTimers()
    setClipboard({ writeText: vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined) })
    const { result, unmount } = setup()
    await act(() => result.current.copy('title', CONTENT.title, null))
    expect(vi.getTimerCount()).toBe(1)

    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('lands nothing from a copy that settles after it unmounted', async () => {
    vi.useFakeTimers()
    const clipboard = deferredClipboard()
    const { result, unmount } = setup()
    let copy: Promise<void> = Promise.resolve()
    await act(async () => {
      copy = result.current.copy('title', CONTENT.title, null)
    })

    unmount()
    clipboard.settle[0].resolve()
    await copy
    // A copy that landed would have started its confirmation's dwell.
    expect(vi.getTimerCount()).toBe(0)
  })
})
