// The draft's assignments: its voice, its target language, its 템플릿 and that template's data
// fields in ①.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  USER,
  briefField,
  openBrief,
  openStep,
  resetEditorTest,
  templateField,
  voiceField,
} from '@/test/editor'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import { finalizedPostRow, type FakeDraftSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('the post language', () => {
  const AUTOSAVED = { timeout: 4_000 }

  it('snapshots the current UI locale for a new post and sends it on the first request', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    expect(await briefField(user, 'Post language')).toHaveTextContent('English')
    await user.keyboard('{Escape}')
    await user.type(screen.getByLabelText('Title'), 'English draft')

    await waitFor(() => expect(draftSaves[0]?.targetLanguage).toBe('en'), AUTOSAVED)
  })

  it('sends an explicit pre-create language instead of the UI-locale default', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    const language = await briefField(user, 'Post language')
    await user.click(language)
    await user.click(await screen.findByRole('option', { name: 'Korean' }))
    await user.keyboard('{Escape}')
    await user.type(screen.getByLabelText('Title'), 'Korean target')

    await waitFor(() => expect(draftSaves[0]?.targetLanguage).toBe('ko'), AUTOSAVED)
  })

  it('restores an existing post language independently of the current UI locale', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/korean-post', {
      user: USER,
      posts: {
        posts: [{ slug: 'korean-post', title: '한국어 글', targetLanguage: 'ko' }],
        draftSaves,
      },
    })

    const user = userEvent.setup()
    expect(await briefField(user, 'Post language')).toHaveTextContent('Korean')
    expect(draftSaves).toHaveLength(0)
  })
})

