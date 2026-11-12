import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { clipDisplayTitle } from './display-title'

afterEach(() => initializeI18n('ko'))

// CLIP-130: the title is optional, and an empty one is named until the owner gives one.
it.each([
  ['ko', '제목 없는 영상'],
  ['en', 'Untitled video'],
] as const)('names an empty or blank title in %s', (locale, untitled) => {
  initializeI18n(locale)
  expect(clipDisplayTitle({ title: '' })).toBe(untitled)
  expect(clipDisplayTitle({ title: '   ' })).toBe(untitled)
})

it('shows a typed title trimmed', () => {
  expect(clipDisplayTitle({ title: '  제주 여행 ' })).toBe('제주 여행')
})
