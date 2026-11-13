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
        introPreset: '',
        outroPreset: '',
        allowedCaptionStyles: [],
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
const refusal = () => document.querySelector('[data-refusal]')?.getAttribute('data-refusal')
const SCREENS = ['kind', 'requirements', 'sources', 'template', 'content', 'style', 'note'] as const
type Screen = (typeof SCREENS)[number] | 'review'
const NAMES: Record<Screen, string> = {
  kind: '종류로',
  requirements: '요청으로',
  sources: '영상으로',
  template: '템플릿으로',
  content: '내용으로',
  style: '스타일로',
  note: '덧붙임으로',
  review: '검토',
}
function Form({
  stored = project,
  start = 'content',
  unminted = false,
  onMinted,
}: {
  stored?: ClipProject
  start?: Screen
  /** `/clips/new`: no project yet, minted by the first save (CLIP-130). */
  unminted?: boolean
  onMinted?: (id: string) => void
}) {
  const [part, select] = useState<Screen>(start)
  const [mintedId, setMintedId] = useState('')
  return (
    <ClipProjectForm
      ownerId="alice"
      stored={unminted ? undefined : stored}
      onMinted={onMinted}
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
      layout={(content) => {
        return (
          <>
            <p data-refusal={content.inputRefusal ?? ''} />
            {SCREENS.map((screen) => (
              <div key={screen} hidden={part !== screen}>
                {content.screens[screen]}
              </div>
            ))}
            <div hidden={part !== 'review'}>
              {content.rows.map((row) => (
                <section aria-label={row.label} key={row.id} data-screen={row.screen}>
                  {row.value}
                </section>
              ))}
            </div>
            {(Object.keys(NAMES) as Screen[]).map((screen) => (
              <button key={screen} type="button" onClick={() => select(screen)}>
                {NAMES[screen]}
              </button>
            ))}
            <button
              type="button"
              onClick={() => void content.mint().then(setMintedId, () => setMintedId('refused'))}
            >
              발급
            </button>
            <output aria-label="발급된 영상">{mintedId}</output>
          </>
        )
      }}
    />
  )
}
const open = (screen: Screen) =>
  userEvent.click(screen === 'review' ? byName('검토') : byName(NAMES[screen]))
const byName = (name: string | RegExp) => screen.getByRole('button', { name })
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
  const title = screen.getByRole('textbox', { name: /클립 제목/ })
  fireEvent.compositionStart(title)
  fireEvent.input(title, { target: { value: '저장 대기 한글' }, isComposing: true })
  await open('note')
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
  await open('review')
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
  await open('content')
  expect(screen.getByRole('textbox', { name: /클립 제목/ })).toBe(title)
  expect(title).toHaveValue('저장 대기 한글')
})

it('starts from a queued draft, keeps an empty title optional and refuses only an overlong one', async () => {
  queueClipDraft('one', { ...project, title: '큐의 제목' }, async () => {})
  render(<Form />)
  const title = screen.getByRole('textbox', { name: /클립 제목/ })
  expect(title).toHaveValue('큐의 제목')
  expect(title).toHaveAttribute('placeholder', '예: 성수동 감성 카페 브이로그')
  expect(title).toHaveAccessibleDescription(/비워 두어도 괜찮아요/)
  fireEvent.change(title, { target: { value: '' } })
  expect(ready).toBe(true)
  expect(title).toHaveAttribute('aria-invalid', 'false')
  await open('review')
  expect(screen.getByText('제목 없는 영상 (비워 두어도 괜찮아요)')).toBeVisible()
  await open('content')
  fireEvent.change(title, { target: { value: 'x'.repeat(101) } })
  expect(ready).toBe(false)
  expect(title).toHaveAttribute('aria-invalid', 'true')
})

it('saves 영상 종류 as two explicit choices, each tap only marking its card', async () => {
  render(<Form stored={{ ...project, campaign: false }} start="kind" />)
  const sponsored = byName(/체험단으로 다녀왔어요/)
  const paid = byName(/내 돈으로 다녀왔어요/)
  expect(paid).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(sponsored)
  expect(sponsored).toHaveAttribute('aria-pressed', 'true')
  await act(async () => {
    await flushClipDraft('one', true)
  })
  expect(ports.save).toHaveBeenLastCalledWith(
    expect.objectContaining({ draft: expect.objectContaining({ campaign: true }) }),
  )
  await open('review')
  expect(screen.getByRole('region', { name: '체험단 글' })).toHaveTextContent(
    '체험단으로 다녀왔어요',
  )
  expect(screen.getByRole('region', { name: '광고주 가이드' })).toBeVisible()
})

it('offers 없음 beside every template and reads the choice back', async () => {
  render(<Form start="template" />)
  const none = byName(/^없음/)
  const chosen = byName(/선택한 구성/)
  expect(none).toHaveAttribute('aria-pressed', 'true')
  expect(within(chosen).getByText('구성 예시')).toBeInTheDocument()
  await userEvent.click(chosen)
  expect(chosen).toHaveAttribute('aria-pressed', 'true')
  expect(none).toHaveAttribute('aria-pressed', 'false')
  await open('review')
  expect(screen.getByRole('region', { name: '영상 템플릿' })).toHaveTextContent('선택한 구성')
  await open('template')
  await userEvent.click(none)
  await open('review')
  expect(screen.getByRole('region', { name: '영상 템플릿' })).toHaveTextContent('템플릿 없음')
})

