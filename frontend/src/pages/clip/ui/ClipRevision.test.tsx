import { afterEach, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoGuidelineScope, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeGuidelinesOptions } from '@/test/guidelines'
import type { FakeJobsOptions } from '@/test/jobs'
import type { FakeProvidersOptions } from '@/test/providers'

afterEach(() => initializeI18n('ko'))

const models: FakeProvidersOptions = {
  models: [
    {
      providerId: 'p',
      modelId: 'o',
      label: 'Video observer',
      vision: true,
      videoInput: true,
      inlineStaticVideo: true,
      stages: [Stage.OBSERVE],
    },
    { providerId: 'p', modelId: 'w', label: 'Writer', stages: [Stage.WRITE] },
  ],
  selections: [
    { stage: Stage.OBSERVE, providerId: 'p', modelId: 'o' },
    { stage: Stage.WRITE, providerId: 'p', modelId: 'w' },
  ],
}
function fixture(): FakeClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: 'template',
    ratio: 'vertical',
    targetDurationMs: 19800,
    disclosure: 'ad',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: clipTimelineFixture(),
    result: {
      contentType: 'video/mp4',
      bytes: 5,
      durationMs: 19800,
      createdAt: '2026-09-10T00:00:00Z',
      viewUrl: 'https://private.test/old',
      downloadUrl: 'https://private.test/download',
    },
  }
}
async function mount(clips: FakeClipsOptions = {}, jobs: FakeJobsOptions = {}) {
  const view = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: models,
    jobs,
    clips: {
      templates: [
        {
          id: 'template',
          name: '여행',
          compositionBody: '<clip version="1"/>',
        },
      ],
      projects: [fixture()],
      ...clips,
    },
  })
  await screen.findByRole('region', { name: '컷·자막 수정' })
  return view
}
const panel = () => within(screen.getByLabelText('클립 수정 작업'))
async function write(text: string) {
  await userEvent.type(screen.getByLabelText('수정 요청을 입력하세요'), text)
}

// ②'s step bar (CLIP-40 →POST-56): the field's heading is its label, the field and its icon send
// sit under it, then the row of 이전 and 확정하기; there is no target to choose (CLIP-131).
it('keeps one composer in the step bar and opens only the approval from send', async () => {
  const starts: unknown[] = []
  await mount({ revisionStarts: starts })
  const dockElement = screen.getByLabelText('클립 수정 작업')
  const dock = within(dockElement)
  expect(
    dock.getAllByRole('button').map((b) => b.getAttribute('aria-label') ?? b.textContent),
  ).toEqual(['AI에 수정 요청', '이전: 스토리라인', '확정하기'])
  expect(dock.queryByRole('tablist', { name: '고칠 대상' })).not.toBeInTheDocument()
  expect(dock.queryByRole('button', { name: /렌더|브라우저|서버/ })).not.toBeInTheDocument()
  expect(dock.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
  expect(screen.queryByText(/완료로 표시하면 원본을 삭제/)).not.toBeInTheDocument()
  // ② carries no download: it stands under the delivered video in ③ (CLIP-149).
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
  // The counter appears with the first character, not before.
  expect(dock.queryByText(/1000자/)).not.toBeInTheDocument()
  await write('자막을 더 짧게')
  // Counted in every character, spaces included, as the server bounds it (CLIP-121).
  expect(panel().getByText('8 / 1000자')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /승인하고 수정 요청/ })).not.toBeInTheDocument()
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'AI에 수정 요청' }))
  expect(sheet.queryByRole('tablist')).not.toBeInTheDocument()
  // The ceiling is the two writing calls a request may need, with the cancellation rule
  // (CLIP-132).
  const approve = await sheet.findByRole(
    'button',
    { name: '최대 16 크레딧 · 승인하고 수정 요청' },
    { timeout: 3000 },
  )
  expect(sheet.getByText('컷 구성 1회')).toBeInTheDocument()
  expect(sheet.getByText('자막 작성 1회')).toBeInTheDocument()
  expect(sheet.getByText(/사용하지 않은 예약액|취소하면/, { exact: false })).toBeInTheDocument()
  expect(starts).toHaveLength(0)
  await userEvent.click(approve)
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({ request: '자막을 더 짧게', target: '' })
})

