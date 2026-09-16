import { afterEach, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
import type { FakeClipProject } from '@/test/clips'

afterEach(() => initializeI18n('ko'))

const template = {
  id: 'template',
  name: '여행',
  informationFields: [],
  cutGuidance: '',
  accent: '' as const,
  preset: 'restaurant' as const,
}
const project: FakeClipProject = {
  id: 'clip',
  title: '제주',
  videoTemplateId: 'template',
  ratio: 'vertical',
  targetDurationMs: 30000,
  answers: [],
  disclosure: 'ad',
  cta: '',
  // Newest first, exactly as the server answers (CLIP-133).
  requests: [
    { kind: 'revision:narration', body: '자막을 줄여줘', createdAt: '2026-09-16T04:00:00Z' },
    { kind: 'instruction', body: '', createdAt: '2026-09-15T01:00:00Z' },
  ],
}

// What the owner asked for is theirs to read back, in the project, in order —
// and a clip written with no instruction says so rather than showing nothing.
it('reads back what was asked for, newest first, including a generation with no instruction', async () => {
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: { models: [] },
    clips: { templates: [template], projects: [project] },
  })
  await userEvent.click(await screen.findByText('AI에 요청한 내용 2건'))
  const entries = within(screen.getByRole('list', { name: 'AI에 요청한 내용 2건' })).getAllByRole(
    'listitem',
  )
  expect(entries).toHaveLength(2)
  expect(entries[0]).toHaveTextContent('수정 요청 · 자막')
  expect(entries[0]).toHaveTextContent('자막을 줄여줘')
  expect(entries[1]).toHaveTextContent('생성 지시')
  expect(entries[1]).toHaveTextContent('지시 없이 생성했어요.')
})
