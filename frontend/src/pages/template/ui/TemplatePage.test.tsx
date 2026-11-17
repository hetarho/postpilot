import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoConfigurationKind, ProtoAuthoringDraftState } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { openConversationDirect, saveConversationDraft } from '@/test/authoring-ui'
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
  await openConversationDirect(user)
  await screen.findByLabelText('이름')
  return view
}
describe('a template, changed by talking', () => {
  it('opens a saved template on itself with the conversation beside it, without a session or paid work', async () => {
    const calls: string[] = []
    mount(undefined, {}, {}, calls)
    await screen.findByRole('heading', { level: 1, name: '식당 리뷰' })
    expect(await screen.findByText('메뉴 소개')).toBeInTheDocument()
    const log = screen.getByRole('log')
    expect(within(log).getByText(/“식당 리뷰”을 어떻게 바꿀까요\?/)).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'AI에게 보낼 말' })).toBeInTheDocument()
    // Deleting lives on the template's own screen now, at the end of its title row.
    expect(screen.getByRole('button', { name: /식당 리뷰.*삭제|삭제/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '변경 저장' })).toBeDisabled()
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
    await saveConversationDraft(user)
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
  it('edits the template as text only: no block builder, nothing to drag or reorder', async () => {
    const user = userEvent.setup()
    await direct(user)
    expect(screen.getByLabelText('원문')).toHaveValue(stored.body)
    expect(screen.getByLabelText('제목 원문')).toHaveValue(stored.titleArea)
    expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '고급' })).not.toBeInTheDocument()
    expect(screen.queryAllByRole('button', { name: /위로|아래로|끌어서|옮기기/ })).toEqual([])
  })
  it('keeps invalid raw title/body unchanged and publication unavailable', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    await direct(user, undefined, {}, { patches })
    fireEvent.change(screen.getByLabelText('원문'), { target: { value: '<write>unfinished' } })
    fireEvent.change(screen.getByLabelText('제목 원문'), {
      target: { value: '<slot kind="photo"/>' },
    })
    await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
    expect(await screen.findByText(/마지막 유효한 미리보기/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '변경 저장' })).toBeDisabled()
    expect(patches[0]).toMatchObject({
      body: '<write>unfinished',
      titleArea: '<slot kind="photo"/>',
    })
  })
  it('reopens unsaved conversation work on a saved template without creating a session', async () => {
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
    expect(await screen.findByText('저장하지 않은 초안')).toBeInTheDocument()
    await openConversationDirect(user)
    expect(await screen.findByLabelText('이름')).toHaveValue('편집 중 이름')
    expect(calls).toContain('GetAuthoringSession')
    expect(calls).not.toContain('CreateAuthoringSession')
  })
  it('opens a new template as a conversation — no wizard — and creates nothing until asked', async () => {
    const calls: string[] = []
    mount('/templates/new', { templates: [] }, {}, calls)
    const log = await screen.findByRole('log')
    expect(within(log).getByText(/어떤 글을 올리세요\?/)).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'AI에게 보낼 말' })).toBeInTheDocument()
    expect(screen.getByText('아직 초안이 없어요')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(
      screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
    expect(calls.filter((call) => /Create|StartAuthoring/.test(call))).toEqual([])
  })
  it('resumes an unsaved creation session in the conversation', async () => {
    const calls: string[] = []
    const draft = {
      id: 'draft',
      name: '이어 만들기',
      description: '',
      titleArea: '',
      body: '<write>방문</write>',
    }
    mount(
      '/templates/new?session=unfinished',
      { templates: [] },
      {
        sessions: [
          {
            id: 'unfinished',
            kind: ProtoConfigurationKind.POST_TEMPLATE,
            revision: 3,
            phase: 'editing',
            hasUnpublishedChanges: true,
            draftState: ProtoAuthoringDraftState.VALID,
            workingSource: draft,
            selected: draft,
          },
        ],
      },
      calls,
    )
    expect(await screen.findByText(/지금 초안은 “이어 만들기”이에요/)).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'AI에게 보낼 말' })).toBeInTheDocument()
    expect(calls).toContain('GetAuthoringSession')
    expect(calls.filter((call) => /Create|Start/.test(call))).toEqual([])
  })
  it('retains the draft on target conflict without publishing or starting AI', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    const calls: string[] = []
    await direct(user, undefined, { updates }, { saveConflict: true }, calls)
    await user.type(screen.getByLabelText('이름'), ' 새 이름')
    await saveConversationDraft(user)
    expect(await screen.findByText(/다른 곳에서 바뀌었/)).toBeInTheDocument()
    expect(updates).toHaveLength(0)
    expect(calls).not.toContain('StartAuthoringOperation')
  })
})
