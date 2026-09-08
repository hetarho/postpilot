import { describe, expect, it } from 'vitest'
import type { Template } from '@/entities/template'
import { answerFields, toAnswerPatch, withAnswer } from './answers'

const template = (body: string): Template => ({
  id: 'template-review',
  name: '리뷰',
  description: '',
  body,
  postCount: 0,
  createdAt: '',
  updatedAt: '',
})

describe('the fields the write screen asks for', () => {
  it('follows body order and carries this post’s answers', () => {
    const fields = answerFields(
      template('오늘의 기록\n<ask label="방문일"/>\n<ask label="총평">총평을 쓰세요</ask>'),
      [{ label: '총평', text: '4.5점', enabled: false }],
    )
    expect(fields).toEqual([
      { label: '방문일', flavor: 'verbatim', text: '', enabled: true },
      { label: '총평', flavor: 'write', text: '4.5점', enabled: false },
    ])
  })

  // The field exists because the template requires that data, and a blank one is dropped at the
  // freeze anyway — so an unanswered field is presented switched ON (POST-62).
  it('presents an unanswered field switched on and empty', () => {
    const [field] = answerFields(template('<ask label="방문일"/>'), [])
    expect(field).toMatchObject({ text: '', enabled: true })
  })

  it('asks for nothing without a template, without a field, or from a body that does not parse', () => {
    expect(answerFields(undefined, [])).toEqual([])
    expect(answerFields(template('<write>인트로</write>'), [])).toEqual([])
    expect(answerFields(template('<ask label="총평"/>\n<ask label="총평"/>'), [])).toEqual([])
  })

  // An answer typed under another template is not rendered and not sent, but it is not touched
  // either: the patch is upsert-only and the label is the key.
  it('ignores an answer whose label the current template does not declare', () => {
    const fields = answerFields(template('<ask label="방문일"/>'), [
      { label: '다른 템플릿의 칸', text: '값', enabled: true },
    ])
    expect(fields).toEqual([{ label: '방문일', flavor: 'verbatim', text: '', enabled: true }])
    expect(toAnswerPatch(fields)).toEqual([{ label: '방문일', text: '', enabled: true }])
  })

  it('applies one edit by label and leaves the others alone', () => {
    const fields = answerFields(template('<ask label="방문일"/>\n<ask label="총평"/>'), [])
    const typed = withAnswer(fields, '총평', { text: '4.5점' })
    expect(toAnswerPatch(typed)).toEqual([
      { label: '방문일', text: '', enabled: true },
      { label: '총평', text: '4.5점', enabled: true },
    ])
    const switched = withAnswer(typed, '총평', { enabled: false })
    expect(toAnswerPatch(switched)[1]).toEqual({ label: '총평', text: '4.5점', enabled: false })
  })
})
