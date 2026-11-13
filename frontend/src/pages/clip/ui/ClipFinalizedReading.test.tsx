import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { Stage } from '@/shared/api'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import { clipObservationsFixture } from '@/test/clip-observations'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'

afterEach(() => initializeI18n('ko'))

/** A confirmed clip whose originals finalization deleted, with the plan and the evidence the
 *  server still serves as readings (CLIP-160). */
const finalized = (): FakeClipProject => ({
  id: 'clip',
  title: '확정된 클립',
  videoTemplateId: '',
  ratio: 'vertical',
  targetDurationMs: 19800,
  disclosure: '',
  editPlanRevision: 1,
  renderedPlanRevision: 1,
  editing: clipTimelineFixture(),
  observations: clipObservationsFixture(),
  finalized: { at: '2026-09-13T00:00:00Z', planRevision: 1, resultId: 'render-1' },
  result: {
    id: 'render-1',
    contentType: 'video/mp4',
    bytes: 5,
    durationMs: 19800,
    createdAt: '2026-09-13T00:00:00Z',
    viewUrl: 'https://private.test/result',
    downloadUrl: 'https://private.test/download',
  },
})
const mount = (options: FakeClipsOptions = {}) =>
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    clips: { projects: [finalized()], ...options },
  })

it('opens a finalized clip on ③ and keeps its earlier steps selectable', async () => {
  mount()
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  expect(screen.getByRole('tab', { name: '완성' })).toHaveAttribute('aria-selected', 'true')
  for (const name of ['생성', '다듬기']) expect(screen.getByRole('tab', { name })).toBeVisible()
})

it('states ①’s settings without a control that could change them', async () => {
  mount()
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  await userEvent.click(screen.getByRole('tab', { name: '생성' }))
  const title = await screen.findByLabelText('클립 제목')
  expect(title).toHaveValue('확정된 클립')
  expect(title).toBeDisabled()
  // ① commits nothing while reading: no generation, no upload, no model choice (CLIP-160 r43).
  expect(screen.queryByRole('button', { name: /생성하기|영상 만들기/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '영상 추가' })).not.toBeInTheDocument()
  expect(screen.queryByText('생성 모델')).not.toBeInTheDocument()
  // The observations are ②'s, read with no step bar (CLIP-160, CLIP-48).
  expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: '관찰' }))
  expect(screen.getByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
  expect(screen.queryByRole('button', { name: '바로 만들기' })).not.toBeInTheDocument()
})

it('plays the confirmed result in ④ and opens a caption as values', async () => {
  const calls: string[] = []
  mount({ calls })
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  await userEvent.click(screen.getByRole('tab', { name: '다듬기' }))
  // The delivered file stands where the draft preview did: the originals are gone, so no draft
  // is prepared and no fragment is drawn (CLIP-160, CLIP-76).
  expect(await screen.findByLabelText('클립 미리보기')).toHaveAttribute(
    'src',
    finalized().result!.viewUrl,
  )
  expect(calls).not.toContain('PrepareClipPreview')
  expect(calls).not.toContain('GetClipCaptionPreview')
  // No dock: nothing in ② asks for a rewrite, a render, a download or a confirmation.
  for (const name of ['렌더하기', '다시 렌더', '확정하기'])
    expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
  expect(screen.queryByLabelText('수정 요청을 입력하세요')).not.toBeInTheDocument()

  const timeline = within(screen.getByRole('region', { name: '장면 블록' }))
  await userEvent.click(
    within(
      timeline.getByDisplayValue('caption a').closest<HTMLElement>('[data-scene-text]')!,
    ).getByRole('button', { name: /편집$/ }),
  )
  const sheet = within(await screen.findByRole('dialog', { name: '선택한 문구' }))
  expect(sheet.getByText('caption a')).toBeVisible()
  expect(sheet.getByText('자막 속도')).toBeVisible()
  expect(sheet.queryByRole('textbox')).not.toBeInTheDocument()
  expect(sheet.queryByLabelText('자막 원문')).not.toBeInTheDocument()
  await userEvent.keyboard('{Escape}')

  await userEvent.click(timeline.getByRole('button', { name: '컷 1 편집' }))
  const cutSheet = within(await screen.findByRole('dialog'))
  expect(cutSheet.getByText('원본 구간')).toBeVisible()
  expect(cutSheet.queryByLabelText('원본 시작 (초)')).not.toBeInTheDocument()
  expect(cutSheet.queryByRole('button', { name: '컷 삭제' })).not.toBeInTheDocument()
  await userEvent.keyboard('{Escape}')

  // ④'s reference keeps only 원본 영상 and 요청 기록; the observations stand in ② (CLIP-150).
  await userEvent.click(screen.getByRole('button', { name: '원본 소스' }))
  expect(
    within(screen.getByRole('dialog', { name: '원본 소스' })).queryByRole('heading', {
      name: 'AI가 관찰한 내용',
    }),
  ).not.toBeInTheDocument()
})

