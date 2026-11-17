import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
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
      name: '직접 고치기',
    }),
  )
  await screen.findByLabelText('이름')
  return view
}
describe('named template shared editing', () => {
  it('opens the saved usable template before a method and reads summaries without a session or paid work', async () => {
    const calls: string[] = []
    mount(undefined, {}, {}, calls)
    await screen.findByRole('heading', { level: 1, name: '식당 리뷰' })
    expect(await screen.findByText('메뉴 소개')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'AI와 대화로 고치기' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '직접 고치기' })).toBeInTheDocument()
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
  // Guide: by hand the owner changes what a row says, adds or removes one; the ORDER is changed
  // by asking the AI. No row can be dragged and no row has move buttons.
  it('edits a saved template by hand with nothing to drag or reorder', async () => {
    const user = userEvent.setup()
    await direct(user)
    expect(screen.getByRole('group', { name: '블록 추가' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '위로' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '아래로' })).not.toBeInTheDocument()
    expect(document.querySelector('[draggable="true"]')).toBeNull()
  })
  it('keeps invalid raw title/body unchanged and publication unavailable', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    await direct(user, undefined, {}, { patches })
    await user.click(screen.getByRole('button', { name: '고급' }))
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
    await user.click(await screen.findByRole('button', { name: '하던 대화 이어 가기' }))
    await user.click(await screen.findByRole('button', { name: '직접 고치기' }))
    expect(await screen.findByLabelText('이름')).toHaveValue('편집 중 이름')
    expect(calls).toContain('GetAuthoringSession')
    expect(calls).not.toContain('CreateAuthoringSession')
  })
  // Guide: a new template is made by talking to the AI, from scratch — no wizard. The page opens
  // on the AI's question with ready-made openings that fill the field, and sends nothing itself.
  it('opens a new template on the conversation, not a wizard', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    mount('/templates/new', { templates: [] }, {}, calls)
    expect(await screen.findByText(/어떤 글을 올리세요\?/)).toBeInTheDocument()
    const field = screen.getByRole('textbox', { name: '어떤 글을 올리는지 말해 주세요' })
    await user.click(screen.getByRole('button', { name: /조립이 필요한 가구를 리뷰해요/ }))
    expect(field).toHaveValue(
      '조립이 필요한 가구를 리뷰해요. 구성품, 조립 과정, 완성 후 사용감 순서로요',
    )
    // The preview waits for a draft; there is nothing to save yet.
    expect(screen.getByText('대화를 시작하면 여기에 템플릿이 그려져요')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '저장' })).not.toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
    expect(calls.filter((call) => /Create|StartAuthoring/.test(call))).toEqual([])
  })
  it('resumes an unsaved creation session', async () => {
    const calls: string[] = []
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
            workingSource: {
              id: 'draft',
              name: '이어 만들기',
              description: '',
              titleArea: '',
              body: '<write>방문</write>',
            },
            selected: {
              id: 'draft',
              name: '이어 만들기',
              description: '',
              titleArea: '',
              body: '<write>방문</write>',
            },
          },
        ],
      },
      calls,
    )
    await screen.findByRole('textbox', { name: '무엇을 바꿀지 말해 주세요' })
    expect(calls).toContain('GetAuthoringSession')
    expect(calls.filter((call) => /Create|Start/.test(call))).toEqual([])
    expect(
      screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
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
