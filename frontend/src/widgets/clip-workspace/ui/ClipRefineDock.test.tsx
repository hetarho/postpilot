import { afterEach, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
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
function fixture(overrides: Pick<FakeClipProject, 'confirmationRefusal'> = {}): FakeClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: '',
    ratio: 'vertical',
    targetDurationMs: 19800,
    disclosure: '',
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
    ...overrides,
  }
}
async function mount(clips: FakeClipsOptions = {}) {
  const view = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: models,
    clips: { projects: [fixture()], ...clips },
  })
  await screen.findByRole('region', { name: '컷·자막 수정' })
  return view
}
const bar = () => within(screen.getByLabelText('클립 수정 작업'))
const blocks = () => within(screen.getByRole('region', { name: '장면 블록' }))

// CLIP-40 →POST-56: the field's visible heading is its label, the send is an icon, and the row
// under it is the way back and 확정하기, with no target control and no render in the bar.
it('docks the revision field and 확정하기 the way the post’s 글 다듬기 does', async () => {
  await mount()
  const label = bar().getByText('수정 요청을 입력하세요')
  expect(label.tagName).toBe('LABEL')
  expect(bar().getByLabelText('수정 요청을 입력하세요').tagName).toBe('TEXTAREA')
  const send = bar().getByRole('button', { name: 'AI에 수정 요청' })
  expect(send).toHaveTextContent('')
  expect(bar().getByRole('button', { name: '이전: 영상 생성' })).toBeInTheDocument()
  expect(bar().getByRole('button', { name: '확정하기' })).toHaveClass('bg-button-cta-bg')
  expect(bar().queryByRole('button', { name: /렌더/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('tablist', { name: '고칠 대상' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'AI로 수정' })).not.toBeInTheDocument()
  // ② keeps no render and no download: both stand in ③ (CLIP-149, CLIP-153).
  expect(screen.queryByRole('button', { name: /^(렌더하기|다시 렌더)$/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
})

// CLIP-131/132: one request quoted as auto at the two-call ceiling, scoped to the selected block
// by a removable chip, and the whole clip without one.
it('quotes an auto request scoped by the selected block’s chip and approves it', async () => {
  const quotes: Array<{ target: string; scopeCut?: number }> = []
  const starts: Array<{ request: string; target: string; scopeCut?: number }> = []
  await mount({ revisionQuotes: quotes, revisionStarts: starts })
  expect(bar().queryByRole('button', { name: /장면 \d/ })).not.toBeInTheDocument()
  await userEvent.click(blocks().getByRole('button', { name: '컷 2' }))
  const chip = bar().getByRole('button', { name: '장면 2 선택 해제' })
  expect(chip).toHaveTextContent('장면 2')
  await userEvent.type(bar().getByLabelText('수정 요청을 입력하세요'), '이 장면을 빠르게')
  await userEvent.click(bar().getByRole('button', { name: 'AI에 수정 요청' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'AI에 수정 요청' }))
  expect(sheet.queryByRole('tablist')).not.toBeInTheDocument()
  const approve = await sheet.findByRole(
    'button',
    { name: '최대 16 크레딧 · 승인하고 수정 요청' },
    { timeout: 3000 },
  )
  expect(quotes.at(-1)).toMatchObject({ target: '', scopeCut: 2 })
  await userEvent.click(approve)
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({ request: '이 장면을 빠르게', target: '', scopeCut: 2 })
})

it('drops the scope with its chip so the request covers the whole clip', async () => {
  const quotes: Array<{ target: string; scopeCut?: number }> = []
  await mount({ revisionQuotes: quotes })
  await userEvent.click(blocks().getByRole('button', { name: '컷 1' }))
  await userEvent.click(bar().getByRole('button', { name: '장면 1 선택 해제' }))
  expect(bar().queryByRole('button', { name: /장면 \d/ })).not.toBeInTheDocument()
  await userEvent.type(bar().getByLabelText('수정 요청을 입력하세요'), '전체를 밝게')
  await userEvent.click(bar().getByRole('button', { name: 'AI에 수정 요청' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'AI에 수정 요청' }))
  await sheet.findByRole('button', { name: /승인하고 수정 요청/ }, { timeout: 3000 })
  expect(quotes.at(-1)).toMatchObject({ target: '' })
  expect(quotes.at(-1)?.scopeCut).toBeUndefined()
})

// CLIP-75: 확정하기 flushes the queues, confirms the saved revision and opens ③ at once.
it('flushes, confirms the saved plan revision and moves to ③ with no dialog', async () => {
  const calls: string[] = []
  const confirmations: Array<{ expectedPlanRevision: number }> = []
  await mount({ calls, confirmations })
  await userEvent.click(bar().getByRole('button', { name: '확정하기' }))
  await waitFor(() => expect(confirmations).toHaveLength(1))
  expect(confirmations[0]).toMatchObject({ expectedPlanRevision: 1 })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await waitFor(() =>
    expect(screen.getByRole('tab', { name: '영상 완성' })).toHaveAttribute('aria-selected', 'true'),
  )
  expect(calls).not.toContain('FinalizeClipProject')
  expect(screen.queryByLabelText('클립 수정 작업')).not.toBeInTheDocument()
})

it('saves an unsaved edit before confirming the revision it landed on', async () => {
  const confirmations: Array<{ expectedPlanRevision: number }> = []
  const planWrites: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ confirmations, planWrites })
  await userEvent.click(blocks().getByRole('button', { name: '컷 1 편집' }))
  fireEvent.change(screen.getByLabelText('원본 시작 (초)'), { target: { value: '0.5' } })
  await userEvent.click(bar().getByRole('button', { name: '확정하기' }))
  await waitFor(() => expect(confirmations).toHaveLength(1))
  expect(planWrites.length).toBeGreaterThan(0)
  expect(confirmations[0]).toMatchObject({ expectedPlanRevision: 2 })
})

it.each([
  ['narration', '바뀐 대본의 음성을 다시 만든 뒤 확정해 주세요.'],
  ['invalid_plan', '편집 내용의 오류를 고친 뒤 확정해 주세요.'],
  ['unavailable', '확정할 수 있는지 확인하지 못했어요. 화면을 새로고침해 주세요.'],
] as const)('states the %s refusal above the row and holds 확정하기', async (refusal, message) => {
  await mount({ projects: [fixture({ confirmationRefusal: refusal })] })
  const reason = bar().getByText(message)
  const confirm = bar().getByRole('button', { name: '확정하기' })
  expect(confirm).toBeDisabled()
  expect(confirm).toHaveAccessibleDescription(message)
  expect(
    reason.compareDocumentPosition(bar().getByLabelText('수정 요청을 입력하세요')) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy()
  if (refusal === 'narration')
    expect(bar().getByRole('button', { name: '대본 열기' })).toBeInTheDocument()
})
