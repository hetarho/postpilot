import type { ClipboardEvent } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyOriginSelection } from './copy-origin-selection'

afterEach(() => {
  document.body.replaceChildren()
  window.getSelection()?.removeAllRanges()
})

function copy(root: HTMLDivElement, range: Range, target: Element = root) {
  const selection = window.getSelection()!
  selection.removeAllRanges()
  selection.addRange(range)
  const setData = vi.fn()
  const preventDefault = vi.fn()
  copyOriginSelection({
    currentTarget: root,
    target,
    clipboardData: { setData },
    preventDefault,
    defaultPrevented: false,
  } as unknown as ClipboardEvent<HTMLDivElement>)
  return { setData, preventDefault }
}

describe('copyOriginSelection', () => {
  it('excludes DOM-marked review labels from both clipboard flavors while retaining identical owner words and canonical media text', () => {
    const root = document.createElement('div')
    root.innerHTML =
      '<p><span data-writing-origin-phrase>소유자 원문: 대체 텍스트 출처.\n  공백도 보존😊</span></p><div><p data-writing-origin-review-ui>대체 텍스트 출처</p><p><span data-writing-origin-phrase>사진의 원문 대체 텍스트</span></p></div><figcaption><span data-writing-origin-phrase>사진의 원문 설명</span></figcaption>'
    document.body.append(root)
    const range = document.createRange()
    range.selectNodeContents(root)
    const before = range.toString()
    const result = copy(root, range)
    const text = result.setData.mock.calls.find(([type]) => type === 'text/plain')![1]
    const html = result.setData.mock.calls.find(([type]) => type === 'text/html')![1]
    expect(text.match(/대체 텍스트 출처/g)).toHaveLength(1)
    expect(text).toContain('소유자 원문: 대체 텍스트 출처.\n  공백도 보존😊')
    expect(text).toContain('사진의 원문 대체 텍스트')
    expect(text).toContain('사진의 원문 설명')
    expect(text).toContain('\n\n')
    expect(html.match(/대체 텍스트 출처/g)).toHaveLength(1)
    expect(html).not.toContain('data-writing-origin-review-ui')
    expect(root.querySelector('[data-writing-origin-review-ui]')).not.toBeNull()
    expect(window.getSelection()!.toString()).toBe(before)
    expect(document.body.children).toHaveLength(1)
  })

  it('preserves partial mixed-origin words and literal owner markup while removing annotation styling', () => {
    const root = document.createElement('div')
    root.innerHTML =
      '<p><span data-writing-origin-phrase role="button" aria-label="PRIVATE" class="text-origin-owner-foreground" style="color:red">기록😊</span> · <span data-writing-origin-phrase style="background:gold">사진</span> · <span data-writing-origin-phrase>향</span></p>'
    const literal = '<span class="text-origin-owner-foreground">출처 보기</span>'
    root.querySelector('p')!.append(document.createTextNode(literal))
    document.body.append(root)
    const range = document.createRange()
    range.setStart(root.querySelector('span')!.firstChild!, 2)
    range.setEnd(root.querySelector('p')!.lastChild!, literal.length)
    const text = range.toString()
    const result = copy(root, range)
    expect(result.preventDefault).toHaveBeenCalledOnce()
    expect(result.setData).toHaveBeenCalledWith('text/plain', text)
    const html = result.setData.mock.calls.find(([type]) => type === 'text/html')![1]
    const pasted = document.createElement('div')
    pasted.innerHTML = html
    expect(pasted.textContent).toBe(text)
    expect(
      pasted.querySelector('[style], [class], [role], [aria-label], [data-writing-origin-phrase]'),
    ).toBeNull()
    expect(html).toContain(
      '&lt;span class="text-origin-owner-foreground"&gt;출처 보기&lt;/span&gt;',
    )
  })

  it('keeps selected heading, paragraph, quote and list structure without copying media URLs or interaction attributes', () => {
    const root = document.createElement('div')
    root.innerHTML =
      '<h2 class="theme-heading"><span data-writing-origin-phrase role="button">제목</span></h2><p><span data-writing-origin-phrase style="color:red">본문</span></p><blockquote>인용</blockquote><ul><li><span data-writing-origin-phrase>목록</span></li></ul><img src="https://private.invalid/signed?secret=1" alt="사진">'
    document.body.append(root)
    const range = document.createRange()
    range.selectNodeContents(root)
    const result = copy(root, range)
    expect(result.setData).toHaveBeenCalledWith(
      'text/html',
      '<h2>제목</h2><p>본문</p><blockquote>인용</blockquote><ul><li>목록</li></ul>',
    )
    expect(result.setData).toHaveBeenCalledWith('text/plain', window.getSelection()!.toString())
  })

  it('leaves editable fields, unrelated selections and ranges outside the review browser-owned', () => {
    const root = document.createElement('div')
    root.innerHTML =
      '<p><span data-writing-origin-phrase>본문</span></p><textarea>편집 중 원문😊</textarea><p>다른 글</p>'
    document.body.append(root)
    const range = document.createRange()
    range.selectNodeContents(root.querySelector('span')!)
    expect(copy(root, range, root.querySelector('textarea')!).preventDefault).not.toHaveBeenCalled()
    range.selectNodeContents(root.querySelector('p:last-child')!)
    expect(copy(root, range).preventDefault).not.toHaveBeenCalled()
    const outside = document.createElement('p')
    outside.textContent = 'outside'
    document.body.append(outside)
    range.setStart(root.querySelector('span')!.firstChild!, 0)
    range.setEnd(outside.firstChild!, 3)
    expect(copy(root, range).preventDefault).not.toHaveBeenCalled()
  })
})
