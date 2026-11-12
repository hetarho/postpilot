import { useState } from 'react'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { emptyClipProject, type ClipProject, type ClipProjectDraft } from '@/entities/clip-project'
import { ClipProjectForm } from './ClipProjectForm'
import { discardClipDraftQueues, flushClipDraft, queueClipDraft } from '../model/clip-draft-queue'

const ports = vi.hoisted(() => ({
  save: vi.fn(),
  voices: [] as Array<{ id: string; name: string; removedAt: string }>,
  voicePending: false,
  voiceFailed: false,
  templatePending: false,
  templateFailed: false,
}))
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  useBlocker: () => ({ status: 'idle' }),
  Link: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}))
vi.mock('@/entities/clip-project', async (original) => ({
  ...(await original<object>()),
  useClipProjectMutations: () => ({
    save: { mutateAsync: ports.save, isPending: false, error: null },
  }),
}))
vi.mock('@/entities/clip-template', async (original) => ({
  ...(await original<object>()),
  useClipTemplates: () => ({
    templates: [
      {
        id: 'template',
        name: '선택한 구성',
        compositionBody:
          '<clip version="1"><field id="place" label="장소" required="true" /></clip>',
      },
    ],
    isPending: ports.templatePending,
    isError: ports.templateFailed,
  }),
}))
vi.mock('@/entities/spoken-voice', () => ({
  useSpokenLibrary: () => ({
    voices: ports.voices,
    voicesQuery: { isPending: ports.voicePending, isError: ports.voiceFailed },
  }),
}))
vi.mock('./ClipLinkedPost', () => ({ ClipLinkedPost: () => <p>연결 입력</p> }))
vi.mock('./MoveClipChannel', () => ({ MoveClipChannel: () => null }))
vi.mock('./ClipDesignSelection', () => ({ ClipDesignSelection: () => <p>디자인 입력</p> }))

const project: ClipProject = {
  ...emptyClipProject(),
  id: 'one',
  title: '서버 제목',
  targetDurationMs: 30000,
  composition: {
    snapshot: { version: 1, body: '<clip version="1"/>', templateId: '' },
    inputs: { values: {}, items: {}, associations: [] },
  },
  createdAt: '',
  updatedAt: '',
  editPlanRevision: 0,
  renderedPlanRevision: 0,
}
let ready: boolean
function Form({ stored = project }: { stored?: ClipProject }) {
  const [part, select] = useState<'sources' | 'story' | 'style' | 'review'>('story')
  return (
    <ClipProjectForm
      ownerId="alice"
      stored={stored}
      dubbing={(value, change) => (
        <label>
          <input
            type="checkbox"
            checked={value.enabled}
            onChange={(e) => change({ ...value, enabled: e.target.checked })}
          />
          더빙 선택
        </label>
      )}
      actions={(value) => {
        ready = value
        return null
      }}
      layout={(content) => (
        <>
          {(['sources', 'story', 'style'] as const).map((screen) => (
            <div key={screen} hidden={part !== screen}>
              {content.screens[screen]}
            </div>
          ))}
          <div hidden={part !== 'review'}>
            {content.rows.map((row) => (
              <section aria-label={row.label} key={row.id}>
                {row.value}
              </section>
            ))}
          </div>
          <button type="button" onClick={() => select('review')}>
            검토
          </button>
          <button type="button" onClick={() => select('story')}>
            이야기
          </button>
          <button type="button" onClick={() => select('style')}>
            스타일
          </button>
        </>
      )}
    />
  )
}
// The component's transport, router and library reads are explicit injected mocks; no app route tree.
beforeEach(() => {
  ports.save.mockImplementation(async ({ draft }: { draft: ClipProjectDraft }) => ({
    ...project,
    ...draft,
  }))
  ports.voices = []
  ports.voicePending = false
  ports.voiceFailed = false
  ports.templatePending = false
  ports.templateFailed = false
})
afterEach(() => {
  discardClipDraftQueues()
  vi.clearAllMocks()
})

it('summarizes queued local values before a delayed save and retains them after rejection', async () => {
  let reject!: (error: Error) => void
  ports.save.mockImplementation(
    () =>
      new Promise((_, no) => {
        reject = no
      }),
  )
  render(<Form />)
  const title = screen.getByRole('textbox', { name: '클립 제목' })
  fireEvent.compositionStart(title)
  fireEvent.input(title, { target: { value: '저장 대기 한글' }, isComposing: true })
  fireEvent.change(screen.getByRole('textbox', { name: /클립에 담고 싶은 내용/ }), {
    target: { value: '정확한 원문 🥗\n두 번째 줄' },
  })
  let flushing!: Promise<void>
  act(() => {
    flushing = flushClipDraft('one', true).catch(() => undefined)
  })
  await act(async () => {
    await Promise.resolve()
  })
  await userEvent.click(screen.getByRole('button', { name: '검토' }))
  expect(
    within(screen.getByRole('region', { name: '클립 제목' })).getByText('저장 대기 한글'),
  ).toBeVisible()
  expect(screen.getByRole('region', { name: '클립에 담고 싶은 내용' })).toHaveTextContent(
    '정확한 원문 🥗 두 번째 줄',
  )
  await act(async () => {
    reject(new Error('save failed'))
    await flushing
  })
  await userEvent.click(screen.getByRole('button', { name: '이야기' }))
  expect(screen.getByRole('textbox', { name: '클립 제목' })).toBe(title)
  expect(title).toHaveValue('저장 대기 한글')
})

