import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
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
  answers: [],
  disclosure: '',
  cta: '',
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
  await screen.findByRole('link', { name: '영상 다운로드' })
  expect(screen.getByRole('tab', { name: '완성' })).toHaveAttribute('aria-selected', 'true')
  for (const name of ['생성', '수정']) expect(screen.getByRole('tab', { name })).toBeVisible()
})

it('states ①’s settings without a control that could change them', async () => {
  mount()
  await screen.findByRole('link', { name: '영상 다운로드' })
  await userEvent.click(screen.getByRole('tab', { name: '생성' }))
  const title = await screen.findByLabelText('클립 제목')
  expect(title).toHaveValue('확정된 클립')
  expect(title).toBeDisabled()
  // ① commits nothing while reading: no generation, no upload, no model choice (CLIP-160 r43).
  expect(screen.queryByRole('button', { name: /생성하기|영상 만들기/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '영상 추가' })).not.toBeInTheDocument()
  expect(screen.queryByText('생성 모델')).not.toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
})

it('plays the confirmed result in ② and opens a caption as values', async () => {
  const calls: string[] = []
  mount({ calls })
  await screen.findByRole('link', { name: '영상 다운로드' })
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
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

  const timeline = within(screen.getByLabelText('편집 타임라인'))
  await userEvent.click(timeline.getByRole('button', { name: 'caption a' }))
  const sheet = within(await screen.findByRole('dialog', { name: '선택한 문구' }))
  expect(sheet.getByText('caption a')).toBeVisible()
  expect(sheet.getByText('자막 속도')).toBeVisible()
  expect(sheet.queryByRole('textbox')).not.toBeInTheDocument()
  expect(sheet.queryByLabelText('자막 원문')).not.toBeInTheDocument()
  await userEvent.keyboard('{Escape}')

  await userEvent.click(timeline.getByRole('button', { name: '컷 1' }))
  const cutSheet = within(await screen.findByRole('dialog'))
  expect(cutSheet.getByText('원본 구간')).toBeVisible()
  expect(cutSheet.queryByLabelText('원본 시작 (초)')).not.toBeInTheDocument()
  expect(cutSheet.queryByRole('button', { name: '컷 삭제' })).not.toBeInTheDocument()
  await userEvent.keyboard('{Escape}')

  // What the clip was observed from is still reachable from ② (CLIP-150, CLIP-160).
  await userEvent.click(screen.getByRole('button', { name: '원본 소스' }))
  expect(
    within(screen.getByRole('dialog', { name: '원본 소스' })).getByRole('heading', {
      name: 'AI가 관찰한 내용',
    }),
  ).toBeVisible()
})
