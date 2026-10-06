import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoGuidelineKind, ProtoGuidelineScope } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeGuidelineRow, FakeGuidelinesOptions } from '@/test/guidelines'

const USER = { id: 'alice' }

const GUIDELINES: FakeGuidelineRow[] = [
  // A post's 지침 lives on /guidelines and never shows here (GUIDE-2).
  { id: 'post-guideline', text: '없는 사실을 쓰지 않기' },
  { id: 'clip-global', kind: 'clip', text: '자막에 가격을 적지 않기' },
  {
    id: 'clip-scoped',
    kind: 'clip',
    text: '첫 컷은 가게 외관으로',
    templateRefs: [{ id: 'video-template-store', name: '가게 소개' }],
  },
  // Its video template was deleted: 적용 대상 없음 (GUIDE-14).
  { id: 'clip-orphan', kind: 'clip', text: '음식은 가까이', scope: 'templates', templateRefs: [] },
]

function renderVideoGuidelines(guidelines: FakeGuidelinesOptions = {}, calls: string[] = []) {
  return renderAppAt('/video-guidelines', {
    user: USER,
    calls,
    clips: {
      templates: [
        { id: 'video-template-store', name: '가게 소개', compositionBody: '<clip version="1"/>' },
        { id: 'video-template-menu', name: '메뉴 소개', compositionBody: '<clip version="1"/>' },
      ],
    },
    guidelines: { guidelines: GUIDELINES, ...guidelines },
  })
}

const section = async (name: string) => within(await screen.findByRole('region', { name }))

async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '새 영상 지침' }))
  return within(await screen.findByRole('dialog'))
}

describe('/video-guidelines', () => {
  // GUIDE-44: the clip kind's own directory, in /guidelines' shape; reading it asks no model.
  it('lists the clip 기본 지침 and the owner’s 영상 지침 in one list, never a post’s', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderVideoGuidelines(
      {
        defaults: [{ key: 'post-default', name: '글 기본', text: '글에만' }],
        clipDefaults: [{ key: 'clip-default', name: '과장 금지', text: '자막을 과장하지 않기' }],
      },
      calls,
    )

    expect(await screen.findByRole('heading', { level: 1, name: '영상 지침' })).toBeInTheDocument()
    const list = await section('영상 지침 목록')
    const items = list.getAllByRole('listitem')
    // The clip 기본 지침 in use first, then the owner's, as one list (GUIDE-44).
    expect(items).toHaveLength(4)
    expect(within(items[0]).getByText('과장 금지')).toBeInTheDocument()
    expect(within(items[0]).getByText('추천')).toBeInTheDocument()
    expect(list.queryByText('글 기본')).not.toBeInTheDocument()
    expect(within(items[1]).getByText('자막에 가격을 적지 않기')).toBeInTheDocument()
    expect(within(items[1]).getByText('전역')).toBeInTheDocument()
    expect(within(items[2]).getByText('영상 템플릿')).toBeInTheDocument()
    expect(within(items[3]).getByText('적용 대상 없음')).toBeInTheDocument()
    await user.click(within(items[2]).getByRole('button', { expanded: false }))
    expect(within(items[2]).getByText('가게 소개')).toBeInTheDocument()
    expect(screen.queryByText('없는 사실을 쓰지 않기')).not.toBeInTheDocument()

    const allowed = [
      'InitializeDefaultSelections',
      'GetMe',
      'GetMyPlan',
      'ListGuidelines',
      'ListGuidelineCandidates',
    ]
    expect(calls.filter((call) => !allowed.includes(call))).toEqual([])
  })

  it('takes a clip 기본 지침 out of use for the clip kind', async () => {
    const user = userEvent.setup()
    const defaultSwitches: NonNullable<FakeGuidelinesOptions['defaultSwitches']> = []
    renderVideoGuidelines({
      clipDefaults: [{ key: 'clip-default', name: '과장 금지', text: '자막을 과장하지 않기' }],
      defaultSwitches,
    })

    const list = await section('영상 지침 목록')
    await user.click(list.getByRole('button', { name: /과장 금지/, expanded: false }))
    await user.click(list.getByRole('button', { name: '과장 금지 적용 안함' }))
    await waitFor(() => expect(defaultSwitches).toHaveLength(1))
    expect(defaultSwitches[0]).toEqual({
      kind: ProtoGuidelineKind.CLIP,
      key: 'clip-default',
      enabled: false,
    })
  })

  it('creates a 전역 영상 지침', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderVideoGuidelines({ guidelines: [], creates })
    const form = await openCreateSheet(user)

    // 전역 or 특정 영상 템플릿: a 영상 지침 has no 분야 (GUIDE-5).
    expect(form.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      '전역',
      '특정 영상 템플릿',
    ])
    await user.type(form.getByLabelText('지침'), '자막은 두 줄까지')
    await user.click(form.getByRole('button', { name: '지침 만들기' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      kind: 'clip',
      text: '자막은 두 줄까지',
      scope: ProtoGuidelineScope.GLOBAL,
      templateIds: [],
      fields: [],
    })
    const list = await section('영상 지침 목록')
    await waitFor(() => expect(list.getByText('자막은 두 줄까지')).toBeInTheDocument())
  })

  it('offers the clip 기본 지침 in its own sheet from the dock', async () => {
    const user = userEvent.setup()
    const defaultSwitches: NonNullable<FakeGuidelinesOptions['defaultSwitches']> = []
    renderVideoGuidelines({
      defaults: [{ key: 'post-default', name: '글 기본', text: '글에만' }],
      clipDefaults: [
        { key: 'clip-default', name: '과장 금지', text: '자막을 과장하지 않기', enabled: false },
      ],
      defaultSwitches,
    })

    await user.click(await screen.findByRole('button', { name: '기본 지침' }))
    const sheet = within(await screen.findByRole('dialog', { name: '기본 지침' }))
    expect(sheet.queryByText('글 기본')).not.toBeInTheDocument()
    await user.click(sheet.getByRole('button', { name: '과장 금지 추가' }))
    await waitFor(() =>
      expect(defaultSwitches).toEqual([
        { kind: ProtoGuidelineKind.CLIP, key: 'clip-default', enabled: true },
      ]),
    )
  })

  it('creates a 영상 지침 scoped to a video template', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderVideoGuidelines({ guidelines: [], creates })
    const form = await openCreateSheet(user)

    await user.type(form.getByLabelText('지침'), '메뉴판을 먼저')
    await user.click(form.getByRole('tab', { name: '특정 영상 템플릿' }))
    expect(form.getByRole('button', { name: '지침 만들기' })).toBeDisabled()
    await user.click(await form.findByLabelText('메뉴 소개'))
    await user.click(form.getByRole('button', { name: '지침 만들기' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      kind: 'clip',
      text: '메뉴판을 먼저',
      scope: ProtoGuidelineScope.TEMPLATES,
      templateIds: ['video-template-menu'],
      fields: [],
    })
  })
})

