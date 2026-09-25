import { describe, expect, it } from 'vitest'
import { focusablesIn } from './focusable'

describe('focusablesIn', () => {
  it('lists what Tab reaches, in document order', () => {
    const root = document.createElement('div')
    root.innerHTML = `
      <a href="#a">링크</a>
      <a>주소 없는 링크</a>
      <button>버튼</button>
      <button disabled>꺼진 버튼</button>
      <input aria-label="입력" />
      <input aria-label="꺼진 입력" disabled />
      <select aria-label="선택"><option>하나</option></select>
      <textarea aria-label="글"></textarea>
      <details><summary>요약</summary></details>
      <div tabindex="0">탭 가능</div>
      <div role="option" tabindex="-1">옵션</div>
      <span>글자</span>
    `
    expect(
      focusablesIn(root).map((node) => node.getAttribute('aria-label') ?? node.textContent?.trim()),
    ).toEqual(['링크', '버튼', '입력', '선택', '글', '요약', '탭 가능'])
    expect(focusablesIn(null)).toEqual([])
    expect(focusablesIn(undefined)).toEqual([])
  })
})
