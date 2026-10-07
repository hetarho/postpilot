import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoConfigurationKind, ProtoAuthoringDraftState } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { publishAuthoringDraft } from '@/test/authoring-ui'
import type { FakeTemplatesOptions } from '@/test/templates'
import type { FakeAuthoringOptions } from '@/test/authoring'
const stored = {
  id: 'review',
  name: '식당 리뷰',
  description: '방문 구성',
  titleArea: '<write>메뉴를 한 줄로</write>',
  body: '<write>메뉴 소개</write>\n<slot kind="place" label="지도"/>',
  targetLength: 1800,
  tagCount: 7,
}
function mount(
  path = '/templates/review',
  templates: FakeTemplatesOptions = {},
  authoring: FakeAuthoringOptions = {},
  calls: string[] = [],
) {
  return renderAppAt(path, {
    user: { id: 'alice' },
    calls,
    templates: { templates: [stored], ...templates },
    authoring,
  })
}
async function direct(
  user: ReturnType<typeof userEvent.setup>,
  path = '/templates/review',
  templates: FakeTemplatesOptions = {},
  authoring: FakeAuthoringOptions = {},
  calls: string[] = [],
) {
  const view = mount(path, templates, authoring, calls)
  await user.click(
    await screen.findByRole('button', {
      name: path.endsWith('/new') ? '직접 편집' : /식당 리뷰.*직접 편집하기/,
    }),
  )
  await screen.findByLabelText('이름')
  return view
}
const add = (name: string) =>
  within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
    name: new RegExp('^' + name),
  })
describe('named template shared editing', () => {
  it('opens the saved usable template before a method and reads summaries without a session or paid work', async () => {
    const calls: string[] = []
    mount(undefined, {}, {}, calls)
    await screen.findByRole('heading', { level: 1, name: '식당 리뷰' })
    expect(await screen.findByText('메뉴 소개')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /식당 리뷰.*AI로 편집하기/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /식당 리뷰.*직접 편집하기/ })).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('ListAuthoringSummaries'))
    expect(
      calls.filter((name) =>
        /AuthoringSession|Start|EstimateAuthoring|SaveAuthoring|GetAuthoringSession/.test(name),
      ),
    ).toEqual([])
  })
  it('preserves an untouched legacy body and all owner numbers in one versioned publication', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    const calls: string[] = []
    await direct(user, undefined, { updates }, {}, calls)
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('1800')
    await user.type(screen.getByLabelText('이름'), ' 2편')
    expect(updates).toHaveLength(0)
    await publishAuthoringDraft(user)
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toMatchObject({
      id: 'review',
      name: '식당 리뷰 2편',
      body: stored.body,
      titleArea: stored.titleArea,
      targetLength: 1800,
      tagCount: 7,
    })
    expect(calls).not.toContain('StartAuthoringOperation')
    expect(calls).not.toContain('StartTemplateRequest')
  })
  it('retains incomplete builder rows across source, AI and direct modes and durable recovery', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    await direct(user, undefined, {}, { patches })
    await user.click(add('고정 문구'))
    await user.click(screen.getByRole('tab', { name: '원문' }))
    expect(screen.getByLabelText('원문')).toHaveValue(
      '<write>메뉴 소개</write>\n지도\n<write></write>',
    )
    await user.click(screen.getByRole('tab', { name: '블록' }))
    expect(screen.getByText('들어갈 문구를 적어 주세요')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /식당 리뷰.*AI로 편집하기/ }))
    await screen.findByRole('heading', { name: '어떤 점을 바꿔 볼까요?' })
    expect(patches).toHaveLength(1)
    expect(patches[0].builderState).toContain('"kind":"text"')
    expect(patches[0].body).toContain('<write></write>')
    await user.click(screen.getByRole('button', { name: /식당 리뷰.*직접 편집하기/ }))
    expect(await screen.findByText('들어갈 문구를 적어 주세요')).toBeInTheDocument()
    const emptyRow = screen
      .getAllByRole('button')
      .find((button) => button.textContent?.includes('들어갈 문구를 적어 주세요'))!
    await user.click(emptyRow)
    await user.type(screen.getByLabelText('들어갈 문구'), '마무리')
    await publishAuthoringDraft(user)
    await screen.findByRole('heading', { level: 1, name: '식당 리뷰' })
  })
  it('keeps invalid raw title/body unchanged and publication unavailable', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    await direct(user, undefined, {}, { patches })
    await user.click(screen.getByRole('tab', { name: '원문' }))
    fireEvent.change(screen.getByLabelText('원문'), { target: { value: '<write>unfinished' } })
    fireEvent.change(screen.getByLabelText('제목 원문'), {
      target: { value: '<slot kind="photo"/>' },
    })
    await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
    expect(await screen.findByText(/마지막 유효한 미리보기/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '저장할 내용 확인하기' })).not.toBeInTheDocument()
    expect(patches[0]).toMatchObject({
      body: '<write>unfinished',
      titleArea: '<slot kind="photo"/>',
    })
  })
  it('shows named unfinished work separately and resume retains it while a fresh method captures saved content', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const draft = {
      id: 'draft',
      name: '편집 중 이름',
      description: '',
      body: '<write>다른 구성</write>',
      titleArea: '',
    }
    mount(
      undefined,
      {},
      {
        sessions: [
          {
            id: 'work',
            kind: ProtoConfigurationKind.POST_TEMPLATE,
            targetId: 'review',
            revision: 3,
            phase: 'editing',
            workingSource: draft,
            selected: draft,
            savedBaseline: { ...draft, name: stored.name, body: stored.body },
            savedAvailable: true,
            hasUnpublishedChanges: true,
            draftState: ProtoAuthoringDraftState.VALID,
          },
        ],
      },
      calls,
    )
    await user.click(await screen.findByRole('button', { name: /식당 리뷰.*이어서 편집하기/ }))
    await user.click(await screen.findByRole('button', { name: /식당 리뷰.*직접 편집하기/ }))
    expect(await screen.findByLabelText('이름')).toHaveValue('편집 중 이름')
    expect(calls).toContain('GetAuthoringSession')
    expect(calls).not.toContain('CreateAuthoringSession')
  })
  it('starts seed-free manual creation and publishes only after explicit named confirmation', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeTemplatesOptions['creates']> = []
    const { router } = await direct(user, '/templates/new', { templates: [], creates })
    await user.type(screen.getByLabelText('이름'), '내 새 구성')
    await user.click(add('AI가 쓰는 글'))
    await user.type(screen.getByLabelText('이 자리에 오는 것'), '오늘의 이야기')
    expect(creates).toHaveLength(0)
    await publishAuthoringDraft(user)
    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/templates\/template-/))
    expect(await screen.findByText('“내 새 구성” 글 구성을 새로 저장했어요.')).toBeInTheDocument()
    expect(creates[0]).toMatchObject({ name: '내 새 구성', body: '<write>오늘의 이야기</write>' })
  })
  it('retains the draft on target conflict without publishing or starting AI', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    const calls: string[] = []
    await direct(user, undefined, { updates }, { saveConflict: true }, calls)
    await user.type(screen.getByLabelText('이름'), ' 새 이름')
    await publishAuthoringDraft(user)
    expect(await screen.findByText(/다른 곳에서 바뀌었/)).toBeInTheDocument()
    expect(updates).toHaveLength(0)
    expect(calls).not.toContain('StartAuthoringOperation')
  })
})