describe('the post voice', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const OPTION_NAMES: Record<string, string> = {
    'voice-review': '리뷰',
    'voice-default': '기본 말투',
    '': '말투 없음',
  }
  /** The picker lists the directory, which answers after the first paint; choose only once it has. */
  async function pickVoice(user: ReturnType<typeof userEvent.setup>, voiceId: string) {
    const picker = await voiceField(user)
    await waitFor(() => expect(picker).toBeEnabled())
    await user.click(picker)
    await user.click(await screen.findByRole('option', { name: OPTION_NAMES[voiceId] }))
    return picker
  }
  const confirmDialog = () => screen.findByRole('dialog', { name: '말투를 바꿀까요?' })
  const TWO_VOICES = [
    { id: 'voice-default', name: '기본 말투', isDefault: true },
    { id: 'voice-review', name: '리뷰' },
  ]
  const POST_VOICES = [
    { id: 'voice-default', name: '기본 말투' },
    { id: 'voice-review', name: '리뷰' },
  ]
  const reviewPost = {
    slug: '20260820-jeju',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }

  // POST-23: the picker opens on the default and the create names it.
  it('starts a new draft in the default voice and sends its id with the first save', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    expect(await voiceField(user)).toHaveTextContent('기본 말투')
    await user.type(screen.getByLabelText('제목'), '제주')

    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-제주'),
      AUTOSAVED,
    )
    expect(draftSaves[0]).toEqual({
      slug: '',
      voiceId: 'voice-default',
      templateId: undefined,
      templateAnswers: [],
      targetLanguage: 'ko',
    })
    // The editor the mint mounted shows the same voice, and later saves leave it alone.
    expect(await voiceField(user)).toHaveTextContent('기본 말투')
    await user.type(screen.getByLabelText('메모'), '첫날')
    await waitFor(() => expect(draftSaves).toHaveLength(2), AUTOSAVED)
    expect(draftSaves[1]).toEqual({
      slug: '20260828-제주',
      voiceId: undefined,
      templateId: undefined,
      templateAnswers: [],
      targetLanguage: undefined,
    })
  })

  // POST-101: no 기본 is no reason to block — the draft starts on 말투 없음, and the create names
  // no voice, which is how the server hears 말투 없음 (POST-8).
  it('starts a new draft on 말투 없음 when the account has no 기본', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves, voices: POST_VOICES },
      voice: { voices: [{ id: 'voice-review', name: '리뷰' }] },
    })

    expect(await voiceField(user)).toHaveTextContent('말투 없음')
    await user.type(screen.getByLabelText('제목'), '제주')

    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-제주'),
      AUTOSAVED,
    )
    expect(draftSaves[0]).toEqual({
      slug: '',
      voiceId: undefined,
      templateId: undefined,
      templateAnswers: [],
      targetLanguage: 'ko',
    })
    expect(await voiceField(user)).toHaveTextContent('말투 없음')
  })

  // A 기본 not yet made cannot be assigned (VOICE-32), so it is no seed either.
  it('starts a new draft on 말투 없음 when the 기본 is not made yet', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/new', {
      user: USER,
      voice: { voices: [{ id: 'voice-default', name: '기본 말투', isDefault: true, made: false }] },
    })

    expect(await voiceField(user)).toHaveTextContent('말투 없음')
  })

  it('lets a new draft switch to 말투 없음 before anything is typed', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    const picker = await pickVoice(user, '')
    expect(picker).toHaveTextContent('말투 없음')
    await user.type(screen.getByLabelText('제목'), '제주')

    await waitFor(() => expect(draftSaves).toHaveLength(1), AUTOSAVED)
    expect(draftSaves[0].voiceId).toBeUndefined()
  })

  // VOICE-53: the picker's last option opens the directory's own create sheet, over the draft,
  // and choosing it assigns nothing.
  it('opens the create sheet from 새 말투 만들기 and keeps the voice', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [reviewPost], draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
    })

    const picker = await voiceField(user)
    await waitFor(() => expect(picker).toBeEnabled())
    await user.click(picker)
    await user.click(await screen.findByRole('option', { name: '새 말투 만들기' }))

    const sheet = await screen.findByRole('dialog', { name: '새 말투' })
    expect(within(sheet).getByLabelText('말투 이름')).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: '말투를 바꿀까요?' })).not.toBeInTheDocument()
    await user.click(within(sheet).getByRole('button', { name: '취소' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(picker).toHaveTextContent('기본 말투')
    expect(draftSaves).toEqual([])
  })

  it('lets a new draft pick another voice before anything is typed', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
    })

    await pickVoice(user, 'voice-review')
    // Choosing a voice is not typing: no post exists until there is something to save.
    expect(draftSaves).toHaveLength(0)
    await user.type(screen.getByLabelText('제목'), '리뷰 글')

    await waitFor(
      () =>
        expect(draftSaves[0]).toEqual({
          slug: '',
          voiceId: 'voice-review',
          templateId: undefined,
          templateAnswers: [],
          targetLanguage: 'ko',
        }),
      AUTOSAVED,
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/20260828-리뷰-글'))
    expect(await voiceField(user)).toHaveTextContent('리뷰')
  })

  // POST-24: reassignment is confirmed, and it changes the voice alone — the content, the baseline
  // and finalizability all stay, so 확정하기 finalizes the moved post as it would have before.
  it('reassigns an existing post after confirmation and leaves it finalizable', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [reviewPost], draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
    })

    const picker = await voiceField(user)
    await waitFor(() => expect(picker).toHaveTextContent('기본 말투'))
    await pickVoice(user, 'voice-review')
    const dialog = await confirmDialog()
    expect(dialog).toHaveTextContent('지금까지 배운 내용은 이전 말투에 남고')
    // The choice is not applied until it is confirmed.
    expect(draftSaves).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: '말투 변경' }))

    await waitFor(() =>
      expect(draftSaves).toEqual([
        { slug: '20260820-jeju', voiceId: 'voice-review', templateAnswers: [] },
      ]),
    )
    await waitFor(() => expect(picker).toHaveTextContent('리뷰'))
    await user.keyboard('{Escape}')
    // The canonical content survived, and 글 다듬기's one way out still finalizes it: nothing about
    // a finalize depends on the voice the draft was written under (POST-13).
    await openStep(user, '글 다듬기')
    expect(await screen.findByRole('button', { name: '확정하기' })).toBeEnabled()
    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
  })

  // POST-24: '' clears to 말투 없음, and a post with no voice still generates (GEN-25).
  it('clears an existing post to 말투 없음 after confirmation', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, status: 'draft' }], draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    const picker = await voiceField(user)
    await waitFor(() => expect(picker).toHaveTextContent('기본 말투'))
    await pickVoice(user, '')
    await user.click(within(await confirmDialog()).getByRole('button', { name: '말투 변경' }))

    await waitFor(() =>
      expect(draftSaves).toEqual([{ slug: '20260820-jeju', voiceId: '', templateAnswers: [] }]),
    )
    await waitFor(() => expect(picker).toHaveTextContent('말투 없음'))
    await waitFor(() => expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeEnabled())
  })

  // POST-25: a voice not yet made refuses the AI actions like a deleted one, with its own
  // sentence, and the warning links to that voice, where it is made.
  it('refuses AI actions on a voice not yet made and links to it', async () => {
    const user = userEvent.setup()
    const unmade = { id: 'voice-cafe', name: '가게 소개', made: false }
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [{ ...reviewPost, status: 'draft', voice: unmade }],
        voices: [...POST_VOICES, unmade],
      },
      voice: { voices: [...TWO_VOICES, unmade] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    expect(await voiceField(user)).toHaveTextContent('가게 소개 · 만드는 중 0%')
    const reason = '아직 만들지 않은 말투예요. 말투를 만들거나 다른 말투로 바꿔 주세요.'
    expect(await screen.findByText(reason)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '말투 학습하기' })).toHaveAttribute(
      'href',
      '/voices/voice-cafe',
    )
    await waitFor(() => expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeDisabled())

    await openStep(user, '글 다듬기')
    expect(await screen.findByRole('button', { name: '수정' })).toBeDisabled()
    expect(screen.getByText(reason)).toBeInTheDocument()
  })

  // POST-24: a reassignment leaves the status alone, so a finalized post stays finalized and its
  // dock keeps the road onward rather than asking for another 확정.
  it('keeps a finalized post finalized when it is reassigned', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          finalizedPostRow({
            ...reviewPost,
            status: 'finalized',
            finalizedRevision: reviewPost.contentRevision,
          }),
        ],
        voices: POST_VOICES,
      },
      voice: { voices: TWO_VOICES },
    })

    await pickVoice(user, 'voice-review')
    await user.click(within(await confirmDialog()).getByRole('button', { name: '말투 변경' }))
    await waitFor(async () => expect(await voiceField(user)).toHaveTextContent('리뷰'))
    await openStep(user, '글 다듬기')

    expect(await screen.findByRole('button', { name: '글 완성으로 가기' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
  })

  // POST-25: the tombstone, the disabled AI controls with their reason, and both ways out.
  it('renders a deleted voice as a tombstone with disabled AI actions and a way out', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            ...reviewPost,
            status: 'draft',
            voice: { id: 'voice-old', name: '옛 말투', deleted: true },
          },
        ],
        voices: [...POST_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }],
      },
      voice: {
        voices: [...TWO_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    // Named twice on template, and both are on the page from the first paint now that the picker
    // rides the dock: the picker's closed trigger (the disabled current option) and 글 생성's own
    // warning.
    expect(await screen.findAllByText('삭제된 말투 · 옛 말투')).toHaveLength(2)
    const picker = await voiceField(user)
    expect(picker).toHaveTextContent('삭제된 말투 · 옛 말투')
    // The refusal is said ONCE, by the surface that carries the way out: 글 생성's tombstone
    // warning offers 복원, so the dock only disables the action rather than re-writing the reason
    // under it (POST-48).
    await waitFor(() => expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeDisabled())
    expect(
      screen.queryByText('생성: 삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.'),
    ).not.toBeInTheDocument()
    // The manual side of the post is untouched: its content and export are still there.
    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    await openStep(user, '글 다듬기')
    expect(
      await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
    // The revision is refused before any provider call, and the refusal says why, above it.
    expect(
      screen.getByText('삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.'),
    ).toBeInTheDocument()
    // A content-only finalize checks no voice (POST-13).
    expect(screen.getByRole('button', { name: '확정하기' })).toBeInTheDocument()

    // Restore is offered in place and asks the server, nothing else ([I5]).
    await openStep(user, '글 생성')
    await user.click(screen.getByRole('button', { name: '복원' }))
    await waitFor(() => expect(calls).toContain('RestoreVoice'))
    expect(calls).not.toContain('StartGeneration')
  })

  it('recovers a deleted-voice post by reassigning it to an active voice', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            memo: '메모',
            voice: { id: 'voice-old', name: '옛 말투', deleted: true },
          },
        ],
        draftSaves,
        voices: [...POST_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }],
      },
      voice: { voices: [...TWO_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    // Twice from the first paint: the dock's picker names the tombstone, and 글 생성 warns about it.
    await screen.findAllByText('삭제된 말투 · 옛 말투')
    await pickVoice(user, 'voice-review')
    await user.click(within(await confirmDialog()).getByRole('button', { name: '말투 변경' }))

    await waitFor(() =>
      expect(draftSaves).toEqual([
        { slug: '20260820-jeju', voiceId: 'voice-review', templateAnswers: [] },
      ]),
    )
    await waitFor(() => expect(screen.queryAllByText('삭제된 말투 · 옛 말투')).toHaveLength(0))
    await waitFor(() => expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeEnabled())
  })
})