it('starts from a queued draft and refuses invalid local input without clearing it', async () => {
  queueClipDraft('one', { ...project, title: '큐의 제목' }, async () => {})
  render(<Form />)
  expect(screen.getByRole('textbox', { name: '클립 제목' })).toHaveValue('큐의 제목')
  fireEvent.change(screen.getByRole('textbox', { name: '클립 제목' }), { target: { value: '' } })
  expect(ready).toBe(false)
  await userEvent.click(screen.getByRole('button', { name: '검토' }))
  expect(screen.getByText('제목을 입력해 주세요.')).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: '이야기' }))
  expect(screen.getByRole('textbox', { name: '클립 제목' })).toHaveValue('')
})

it('keeps the retained disclosure control on style and summarizes its latest local visibility', async () => {
  render(<Form stored={{ ...project, composition: undefined, disclosure: 'ad' }} />)
  expect(screen.queryByRole('checkbox', { name: '영상에 광고·협찬 표시' })).toBeNull()
  await userEvent.click(screen.getByRole('button', { name: '스타일' }))
  const checkbox = screen.getByRole('checkbox', { name: '영상에 광고·협찬 표시' })
  expect(checkbox).toBeChecked()
  await userEvent.click(checkbox)
  await userEvent.click(screen.getByRole('button', { name: '검토' }))
  expect(screen.getByRole('region', { name: '디자인과 자막 스타일' })).toHaveTextContent(
    '영상에 광고·협찬 표시 안 함',
  )
})

it.each(['naver_clip', 'instagram_clip'] as const)(
  'reviews current %s values without a second editor',
  async (channel) => {
    render(
      <Form
        stored={{
          ...project,
          channel,
          videoTemplateId: 'template',
          campaign: true,
          advertiserGuide: { text: '광고주 원문', keywords: ['키워드'], hashtags: ['해시'] },
          linkedPost: { title: '내 글', slug: 'post', updatedAt: '' },
        }}
      />,
    )
    fireEvent.change(screen.getByLabelText('장소'), { target: { value: '내가 입력한 장소' } })
    await userEvent.click(screen.getByRole('button', { name: '스타일' }))
    fireEvent.change(screen.getByRole('spinbutton', { name: '목표 길이 (초)' }), {
      target: { value: '42' },
    })
    await userEvent.click(screen.getByRole('button', { name: '검토' }))
    expect(screen.queryByRole('textbox', { name: '장소' })).toBeNull()
    expect(
      within(screen.getByRole('region', { name: '템플릿 답변' })).getByText('내가 입력한 장소'),
    ).toBeVisible()
    expect(screen.getByText('목표 42초')).toBeVisible()
    expect(
      within(screen.getByRole('region', { name: '영상 템플릿' })).getByText('선택한 구성'),
    ).toBeVisible()
    expect(screen.getByText('내 글')).toBeVisible()
    expect(screen.getByRole('region', { name: '체험단 글' })).toHaveTextContent('광고주 원문')
    expect(screen.getByRole('region', { name: '체험단 글' })).toHaveTextContent(
      '필수 키워드: 키워드',
    )
    expect(screen.getByRole('region', { name: '화면 비율' })).toHaveTextContent('세로 9:16')
  },
)

it.each(['missing', 'loading', 'failed', 'confirmed'] as const)(
  'states %s voice truthfully and keeps original sound independent',
  async (state) => {
    ports.voicePending = state === 'loading'
    ports.voiceFailed = state === 'failed'
    ports.voices =
      state === 'confirmed' ? [{ id: 'voice', name: '차분한 목소리', removedAt: '' }] : []
    render(<Form stored={{ ...project, dubbing: { enabled: true, voiceId: 'voice' } }} />)
    await userEvent.click(screen.getByRole('button', { name: '검토' }))
    const narration = screen.getByRole('region', { name: '더빙 사용' })
    expect(ready).toBe(state === 'confirmed')
    expect(narration).toHaveTextContent(
      state === 'confirmed'
        ? '확정 목소리: 차분한 목소리'
        : state === 'loading'
          ? '확정 목소리를 확인하는 중'
          : state === 'failed'
            ? '불러오지 못했어요'
            : '사용할 수 있는 확정 목소리',
    )
    await userEvent.click(screen.getByRole('button', { name: '스타일' }))
    await userEvent.click(screen.getByRole('checkbox', { name: '더빙 선택' }))
    await userEvent.click(screen.getByRole('button', { name: '검토' }))
    expect(narration).toHaveTextContent('더빙 사용 안 함 · 원본 소리는 소스별 선택을 따라요.')
  },
)