// The owner stays in ② while it runs (CLIP-131): the plan being rewritten is the
// one on this screen. The timeline goes read-only because an edit made against a
// plan that is being replaced has nowhere to land.
it('keeps ② mounted and read-only while the request runs, with progress and 취소 replacing the composer', async () => {
  await mount(
    { revisionJobId: 'revision-job' },
    {
      jobs: [
        {
          id: 'revision-job',
          kind: 'revise_clip',
          status: 'running',
          stage: 'flow',
          clipProjectId: 'clip',
          canCancel: true,
          cancellationPolicyVersion: 1,
        },
      ],
    },
  )
  await userEvent.click(
    within(screen.getByRole('region', { name: '장면 블록' })).getByRole('button', {
      name: '컷 1 편집',
    }),
  )
  expect(screen.getByRole('button', { name: '컷 삭제' })).toBeEnabled()
  await userEvent.keyboard('{Escape}')
  await write('자막을 더 짧게')
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  await userEvent.click(
    await screen.findByRole(
      'button',
      { name: '최대 16 크레딧 · 승인하고 수정 요청' },
      { timeout: 3000 },
    ),
  )
  // The panel answers with the run; the request field is gone while it lasts.
  expect(await panel().findByText('컷 구성', undefined, { timeout: 3000 })).toBeInTheDocument()
  expect(panel().getByRole('button', { name: '취소' })).toBeEnabled()
  expect(screen.queryByLabelText('수정 요청을 입력하세요')).not.toBeInTheDocument()
  // ② is still the screen: its step bar, its timeline and its preview stay, and
  // no focused job view took over.
  expect(screen.getByRole('tab', { name: '다듬기' })).toBeInTheDocument()
  expect(screen.getByRole('region', { name: '장면 블록' })).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '컷 1 편집' }))
  await waitFor(() => expect(screen.getByRole('button', { name: '컷 삭제' })).toBeDisabled())
})

// GUIDE-45: once the request THIS session started completes, its words can become a standing
// 영상 지침 — seeded with the request, 전역 by default or the project's own video template.
it('offers 영상 지침으로 저장 after a completed revision started here, seeded with the request', async () => {
  const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: models,
    jobs: {
      jobs: [
        {
          id: 'revision-job',
          kind: 'revise_clip',
          status: 'done',
          stage: 'narration',
          clipProjectId: 'clip',
        },
      ],
    },
    clips: {
      templates: [{ id: 'template', name: '여행', compositionBody: '<clip version="1"/>' }],
      projects: [fixture()],
      revisionJobId: 'revision-job',
    },
    guidelines: { creates },
  })
  await screen.findByRole('region', { name: '컷·자막 수정' })
  // Nothing was asked in this session yet, so there is nothing to save.
  expect(screen.queryByRole('button', { name: '영상 지침으로 저장' })).not.toBeInTheDocument()

  await write('자막을 더 짧게')
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  await userEvent.click(
    await screen.findByRole(
      'button',
      { name: '최대 16 크레딧 · 승인하고 수정 요청' },
      { timeout: 3000 },
    ),
  )
  await userEvent.click(
    await panel().findByRole('button', { name: '영상 지침으로 저장' }, { timeout: 3000 }),
  )
  const dialog = within(await screen.findByRole('dialog', { name: '영상 지침으로 저장' }))
  expect(dialog.getByLabelText('영상 지침')).toHaveValue('자막을 더 짧게')
  expect(dialog.getByRole('tab', { name: '전역' })).toHaveAttribute('aria-selected', 'true')
  await userEvent.click(await dialog.findByRole('tab', { name: '이 영상의 템플릿 「여행」에만' }))
  // GUIDE-46: the optional title rides the create when one is typed.
  await userEvent.type(dialog.getByLabelText('제목'), '짧은 자막')
  await userEvent.click(dialog.getByRole('button', { name: '저장' }))

  await waitFor(() => expect(creates).toHaveLength(1))
  expect(creates[0]).toEqual({
    kind: 'clip',
    title: '짧은 자막',
    text: '자막을 더 짧게',
    scope: ProtoGuidelineScope.TEMPLATES,
    templateIds: ['template'],
    fields: [],
  })
  expect(await panel().findByText('영상 지침으로 저장했어요.')).toBeInTheDocument()
})

it('keeps a running upload-text job in ③ with its own cancel action and none in ②', async () => {
  const latestJob = {
    id: 'upload',
    kind: 'clip_upload_text',
    status: 'running',
    stage: 'upload_text',
    clipProjectId: 'clip',
  }
  await mount({ projects: [{ ...fixture(), latestJob }] }, { jobs: [latestJob] })
  expect(await screen.findByRole('region', { name: '장면 블록' })).toBeVisible()
  expect(screen.getByRole('tab', { name: '다듬기' })).toHaveAttribute('aria-selected', 'true')
  // ② no longer offers the 업로드 문구 (CLIP-211); the running write keeps its cancel in ③.
  expect(screen.queryByRole('button', { name: '문구 작성 취소' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '업로드 문구 만들기' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: '완성' }))
  expect(await screen.findByRole('button', { name: '문구 작성 취소' })).toBeEnabled()
})