// TMPL-32: 템플릿 is optional, defaults to 없음, and rides the same draft queue as the text.
describe('the post template', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const PURPOSES = [
    { id: 'template-review', name: '정보성 식당 리뷰', description: '협찬 방문 리뷰' },
    { id: 'template-diary', name: '일기' },
  ]
  const POST_PURPOSES = [
    { id: 'template-review', name: '정보성 식당 리뷰' },
    { id: 'template-diary', name: '일기' },
  ]

  async function pickTemplate(user: ReturnType<typeof userEvent.setup>, name: string) {
    const picker = await templateField(user)
    await waitFor(() => expect(picker).toBeEnabled())
    await user.click(picker)
    await user.click(await screen.findByRole('option', { name }))
    return picker
  }

  it('defaults a new draft to 없음 and sends no template with the create', async () => {
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves },
      templates: { templates: PURPOSES },
    })

    // The select reads 없음 on its own (PostTemplateSelect.test.tsx); the page's part is the create.
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('제목'), '제주')
    await waitFor(() => expect(draftSaves).toHaveLength(1), AUTOSAVED)
    // Omitted, not '': the create has no assignment to clear, so the request is byte-for-byte
    // what it was before templates existed.
    expect(draftSaves[0].templateId).toBeUndefined()
  })

  // TMPL-48: picking a template seeds the post's two generation options, and the screen
  // shows the seeded values at once — on the response the picker already awaits, with no
  // second call and nothing saying a template wrote them.
  it('shows the numbers a picked template seeded', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: {
        posts: [{ slug: '20260820-memo' }],
        templates: [
          { id: 'template-review', name: '정보성 식당 리뷰', targetLength: 1800, tagCount: 7 },
          { id: 'template-diary', name: '일기' },
        ],
      },
      templates: { templates: PURPOSES },
    })

    const brief = await openBrief(user)
    // The post starts with natural length and the default count.
    expect(within(brief).queryByLabelText('목표 글자 수')).not.toBeInTheDocument()
    expect(within(brief).getByLabelText('최대 태그 수')).toHaveValue(4)
    await user.keyboard('{Escape}')

    await pickTemplate(user, '정보성 식당 리뷰')
    await waitFor(() => expect(calls).toContain('SavePostDraft'), AUTOSAVED)

    const seeded = await openBrief(user)
    // The length arrived, so its tick is on and the field carries it.
    await waitFor(() => expect(within(seeded).getByLabelText('목표 글자 수')).toHaveValue(1800))
    expect(within(seeded).getByLabelText('최대 태그 수')).toHaveValue(7)
    // Seeded, not saved twice: the values rode the assignment's own response.
    expect(calls).not.toContain('SavePostGenerationOptions')
  })

  it('carries a chosen template into the create', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves, templates: POST_PURPOSES },
      templates: { templates: PURPOSES },
    })

    await pickTemplate(user, '정보성 식당 리뷰')
    // Choosing is not typing: nothing is saved until there is something to save.
    expect(draftSaves).toHaveLength(0)

    await user.type(screen.getByLabelText('제목'), '리뷰 글')
    await waitFor(
      () => expect(draftSaves[0]).toMatchObject({ slug: '', templateId: 'template-review' }),
      AUTOSAVED,
    )
  })
})