it('routes a missing required answer to 내용, where the answer is marked invalid', async () => {
  const body = '<clip version="1"><field id="place" label="장소" required="true" /></clip>'
  render(
    <Form
      stored={{
        ...project,
        videoTemplateId: 'template',
        composition: {
          snapshot: { version: 1, body, templateId: 'template' },
          inputs: { values: {}, items: {}, associations: [] },
        },
      }}
    />,
  )
  expect(refusal()).toBe('content')
  expect(screen.getByLabelText('장소')).toHaveAttribute('aria-invalid', 'true')
  fireEvent.change(screen.getByLabelText('장소'), { target: { value: '성수' } })
  expect(refusal()).toBe('')
})

it('lists the rows in CLIP-227 order, each routed to its screen', async () => {
  render(
    <Form
      stored={{
        ...project,
        campaign: true,
        advertiserGuide: { text: '가이드', keywords: [], hashtags: [] },
      }}
    />,
  )
  await open('review')
  expect(
    Array.from(document.querySelectorAll('section[data-screen]')).map(
      (row) => `${row.getAttribute('aria-label')}:${row.getAttribute('data-screen')}`,
    ),
  ).toEqual([
    '체험단 글:kind',
    '광고주 가이드:requirements',
    '글 연결:sources',
    '영상 템플릿:template',
    '클립 제목:content',
    '템플릿 답변:content',
    '목표 길이 (초):style',
    '화면 비율:style',
    '디자인과 자막 스타일:style',
    '더빙 사용:style',
    '클립에 담고 싶은 내용:note',
  ])
})

it('keeps the retained disclosure control on style and summarizes its latest local visibility', async () => {
  render(<Form stored={{ ...project, composition: undefined, disclosure: 'ad' }} />)
  expect(screen.queryByRole('checkbox', { name: '영상에 광고·협찬 표시' })).toBeNull()
  await open('style')
  const checkbox = screen.getByRole('checkbox', { name: '영상에 광고·협찬 표시' })
  expect(checkbox).toBeChecked()
  await userEvent.click(checkbox)
  await open('review')
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
    await open('style')
    fireEvent.change(screen.getByRole('spinbutton', { name: '목표 길이 (초)' }), {
      target: { value: '42' },
    })
    await open('review')
    expect(screen.queryByRole('textbox', { name: '장소' })).toBeNull()
    expect(
      within(screen.getByRole('region', { name: '템플릿 답변' })).getByText('내가 입력한 장소'),
    ).toBeVisible()
    expect(screen.getByText('목표 42초')).toBeVisible()
    expect(
      within(screen.getByRole('region', { name: '영상 템플릿' })).getByText('선택한 구성'),
    ).toBeVisible()
    expect(screen.getByText('내 글')).toBeVisible()
    expect(screen.getByRole('region', { name: '광고주 가이드' })).toHaveTextContent('광고주 원문')
    expect(screen.getByRole('region', { name: '광고주 가이드' })).toHaveTextContent(
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
    await open('review')
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
    await open('style')
    await userEvent.click(screen.getByRole('checkbox', { name: '더빙 선택' }))
    await open('review')
    expect(narration).toHaveTextContent('더빙 사용 안 함 · 원본 소리는 소스별 선택을 따라요.')
  },
)

it('mints an unminted clip with the whole queued draft once and updates it afterwards (CLIP-130)', async () => {
  const user = userEvent.setup()
  const minted = vi.fn()
  ports.save.mockImplementation(
    async ({ id, draft }: { id?: string; draft: ClipProjectDraft }) => ({
      ...project,
      ...draft,
      id: id ?? 'minted',
    }),
  )
  render(<Form unminted start="kind" onMinted={minted} />)
  // The guide stands on /clips/new too, with nothing that creates the clip.
  expect(screen.queryByRole('button', { name: '클립 만들기' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /내 돈으로 다녀왔어요/ }))
  await open('content')
  // An empty title saves: nothing in the draft is required to mint it.
  await user.click(byName('발급'))
  expect(await screen.findByRole('status', { name: '발급된 영상' })).toHaveTextContent('minted')
  expect(ports.save).toHaveBeenCalledTimes(1)
  expect(ports.save).toHaveBeenCalledWith({
    draft: expect.objectContaining({ title: '', campaign: false, channel: 'naver_clip' }),
  })
  expect(minted).toHaveBeenCalledWith('minted')
  await user.type(screen.getByRole('textbox', { name: /클립 제목/ }), '라떼')
  await act(() => flushClipDraft('minted'))
  expect(ports.save).toHaveBeenLastCalledWith({
    id: 'minted',
    draft: expect.objectContaining({ title: '라떼' }),
  })
  expect(ports.save.mock.calls.filter(([request]) => !request.id)).toHaveLength(1)
})

it('keeps every local field when the mint is refused (CLIP-130)', async () => {
  const user = userEvent.setup()
  const minted = vi.fn()
  ports.save.mockRejectedValue(new Error('refused'))
  render(<Form unminted onMinted={minted} />)
  await user.type(screen.getByRole('textbox', { name: /클립 제목/ }), '새 경험')
  await user.click(byName('발급'))
  expect(await screen.findByRole('status', { name: '발급된 영상' })).toHaveTextContent('refused')
  expect(minted).not.toHaveBeenCalled()
  expect(screen.getByRole('textbox', { name: /클립 제목/ })).toHaveValue('새 경험')
})
