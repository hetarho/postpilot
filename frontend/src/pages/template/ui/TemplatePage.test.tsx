import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoConfigurationKind, ProtoAuthoringDraftState, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { openTemplateDirect, publishTemplateConversation } from '@/test/authoring-ui'
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
  await screen.findByRole('log', { name: '템플릿 대화' })
  await openTemplateDirect(user)
  return view
}
// Templates are made by talking to the AI (owner 2026-11-17): new or saved, the screen opens on
// one conversation beside the draft's live preview; the text is edited by hand behind ⋯.
describe('templates made by talking', () => {
  it('opens a saved template straight into its conversation, the draft beside it, with no paid work', async () => {
    const calls: string[] = []
    mount(undefined, {}, {}, calls)
    const log = await screen.findByRole('log', { name: '템플릿 대화' })
    expect(within(log).getByText('“식당 리뷰” 템플릿이에요. 무엇을 바꿀까요?')).toBeInTheDocument()
    const draft = screen.getByRole('complementary', { name: '초안' })
    expect(await within(draft).findByText('메뉴 소개')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: '바꾸고 싶은 점' })).toBeInTheDocument()
    // Nothing changed yet, so there is nothing to save.
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(calls.filter((name) => /Start|EstimateAuthoring|SaveAuthoring/.test(name))).toEqual([])
  })
  it('opens a new template on the question of what the owner posts, with no wizard and no paid work', async () => {
    const calls: string[] = []
    mount('/templates/new', { templates: [] }, {}, calls)
    const log = await screen.findByRole('log', { name: '템플릿 대화' })
    expect(within(log).getByText(/어떤 글을 올리세요\?/)).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: '무엇을 올리는지' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '초안 만들기' })).toBeDisabled()
    expect(screen.getByText('초안이 여기에 그려져요')).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
    expect(calls.filter((call) => /Start|EstimateAuthoring|SaveAuthoring/.test(call))).toEqual([])
  })
  it('offers the kinds of post the owner makes as ways to begin, each filling the field and sending nothing', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    mount('/templates/new', { templates: [] }, {}, calls)
    const log = await screen.findByRole('log', { name: '템플릿 대화' })
    for (const name of ['맛집 후기', '조립하는 상품', '음식 상품', '보여 주는 상품'])
      expect(within(log).getByRole('button', { name })).toBeInTheDocument()
    await user.click(within(log).getByRole('button', { name: '조립하는 상품' }))
    expect(screen.getByRole('textbox', { name: '무엇을 올리는지' })).toHaveValue(
      '조립이 필요한 상품 리뷰예요. 구성품, 조립 과정, 완성 모습과 써 본 느낌을 순서대로 보여 줘요.',
    )
    // Once the owner has words in the field, the ways to begin step aside.
    expect(within(log).queryByRole('button', { name: '맛집 후기' })).not.toBeInTheDocument()
    expect(calls.filter((call) => /Start|EstimateAuthoring|SaveAuthoring/.test(call))).toEqual([])
  })
  it('asks for one draft only after the owner describes what they post, and quotes it first', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/templates/new', {
      user: { id: 'alice' },
      calls,
      templates: { templates: [] },
      providers: {
        models: [{ providerId: 'stub', modelId: 'writer', label: '작가AI', stages: [Stage.WRITE] }],
        selections: [{ stage: Stage.WRITE, providerId: 'stub', modelId: 'writer' }],
      },
    })
    const field = await screen.findByRole('textbox', { name: '무엇을 올리는지' })
    await user.type(field, '동네 카페 방문기를 사진 위주로 올려요')
    const send = screen.getByRole('button', { name: '초안 만들기' })
    await waitFor(() => expect(send).toBeEnabled())
    await user.click(send)
    // The estimate comes first; nothing starts until the owner confirms it.
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByRole('heading', { name: '이 설명으로 초안을 만들까요?' }),
    ).toBeInTheDocument()
    expect(calls).toContain('EstimateAuthoringOperation')
    expect(calls.filter((call) => /StartAuthoring/.test(call))).toEqual([])
  })
  it('preserves an untouched legacy body and all owner numbers in one versioned publication', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    const calls: string[] = []
    await direct(user, undefined, { updates }, {}, calls)
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('1800')
    await user.type(screen.getByLabelText('이름'), ' 2편')
    expect(updates).toHaveLength(0)
    await publishTemplateConversation(user)
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
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(patches[0]).toMatchObject({
      body: '<write>unfinished',
      titleArea: '<slot kind="photo"/>',
    })
  })
  it('reopens a saved template on its unsaved conversation instead of a fresh copy', async () => {
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
    await screen.findByRole('log', { name: '템플릿 대화' })
    expect(
      await screen.findByRole('heading', { level: 2, name: '편집 중 이름' }),
    ).toBeInTheDocument()
    await openTemplateDirect(user)
    expect(screen.getByLabelText('이름')).toHaveValue('편집 중 이름')
    expect(calls).toContain('GetAuthoringSession')
    expect(calls).not.toContain('CreateAuthoringSession')
  })
  it('resumes an unsaved creation session in its conversation', async () => {
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
    await screen.findByRole('log', { name: '템플릿 대화' })
    expect(
      await screen.findByRole('heading', { level: 2, name: '이어 만들기' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: '바꾸고 싶은 점' })).toBeInTheDocument()
    expect(calls).toContain('GetAuthoringSession')
    expect(calls.filter((call) => /Create|Start/.test(call))).toEqual([])
  })
  it('retains the draft on target conflict without publishing or starting AI', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    const calls: string[] = []
    await direct(user, undefined, { updates }, { saveConflict: true }, calls)
    await user.type(screen.getByLabelText('이름'), ' 새 이름')
    await publishTemplateConversation(user)
    expect(await screen.findByText(/다른 곳에서 바뀌었/)).toBeInTheDocument()
    expect(updates).toHaveLength(0)
    expect(calls).not.toContain('StartAuthoringOperation')
  })
})