it('opens phone appearance details on a finalized clip while retaining disabled editing controls and no writes', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  mount({ calls })
  await user.click(await screen.findByRole('tab', { name: '생성' }))
  const toggle = await screen.findByRole('button', { name: '디자인과 자막 스타일' })
  expect(toggle).toBeEnabled()
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await user.click(toggle)
  const intro = screen.getByRole('radiogroup', { name: '인트로 디자인' })
  for (const control of within(intro).getAllByRole('radio')) expect(control).toBeDisabled()
  for (const control of within(screen.getByRole('tablist', { name: '자막 흐름' })).getAllByRole(
    'tab',
  ))
    expect(control).toBeDisabled()
  const selected = within(intro).getByRole('radio', { checked: true })
  await user.click(within(intro).getByRole('radio', { name: '위아래 가로선' }))
  expect(selected).toHaveAttribute('aria-checked', 'true')
  await user.click(toggle)
  expect(screen.getByLabelText('클립 제목')).toBeDisabled()
  expect(calls.filter((name) => /^(Update|Save|Start|Finalize|Confirm|Cancel)/.test(name))).toEqual(
    [],
  )
})

it('offers upload text in the finished reading alone, the other channel from 더 해 보기, without reopening the video', async () => {
  const calls: string[] = [],
    starts: unknown[] = []
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    clips: { projects: [finalized()], calls, uploadTextStarts: starts },
    providers: {
      models: [{ providerId: 'p', modelId: 'w', label: '글 모델', stages: [Stage.WRITE] }],
      selections: [{ stage: Stage.WRITE, providerId: 'p', modelId: 'w' }],
    },
  })
  const make = await screen.findByRole('button', { name: '업로드 문구 만들기' })
  expect(make).toBeEnabled()
  expect(starts).toEqual([])
  // ② no longer offers it (CLIP-211): ③ is where the finished video's text stands.
  await userEvent.click(screen.getByRole('tab', { name: '다듬기' }))
  expect(screen.queryByRole('button', { name: '업로드 문구 만들기' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: '완성' }))
  await userEvent.click(await screen.findByRole('button', { name: '인스타 클립 업로드 문구 보기' }))
  expect(screen.getByRole('button', { name: '인스타 클립' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await userEvent.click(screen.getByRole('button', { name: '인스타 클립 문구도 만들기' }))
  expect(await screen.findByRole('button', { name: '최대 4 크레딧 · 문구 작성' })).toBeEnabled()
  expect(calls).toContain('QuoteClipUploadText')
  expect(
    calls.some((call) =>
      /StartClipRender|PrepareClipPreview|StartClipGeneration|CreateClipSourceBatch/u.test(call),
    ),
  ).toBe(false)
  expect(starts).toEqual([])
})

// CLIP-235, CLIP-160: a finalized project's ② reads its observations without a correction field
// or 다시 관찰하기.
it('reads ②’s observations without corrections or 다시 관찰하기 once finalized', async () => {
  mount()
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  await userEvent.click(screen.getByRole('tab', { name: '관찰' }))
  const panel = within(screen.getByRole('tabpanel', { name: '관찰' }))
  await userEvent.click(panel.getByRole('button', { name: /자세히 보기/ }))
  expect(panel.queryByRole('textbox')).not.toBeInTheDocument()
  expect(panel.queryByRole('button', { name: '다시 관찰하기' })).not.toBeInTheDocument()
})
