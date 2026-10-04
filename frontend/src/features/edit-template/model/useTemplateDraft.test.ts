import { describe, expect, it } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import {
  POST_TAG_COUNT_DEFAULT,
  POST_TARGET_LENGTH_DEFAULT,
  POST_TARGET_LENGTH_MAX,
} from '@/entities/post'
import type { Template } from '@/entities/template'
import { useTemplateDraft } from './useTemplateDraft'

/** A stored template the way the directory maps one. */
function stored(fields: Partial<Template> = {}): Template {
  return {
    id: 'template-review',
    name: '정보성 식당 리뷰',
    description: '협찬 방문 리뷰',
    body: '<write>인트로를 씁니다</write>',
    titleArea: '',
    postCount: 0,
    createdAt: '2026-08-28T12:00:00Z',
    updatedAt: '2026-08-28T12:00:00Z',
    ...fields,
  }
}

/** The draft of `template`, or of a new template when there is none. */
function setup(template: Template | undefined) {
  return renderHook(() => useTemplateDraft(template))
}

describe('the draft as it opens', () => {
  it('seeds a stored template clean, its numbers ticked as stored', () => {
    const { result } = setup(stored({ targetLength: 1500, tagCount: 6 }))

    expect(result.current.draft).toMatchObject({
      name: '정보성 식당 리뷰',
      description: '협찬 방문 리뷰',
      body: '<write>인트로를 씁니다</write>',
      titleArea: '',
    })
    expect(result.current.lengthField).toEqual({ enabled: true, text: '1500' })
    expect(result.current.tagsField).toEqual({ enabled: true, text: '6' })
    expect(result.current.dirty).toBe(false)
    expect(result.current.saved).toBe(false)
  })

  it('opens a new template empty, with neither number ticked', () => {
    const { result } = setup(undefined)

    expect(result.current.draft).toMatchObject({ name: '', description: '', body: '' })
    expect(result.current.lengthField).toEqual({ enabled: false, text: '' })
    expect(result.current.dirty).toBe(false)
    // Nothing to save yet: a template needs a name and a body.
    expect(result.current.valid).toBe(false)
  })

  // TMPL-8: the body is the composition's serialization, so its outer bytes are content.
  it('reads a stored body with outer whitespace as clean and sends it untrimmed', () => {
    const { result } = setup(stored({ body: '\n인트로\n', titleArea: ' 제목 ' }))

    expect(result.current.dirty).toBe(false)
    act(() => result.current.setText('name')('여백!'))
    expect(result.current.trimmed.body).toBe('\n인트로\n')
    expect(result.current.trimmed.titleArea).toBe(' 제목 ')
  })
})

describe('an edit', () => {
  it('trims the prose, so outer spaces alone are no change', () => {
    const { result } = setup(stored())

    act(() => result.current.setText('name')('  정보성 식당 리뷰 '))
    expect(result.current.trimmed.name).toBe('정보성 식당 리뷰')
    expect(result.current.dirty).toBe(false)

    act(() => result.current.setText('description')('협찬 아님'))
    expect(result.current.dirty).toBe(true)
    expect(result.current.valid).toBe(true)
  })

  it('makes the draft dirty on a title area edit alone', () => {
    const { result } = setup(stored())

    act(() => result.current.setText('titleArea')('<write>메뉴를 한 줄로</write>'))
    expect(result.current.dirty).toBe(true)
    expect(result.current.trimmed.titleArea).toBe('<write>메뉴를 한 줄로</write>')
  })
})

// TMPL-49 and POST-20: a tick reveals a usable number, and what was typed survives an untick.
describe("the template's two numbers", () => {
  it('fills the default on ticking and keeps what was typed across an untick', () => {
    const { result } = setup(stored())

    act(() => result.current.setTargetLength({ enabled: true }))
    expect(result.current.lengthField).toEqual({
      enabled: true,
      text: String(POST_TARGET_LENGTH_DEFAULT),
    })
    act(() => result.current.setTargetLength({ text: '2000' }))
    act(() => result.current.setTargetLength({ enabled: false }))
    // Unticked is 의견 없음, whatever the field still holds.
    expect(result.current.trimmed.targetLength).toBeUndefined()
    expect(result.current.dirty).toBe(false)
    act(() => result.current.setTargetLength({ enabled: true }))
    expect(result.current.lengthField.text).toBe('2000')
    expect(result.current.trimmed.targetLength).toBe(2000)
    expect(result.current.dirty).toBe(true)

    act(() => result.current.setTagCount({ enabled: true }))
    expect(result.current.trimmed.tagCount).toBe(POST_TAG_COUNT_DEFAULT)
  })

  it('clears a stored number by unticking it', () => {
    const { result } = setup(stored({ tagCount: 6 }))

    act(() => result.current.setTagCount({ enabled: false }))
    expect(result.current.trimmed.tagCount).toBeUndefined()
    expect(result.current.dirty).toBe(true)
  })

  it('refuses a number outside its range, or one that is not a number, while still dirty', () => {
    const { result } = setup(stored())

    act(() => result.current.setTargetLength({ enabled: true }))
    act(() => result.current.setTargetLength({ text: String(POST_TARGET_LENGTH_MAX + 1) }))
    expect(result.current.lengthValid).toBe(false)
    expect(result.current.valid).toBe(false)

    act(() => result.current.setTargetLength({ text: '' }))
    expect(result.current.lengthValid).toBe(false)
    expect(result.current.dirty).toBe(true)

    act(() => result.current.setTargetLength({ text: '1200' }))
    expect(result.current.lengthValid).toBe(true)
    expect(result.current.valid).toBe(true)
  })
})

