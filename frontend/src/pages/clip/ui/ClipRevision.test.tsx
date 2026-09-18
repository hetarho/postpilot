import { afterEach, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
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
    answers: [],
    disclosure: 'ad',
    cta: '',
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
          informationFields: [],
          cutGuidance: '',
          accent: '',
          preset: 'restaurant',
        },
      ],
      projects: [fixture()],
      ...clips,
    },
  })
  await screen.findByRole('heading', { name: '컷·자막 수정' })
  return view
}
const panel = () => within(screen.getByLabelText('클립 수정 작업'))
async function write(text: string) {
  await userEvent.type(screen.getByLabelText('요청 내용'), text)
}

it('keeps two dock rows and opens approval from send without starting a revision', async () => {
  const starts: unknown[] = []
  await mount({ revisionStarts: starts })
  const dockElement = screen.getByLabelText('클립 수정 작업')
  const dock = within(dockElement)
  expect(dockElement.children).toHaveLength(2)
  expect(
    within(dockElement.children[0] as HTMLElement)
      .getAllByRole('button')
      .map((b) => b.textContent),
  ).toEqual(['다시 렌더 · 서버', '확정하기'])
  expect(dockElement.children[1]).toContainElement(dock.getByLabelText('요청 내용'))
  expect(screen.queryByRole('region', { name: 'AI에 수정 요청' })).not.toBeInTheDocument()
  expect(dock.queryByRole('button', { name: /브라우저/ })).not.toBeInTheDocument()
  // Approval and download never increase the dock height.
  expect(dock.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
  expect(screen.queryByText(/확정하면 원본을 삭제/)).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeInTheDocument()
  await write('자막을 더 짧게')
  // Counted CDS-20's way, like every other bounded clip field.
  expect(panel().getByText('6 / 1000자')).toBeInTheDocument()
  // 자막 by default: the narration is what an owner asks about most, and it is
  // the one target that leaves the footage exactly where they put it.
  const targets = within(panel().getByRole('tablist', { name: '고칠 대상' }))
  expect(targets.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
    '영상 흐름',
    '자막',
    '둘 다',
  ])
  expect(targets.getByRole('tab', { name: '자막' })).toHaveAttribute('aria-selected', 'true')
  // The ceiling and the writing calls it pays for, with the cancellation rule.
  expect(screen.queryByRole('button', { name: /승인하고 수정 요청/ })).not.toBeInTheDocument()
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  const approve = await screen.findByRole(
    'button',
    { name: '최대 8 크레딧 · 승인하고 수정 요청' },
    { timeout: 3000 },
  )
  expect(screen.getByText('자막 작성 1회')).toBeInTheDocument()
  expect(screen.getByText(/사용하지 않은 예약액|취소하면/, { exact: false })).toBeInTheDocument()
  expect(starts).toHaveLength(0)
  await userEvent.click(approve)
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({ request: '자막을 더 짧게', target: 'narration' })
})

// The priced calls follow the target: the narration alone is one writing call,
// anything that moves the footage is two (CLIP-135, CLIP-131).
it('re-quotes when the target changes', async () => {
  const quotes: unknown[] = []
  await mount({ revisionQuotes: quotes })
  await write('고기를 먼저 보여줘')
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  await screen.findByRole(
    'button',
    { name: '최대 8 크레딧 · 승인하고 수정 요청' },
    { timeout: 3000 },
  )
  await userEvent.keyboard('{Escape}')
  await userEvent.click(panel().getByRole('tab', { name: '영상 흐름' }))
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  await screen.findByRole(
    'button',
    { name: '최대 16 크레딧 · 승인하고 수정 요청' },
    { timeout: 3000 },
  )
  expect(screen.getByText('컷 구성 1회')).toBeInTheDocument()
  expect(screen.getByText('자막 작성 1회')).toBeInTheDocument()
  expect(quotes).toHaveLength(2)
  expect(quotes[1]).toMatchObject({ target: 'flow' })
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
    within(screen.getByLabelText('편집 타임라인')).getByRole('button', {
      name: /컷 1/,
    }),
  )
  expect(screen.getByRole('button', { name: '컷 삭제' })).toBeEnabled()
  await write('자막을 더 짧게')
  await userEvent.click(panel().getByRole('button', { name: 'AI에 수정 요청' }))
  await userEvent.click(
    await screen.findByRole(
      'button',
      { name: '최대 8 크레딧 · 승인하고 수정 요청' },
      { timeout: 3000 },
    ),
  )
  // The panel answers with the run; the request field is gone while it lasts.
  expect(await panel().findByText('컷 구성', undefined, { timeout: 3000 })).toBeInTheDocument()
  expect(panel().getByRole('button', { name: '취소' })).toBeEnabled()
  expect(screen.queryByLabelText('요청 내용')).not.toBeInTheDocument()
  // ② is still the screen: its step bar, its timeline and its preview stay, and
  // no focused job view took over.
  expect(screen.getByRole('tab', { name: '클립 다듬기' })).toBeInTheDocument()
  expect(screen.getByLabelText('편집 타임라인')).toBeInTheDocument()
  await waitFor(() => expect(screen.getByRole('button', { name: '컷 삭제' })).toBeDisabled())
})