describe('the 영상 지침 후보 section', () => {
  const CANDIDATES: NonNullable<FakeGuidelinesOptions['candidates']> = [
    { id: 'clip-repeated', kind: 'clip', text: '자막 더 짧게', clipId: 'clip-1', occurrences: 3 },
    // The clip project was deleted: the text survives, the link does not.
    { id: 'clip-orphan', kind: 'clip', text: '음악은 조용하게' },
    // A post's candidate waits on /guidelines, not here.
    { id: 'post-candidate', text: '존댓말로 써줘', postSlug: 'post-1' },
  ]

  const candidateRow = async (text: string) => {
    const list = await section('후보 지침')
    const found = list
      .getAllByRole('listitem')
      .find((item) => within(item).queryByText(text) !== null)
    if (!found) throw new Error(`no candidate row with text ${text}`)
    return within(found)
  }

  it('counts only the clip candidates and links each to its clip, or says it is gone', async () => {
    renderVideoGuidelines({ candidates: CANDIDATES })

    expect(await screen.findByText('영상 지침 후보 2개')).toBeInTheDocument()
    const list = await section('후보 지침')
    expect(list.getAllByRole('listitem')).toHaveLength(2)
    expect(list.queryByText('존댓말로 써줘')).not.toBeInTheDocument()
    const repeated = await candidateRow('자막 더 짧게')
    expect(repeated.getByText('3번 요청함')).toBeInTheDocument()
    expect(repeated.getByRole('link', { name: '요청한 영상 보기' })).toHaveAttribute(
      'href',
      '/clips/clip-1',
    )
    const orphan = await candidateRow('음악은 조용하게')
    expect(orphan.queryByRole('link')).not.toBeInTheDocument()
    expect(orphan.getByText('요청한 영상이 삭제됐어요')).toBeInTheDocument()
  })

  it('approves a candidate as a 영상 지침 with the video-template scope control', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderVideoGuidelines({ candidates: CANDIDATES, creates })

    await user.click((await candidateRow('자막 더 짧게')).getByRole('button', { name: '승인' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      '전역',
      '특정 영상 템플릿',
    ])
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toMatchObject({
      kind: 'clip',
      text: '자막 더 짧게',
      scope: ProtoGuidelineScope.GLOBAL,
      fromCandidateId: 'clip-repeated',
    })
    const saved = await section('영상 지침 목록')
    await waitFor(() => expect(saved.getByText('자막 더 짧게')).toBeInTheDocument())
    await waitFor(async () =>
      expect((await section('후보 지침')).getAllByRole('listitem')).toHaveLength(1),
    )
  })

  it('dismisses one candidate, and accepts or dismisses the whole clip queue', async () => {
    const user = userEvent.setup()
    const dismissals: string[] = []
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderVideoGuidelines({ candidates: CANDIDATES, dismissals, creates })

    await user.click((await candidateRow('음악은 조용하게')).getByRole('button', { name: '무시' }))
    await waitFor(() => expect(dismissals).toEqual(['clip-orphan']))

    await user.click(await screen.findByRole('button', { name: '전부 수락' }))
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toMatchObject({ kind: 'clip', fromCandidateId: 'clip-repeated' })
    expect(
      await screen.findByText('1개를 지침으로 저장했어요. 0개는 그대로 남았어요.'),
    ).toBeInTheDocument()
  })

  it('dismisses the whole clip queue after asking once', async () => {
    const user = userEvent.setup()
    const dismissals: string[] = []
    renderVideoGuidelines({ candidates: CANDIDATES, dismissals })

    await user.click(await screen.findByRole('button', { name: '전부 거절' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '전부 거절' }),
    )
    await waitFor(() => expect(dismissals).toEqual(['clip-repeated', 'clip-orphan']))
  })
})