// POST-54 · POST-62: the selected template's data fields belong to ①, between the 가제 and the
// memo, because they are the material 글 생성 works from and the memo carries what they did not
// ask. They autosave on the memo's own queue.
describe('the template data fields in ①', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const WITH_FIELDS = [
    {
      id: 'template-review',
      name: '정보성 식당 리뷰',
      body: '<write>인트로</write>\n<ask label="방문일"/>\n<ask label="총평 별점">별점과 총평</ask>',
    },
  ]

  it('seeds the fields from what the post already answered', async () => {
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: {
        draftSaves,
        posts: [
          {
            slug: '20260301-jeju',
            title: '제주',
            memo: '갔다',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
            templateAnswers: [{ label: '총평 별점', text: '4.5점', enabled: false }],
          },
        ],
        templates: [{ id: 'template-review', name: '정보성 식당 리뷰' }],
      },
      templates: { templates: WITH_FIELDS },
    })

    // Both fields, in body order, with the stored answer and switch on the second.
    expect(await screen.findByLabelText('방문일')).toHaveValue('')
    const rated = screen.getByLabelText('총평 별점')
    expect(rated).toHaveValue('4.5점')
    // Off keeps its text and greys the field: the switch is a decision the author can take back.
    expect(rated).toBeDisabled()
    expect(screen.getByRole('switch', { name: '총평 별점 넣기' })).not.toBeChecked()
    // A mount is not an edit: the fields the post has not answered read as their default, so
    // nothing is queued and no save goes out.
    expect(draftSaves).toEqual([])
  })

  function renderJeju(templateAnswers: { label: string; text: string; enabled: boolean }[] = []) {
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260301-jeju',
            title: '제주',
            memo: '갔다',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
            templateAnswers,
          },
        ],
        templates: [{ id: 'template-review', name: '정보성 식당 리뷰' }],
      },
      templates: { templates: WITH_FIELDS },
    })
  }

  const before = (a: Element, b: Element) =>
    Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)

  it('lays out 가제, the fields in body order, then the memo', async () => {
    renderJeju()
    const visited = await screen.findByLabelText('방문일')
    const rated = screen.getByLabelText('총평 별점')
    const title = screen.getByLabelText('제목')
    const memo = screen.getByLabelText('메모')
    expect(before(title, visited)).toBe(true)
    expect(before(visited, rated)).toBe(true)
    expect(before(rated, memo)).toBe(true)
  })

  it('moves Enter in the 가제 to the first field that takes typing', async () => {
    const user = userEvent.setup()
    renderJeju([{ label: '방문일', text: '', enabled: false }])
    await screen.findByLabelText('방문일')
    await user.click(screen.getByLabelText('제목'))
    await user.keyboard('{Enter}')
    // 방문일 is switched off, so the next field on screen that takes typing is 총평 별점.
    expect(screen.getByLabelText('총평 별점')).toHaveFocus()
    expect(screen.getByLabelText('제목')).toHaveValue('제주')
  })

  it('moves Enter in the 가제 to the memo when the post has no field to fill', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260301-jeju', title: '제주', memo: '갔다' }] },
    })
    await user.click(await screen.findByLabelText('제목'))
    await user.keyboard('{Enter}')
    expect(screen.getByLabelText('메모')).toHaveFocus()
  })

  it('autosaves an answer on the memo’s own queue', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: {
        draftSaves,
        posts: [
          {
            slug: '20260301-jeju',
            title: '제주',
            memo: '갔다',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
          },
        ],
        templates: [{ id: 'template-review', name: '정보성 식당 리뷰' }],
      },
      templates: { templates: WITH_FIELDS },
    })

    await user.type(await screen.findByLabelText('방문일'), '2026-03-01')
    await waitFor(() => expect(draftSaves.length).toBeGreaterThan(0), AUTOSAVED)
    const last = draftSaves[draftSaves.length - 1]
    // The whole set on screen rides one save; every entry is an upsert of its own label.
    expect(last.templateAnswers).toEqual([
      { label: '방문일', text: '2026-03-01', enabled: true },
      { label: '총평 별점', text: '', enabled: true },
    ])
  })
})