describe('what can refuse a save', () => {
  // TMPL-30, TMPL-7: a draft that does not parse cannot be saved from either mode.
  it('refuses a body that does not parse, and names the area that failed', () => {
    const { result } = setup(stored({ body: '<write>닫히지 않음' }))

    act(() => result.current.setText('name')('옛 템플릿'))
    expect(result.current.dirty).toBe(true)
    expect(result.current.valid).toBe(false)
    expect(result.current.failureIn('body')).toMatchObject({ area: 'body' })
    expect(result.current.failureIn('title_area')).toBeNull()
  })

  // TMPL-44: a conflicting row is left out of the text, so only its flag can hold the save.
  it('refuses while either area reports two rows asking under one title', () => {
    const { result } = setup(stored())
    act(() => result.current.setText('name')('리뷰 2편'))
    expect(result.current.valid).toBe(true)

    act(() => result.current.onBodyAskConflict(true))
    expect(result.current.valid).toBe(false)
    act(() => result.current.onBodyAskConflict(false))
    act(() => result.current.onTitleAskConflict(true))
    expect(result.current.valid).toBe(false)
    act(() => result.current.onTitleAskConflict(false))
    expect(result.current.valid).toBe(true)
  })

  // TMPL-55: the title's data fields come first, so the body is told which titles are taken.
  it("hands the body the title area's data-field labels", () => {
    const { result } = setup(
      stored({ titleArea: '<ask label="가게 이름"/> 방문 후기 <write>메뉴를 한 줄로</write>' }),
    )

    expect([...result.current.titleAskLabels]).toEqual(['가게 이름'])
  })
})

describe('a request answer', () => {
  // TMPL-60, TMPL-63: the answer replaces the four texts, never the two numbers.
  it('replaces the four texts, keeps the numbers, and makes the draft dirty', () => {
    const { result } = setup(stored({ targetLength: 1500 }))
    act(() => result.current.markSaved())

    act(() =>
      result.current.applyRequest({
        name: '맛집 리뷰',
        description: '맛집 방문기',
        titleArea: '',
        body: '<write>방문 이유</write>',
      }),
    )
    expect(result.current.draft).toMatchObject({
      name: '맛집 리뷰',
      description: '맛집 방문기',
      body: '<write>방문 이유</write>',
    })
    expect(result.current.lengthField).toEqual({ enabled: true, text: '1500' })
    expect(result.current.dirty).toBe(true)
    expect(result.current.saved).toBe(false)
  })

  it('is one stable function, since the request applies it from an effect', () => {
    const { result, rerender } = setup(stored())
    const first = result.current.applyRequest
    act(() => result.current.setText('name')('다른 이름'))
    rerender()
    expect(result.current.applyRequest).toBe(first)
  })
})

// TMPL-25 and TMPL-6: what the server stored is the new baseline, and the draft shows it.
describe('a landed save', () => {
  it('goes clean on what the server stored, not on the draft it sent', () => {
    const { result } = setup(stored())
    act(() => result.current.setText('body')('  <write>새 인트로</write>  '))
    expect(result.current.dirty).toBe(true)

    act(() => {
      result.current.adoptSaved(stored({ body: '<write>새 인트로</write>' }))
      result.current.markSaved()
    })
    expect(result.current.draft.body).toBe('<write>새 인트로</write>')
    expect(result.current.dirty).toBe(false)
    expect(result.current.saved).toBe(true)

    // Any edit after it takes the confirmation away.
    act(() => result.current.setText('name')('새 이름'))
    expect(result.current.saved).toBe(false)
  })

  it('goes clean on the draft it sent when the answer carries no template', () => {
    const { result } = setup(stored())
    act(() => result.current.setText('name')(' 새 이름 '))

    act(() => result.current.adoptSaved(undefined))
    expect(result.current.dirty).toBe(false)
    // The draft itself is left as typed.
    expect(result.current.draft.name).toBe(' 새 이름 ')
  })
})
