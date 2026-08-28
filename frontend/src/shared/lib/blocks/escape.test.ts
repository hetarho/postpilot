import { expect, it } from 'vitest'
import {
  escapeHtml,
  escapeHtmlComment,
  escapeMarkdownLabel,
  relativeFileUrl,
  yamlString,
} from './escape'

it('escapes HTML text and attributes', () => {
  expect(escapeHtml(`<바다> & "바람" '비'`)).toBe(
    '&lt;바다&gt; &amp; &quot;바람&quot; &#39;비&#39;',
  )
})

it('returns a YAML-safe double-quoted scalar', () => {
  expect(yamlString('제목 "하나"\n둘\\셋')).toBe('"제목 \\"하나\\"\\n둘\\\\셋"')
})

it('escapes filename contexts without leaking a URL', () => {
  expect(relativeFileUrl("여행 photo (2)?#'.jpg")).toBe(
    '%EC%97%AC%ED%96%89%20photo%20%282%29%3F%23%27.jpg',
  )
  expect(escapeHtmlComment('photo-->next.jpg')).toBe('photo-\u200b->next.jpg')
  expect(escapeMarkdownLabel('대괄호 ]와 \\')).toBe('대괄호 \\]와 \\\\')
})
