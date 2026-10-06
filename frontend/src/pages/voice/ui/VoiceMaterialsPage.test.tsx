import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import type { VoicePrompt } from '@/entities/voice'
import { renderAppAt, type RenderAppOptions } from '@/test/app'
import type { FakeVoiceOptions } from '@/test/voice'
import { putPhoto } from '@/features/answer-voice-prompt/api/photo'

// The browser conversion and the storage PUT are the device's and the bucket's; the tab only
// has to hand the converted copy on (VOICE-60).
vi.mock('@/features/answer-voice-prompt/api/photo', () => ({
  preparePhoto: vi.fn(async () => ({ blob: new Blob(['jpeg']), width: 1024, height: 768 })),
  putPhoto: vi.fn(async () => undefined),
}))

afterEach(() => initializeI18n('ko'))

const MATERIALS = '/voices/voice-default/materials'
const ANALYZE = { providerId: 'stub', modelId: 'analyze' }
const WITH_ANALYZE: RenderAppOptions['providers'] = {
  models: [{ ...ANALYZE, label: 'Analyze' }],
  selections: [{ stage: Stage.ANALYZE, ...ANALYZE }],
}
const sentences = (n: number) => '정말 맛있었어요.\n'.repeat(n)
const UNMADE = [{ id: 'voice-default', name: '기본 말투', made: false }]
const MADE = [{ id: 'voice-default', name: '기본 말투', made: true }]
const LEGACY_PROMPTS: VoicePrompt[] = [
  {
    key: 'opening_greeting',
    part: 'opening',
    photo: false,
    text: '블로그 글을 시작할 때 쓰는 첫인사를 평소처럼 2~5문장으로 써 보세요.',
  },
  {
    key: 'photo_food',
    part: 'description',
    photo: true,
    text: '음식이나 음료 사진 한 장을 골라, 블로그에 쓰듯 2~5문장으로 써 보세요.',
  },
  {
    key: 'situation_value',
    part: 'description',
    photo: false,
    text: '가격이나 양, 가성비에 대한 생각을 2~5문장으로 써 보세요.',
  },
  {
    key: 'closing_greeting',
    part: 'closing',
    photo: false,
    text: '글을 마무리할 때 쓰는 끝인사를 평소처럼 2~5문장으로 써 보세요.',
  },
]

function renderMaterials(
  voice: FakeVoiceOptions = {},
  calls: string[] = [],
  providers: RenderAppOptions['providers'] = WITH_ANALYZE,
  jobs?: RenderAppOptions['jobs'],
) {
  return renderAppAt(MATERIALS, {
    user: { id: 'alice' },
    calls,
    providers,
    jobs,
    voice: { voices: UNMADE, prompts: LEGACY_PROMPTS, ...voice },
  })
}

// The made voice's management sheets remain available after the first-use funnel.
function renderLegacyMaterials(
  voice: FakeVoiceOptions = {},
  calls: string[] = [],
  providers: RenderAppOptions['providers'] = WITH_ANALYZE,
) {
  return renderMaterials({ voices: MADE, ...voice }, calls, providers)
}

describe('the 학습 데이터 screen', () => {
  // The novice-facing meter names saved questions while retaining the server readiness value.
  it('shows saved-question progress without asking the owner to count sentences', async () => {
    renderMaterials({
      samples: [
        { id: 'a', label: '', kind: 'answer', promptKey: 'opening_greeting', body: sentences(30) },
      ],
    })
    expect(await screen.findByRole('meter', { name: '나의 말투 찾기' })).toHaveAttribute(
      'aria-valuenow',
      '50',
    )
    expect(screen.getByText('질문 1 / 10개')).toBeInTheDocument()
    expect(screen.getByText('질문 9개만 더 답하면 돼요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '이 자료로 내 말투 만들기' })).toBeNull()
    expect(screen.getByRole('button', { name: '자료 더 보태기' })).toBeEnabled()
  })

  it('shows live progress without offering analysis before the question session completes', async () => {
    const user = userEvent.setup()
    renderMaterials({ samples: [{ id: 'p', label: '글', body: sentences(48) }] })

    await user.click(await screen.findByRole('button', { name: '자료 더 보태기' }))
    const sheet = within(
      await screen.findByRole('region', { name: '짧은 질문으로 내 말투 알아보기' }),
    )
    expect(sheet.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    await user.click(sheet.getByLabelText('답'))
    await user.paste(sentences(12))
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(sheet.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1'),
    )
    expect(sheet.queryByRole('button', { name: '이 자료로 내 말투 만들기' })).toBeNull()
  })

  // VOICE-23, VOICE-55: at 100% 말투 만들기 starts one analysis on the active analyze selection.
  it('makes the voice at 100% on the active analyze selection', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const analyses: NonNullable<FakeVoiceOptions['analyses']> = []
    renderMaterials({ samples: [{ id: 'p', label: '글', body: sentences(60) }], analyses }, calls)

    expect(await screen.findByText('이제 말투를 만들 수 있어요')).toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: '이 자료로 내 말투 만들기' }))
    await waitFor(() =>
      expect(analyses).toEqual([{ voiceId: 'voice-default', model: 'stub/analyze' }]),
    )
    expect(
      await screen.findByRole('heading', { name: '내 글에 담긴 말투를 살펴보고 있어요' }),
    ).toBeInTheDocument()
  })

  it('reveals the three tabs when the first analysis publishes', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderMaterials(
      {
        samples: [{ id: 'p', label: '글', body: sentences(60) }],
        analysisAfterAnalysis: '차분한 말투예요.',
      },
      calls,
      WITH_ANALYZE,
      {
        jobs: [
          {
            id: 'voice-job',
            kind: 'analyze_voice',
            status: 'done',
            stage: 'analyze',
            progressDone: 1,
            progressTotal: 1,
          },
        ],
      },
    )

    expect(await screen.findByRole('heading', { level: 2, name: '말투 학습' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: '말투 설정' })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: '이 자료로 내 말투 만들기' }))
    const use = await screen.findByRole('button', { name: '이 말투를 내 글에 사용하기' })
    expect(calls).not.toContain('SetDefaultVoice')
    await user.click(use)
    await waitFor(() => expect(calls).toContain('SetDefaultVoice'))
    const tabs = within(await screen.findByRole('navigation', { name: '말투 설정' })).getAllByRole(
      'link',
    )
    expect(tabs.map((tab) => tab.getAttribute('aria-label'))).toEqual([
      '말투 분석',
      '학습 데이터',
      '검증',
    ])
  })

  it('says a model is needed, with the way to choose one, when there is none', async () => {
    renderMaterials({ samples: [{ id: 'p', label: '글', body: sentences(60) }] }, [], {})
    expect(
      await screen.findByText('AI를 준비하지 못했어요. 다시 확인해 주세요.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'AI 설정 확인하기' })).toHaveAttribute(
      'href',
      '/ai-models',
    )
    expect(screen.queryByRole('button', { name: '이 자료로 내 말투 만들기' })).toBeNull()
    expect(screen.getByRole('button', { name: 'AI 다시 준비하기' })).toBeEnabled()
  })

  // A made voice offers 다시 분석 and no meter.
  it('offers 다시 분석 on a made voice', async () => {
    renderMaterials({
      voices: [{ id: 'voice-default', name: '기본 말투' }],
      samples: [{ id: 'p', label: '글', body: sentences(60) }],
    })
    const again = await screen.findByRole('button', { name: '다시 분석' })
    await waitFor(() => expect(again).toBeEnabled())
    expect(screen.queryByText('말투 학습에 필요한 정보')).not.toBeInTheDocument()
  })

  // VOICE-20, VOICE-65: 글 붙여넣기 takes 200 characters, then the 학습 글 lists the post and the
  // sheet stays open, blank, for the next one; nothing starts.
  it('pastes a post as a 학습 글 without starting anything', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderLegacyMaterials({}, calls)

    await user.click(await screen.findByRole('button', { name: '글 붙여넣기' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.type(sheet.getByLabelText('제목 (선택)'), '제주 여행')
    await user.type(sheet.getByLabelText('내가 쓴 글'), '가'.repeat(199))
    expect(sheet.getByRole('button', { name: '추가' })).toBeEnabled()
    await user.click(sheet.getByRole('button', { name: '추가' }))
    expect(sheet.getByRole('alert')).toHaveTextContent('1자 더 필요해요')
    expect(calls).not.toContain('AddVoiceSample')
    await user.type(sheet.getByLabelText('내가 쓴 글'), '가')
    await user.click(sheet.getByRole('button', { name: '추가' }))

    await waitFor(() => expect(calls).toContain('AddVoiceSample'))
    expect(await screen.findByText('제주 여행')).toBeInTheDocument()
    expect(calls).not.toContain('AnalyzeVoice')

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(sheet.getByRole('status')).toHaveTextContent('글을 추가했어요')
    expect(sheet.getByLabelText('제목 (선택)')).toHaveValue('')
    await waitFor(() => expect(sheet.getByLabelText('제목 (선택)')).toHaveFocus())
    expect(sheet.getByLabelText('내가 쓴 글')).toHaveValue('')
    expect(sheet.queryByRole('button', { name: '취소' })).not.toBeInTheDocument()

    await user.type(sheet.getByLabelText('제목 (선택)'), '부산 여행')
    expect(sheet.getByRole('status')).toHaveTextContent('')
    await user.click(sheet.getByLabelText('내가 쓴 글'))
    await user.paste('나'.repeat(200))
    await user.click(sheet.getByRole('button', { name: '추가' }))
    expect(await screen.findByText('부산 여행')).toBeInTheDocument()
    expect(calls.filter((call) => call === 'AddVoiceSample')).toHaveLength(2)

    await user.click(sheet.getByRole('button', { name: '닫기' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  // VOICE-65: a refused post keeps the sheet on it, text and all.
  it('keeps a refused post in the sheet', async () => {
    const user = userEvent.setup()
    renderLegacyMaterials({ addError: 'too short' })

    await user.click(await screen.findByRole('button', { name: '글 붙여넣기' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.click(sheet.getByLabelText('내가 쓴 글'))
    await user.paste('가'.repeat(200))
    await user.click(sheet.getByRole('button', { name: '추가' }))

    expect(await sheet.findByRole('alert')).toHaveTextContent('200자')
    expect(sheet.getByLabelText('내가 쓴 글')).toHaveValue('가'.repeat(200))
    expect(sheet.getByRole('status')).toHaveTextContent('')
    expect(sheet.getByRole('button', { name: '취소' })).toBeInTheDocument()
  })

  // VOICE-65: the sheet starts with the first unanswered prompt, then advances after a save.
  it('answers a prompt and moves on to the next one', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderLegacyMaterials({ answers, prompts: LEGACY_PROMPTS.filter((prompt) => !prompt.photo) })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByText(/블로그 글을 시작할 때 쓰는 첫인사/)).toBeInTheDocument()
    expect(sheet.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(sheet.getByRole('button', { name: '다른 질문으로 바꾸기' })).toBeEnabled()
    expect(sheet.queryByRole('button', { name: '문항 목록' })).not.toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '안녕하세요!')
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'opening_greeting', body: '안녕하세요!', uploadId: '' },
      ]),
    )
    expect(await sheet.findByText(/가격이나 양, 가성비/)).toBeInTheDocument()
    expect(sheet.getByRole('status')).toHaveTextContent('답을 저장했어요')
    expect(sheet.getByLabelText('답')).toHaveValue('')
    expect(sheet.getByLabelText('답')).not.toHaveFocus()
    await user.click(sheet.getByRole('button', { name: '닫기' }))
    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    expect(
      await within(screen.getByRole('dialog')).findByText(/가격이나 양, 가성비/),
    ).toBeInTheDocument()
  })

  // VOICE-65: at 100%, the last answer shows completion and a close action.
  it('shows completion after the last unanswered prompt at 100%', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderLegacyMaterials({
      answers,
      samples: [
        { id: 'a0', label: '', kind: 'answer', promptKey: 'opening_greeting', body: sentences(57) },
        {
          id: 'a1',
          label: '',
          kind: 'answer',
          promptKey: 'photo_food',
          hasPhoto: true,
          body: sentences(1),
        },
        { id: 'a2', label: '', kind: 'answer', promptKey: 'situation_value', body: sentences(1) },
      ],
    })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByText(/글을 마무리할 때 쓰는 끝인사/)).toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '다음에 또 만나요!')
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'closing_greeting', body: '다음에 또 만나요!', uploadId: '' },
      ]),
    )
    expect(await sheet.findByText('모든 문항에 답했어요.')).toBeInTheDocument()
    expect(sheet.getByRole('status')).toHaveTextContent('답을 저장했어요')
    expect(sheet.queryByLabelText('답')).not.toBeInTheDocument()
    await user.click(sheet.getByRole('button', { name: '닫기' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // Catalog exhaustion never silently reopens or rewrites a saved answer.
  it('offers completion and explicit review when the catalog is exhausted', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderLegacyMaterials({
      answers,
      samples: [
        { id: 'a0', label: '', kind: 'answer', promptKey: 'opening_greeting', body: '안녕하세요.' },
        {
          id: 'a1',
          label: '',
          kind: 'answer',
          promptKey: 'photo_food',
          hasPhoto: true,
          body: '맛있어요.',
        },
        { id: 'a2', label: '', kind: 'answer', promptKey: 'situation_value', body: '좋았어요.' },
      ],
    })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByText(/글을 마무리할 때 쓰는 끝인사/)).toBeInTheDocument()
    expect(sheet.getByRole('button', { name: '다른 질문으로 바꾸기' })).toBeEnabled()
    expect(sheet.queryByRole('button', { name: '문항 목록' })).not.toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '다음에 만나요.')
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'closing_greeting', body: '다음에 만나요.', uploadId: '' },
      ]),
    )
    expect(await sheet.findByText('모든 문항에 답했어요.')).toBeInTheDocument()
    expect(sheet.queryByLabelText('답')).toBeNull()
    await user.click(sheet.getByRole('button', { name: '답변을 더 보태거나 고치기' }))
    expect(sheet.getByRole('combobox', { name: /저장한 답변 다시 보기/ })).toBeInTheDocument()
    expect(sheet.queryByRole('button', { name: '답 고치기' })).toBeNull()
  })

  // VOICE-60, VOICE-32: an answered prompt reopens on its answer; the rewrite replaces it — a
  // photo answer on its own photo — and the meter counts the added sentences.
  it('rewrites an answered prompt with more sentences', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderLegacyMaterials({
      answers,
      samples: [
        {
          id: 'a1',
          label: '',
          kind: 'answer',
          promptKey: 'opening_greeting',
          body: '안녕하세요. 반가워요.',
        },
        {
          id: 'a2',
          label: '',
          kind: 'answer',
          promptKey: 'photo_food',
          hasPhoto: true,
          body: '짜장면이에요. 맛있었어요.',
        },
        {
          id: 'a3',
          label: '',
          kind: 'answer',
          promptKey: 'situation_value',
          body: '가격이 좋아요. 양도 좋아요.',
        },
        {
          id: 'a4',
          label: '',
          kind: 'answer',
          promptKey: 'closing_greeting',
          body: '다음에 봐요. 안녕히 계세요.',
        },
      ],
    })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.click(await sheet.findByRole('button', { name: '답변을 더 보태거나 고치기' }))
    await user.click(await sheet.findByRole('combobox', { name: /저장한 답변 다시 보기/ }))
    await user.click(
      await screen.findByRole('option', { name: /블로그 글을 시작할 때 쓰는 첫인사/ }),
    )
    const field = await sheet.findByLabelText('답')
    expect(field).toHaveValue('안녕하세요. 반가워요.')
    expect(field).not.toHaveFocus()
    await user.type(field, ' 오늘도 와 주셔서 고마워요.')
    await user.click(sheet.getByRole('button', { name: '답 고치기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        {
          promptKey: 'opening_greeting',
          body: '안녕하세요. 반가워요. 오늘도 와 주셔서 고마워요.',
          uploadId: '',
        },
      ]),
    )
    expect(await sheet.findByText('모든 문항에 답했어요.')).toBeInTheDocument()
    expect(sheet.queryByLabelText('답')).toBeNull()
    await user.click(sheet.getByRole('combobox', { name: /저장한 답변 다시 보기/ }))
    await user.click(await screen.findByRole('option', { name: /음식이나 음료 사진/ }))
    expect(await sheet.findByRole('img', { name: '고른 사진' })).toHaveAttribute(
      'src',
      'https://storage.test/voices/a2.jpg',
    )
    await user.type(sheet.getByLabelText('답'), ' 면이 쫄깃했어요.')
    await user.click(sheet.getByRole('button', { name: '답 고치기' }))
    await waitFor(() =>
      expect(answers.at(-1)).toEqual({
        promptKey: 'photo_food',
        body: '짜장면이에요. 맛있었어요. 면이 쫄깃했어요.',
        uploadId: '',
      }),
    )
    expect(await sheet.findByText('모든 문항에 답했어요.')).toBeInTheDocument()
    expect(sheet.queryByLabelText('답')).toBeNull()
  })

  // VOICE-60: a photo prompt is answered on the owner's own photo, converted and PUT first.
  it('answers a photo prompt on a picked photo', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderLegacyMaterials(
      {
        answers,
        voices: MADE,
        samples: [{ id: 'a0', label: '', kind: 'answer', promptKey: 'opening_greeting' }],
      },
      calls,
    )

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByText(/음식이나 음료 사진/)).toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '짜장면이 맛있었어요.')
    // Without a photo there is nothing to answer on.
    expect(sheet.getByRole('button', { name: '답하기' })).toBeDisabled()
    await user.upload(
      sheet.getByLabelText('사진 고르기'),
      new File(['x'], 'food.heic', { type: 'image/heic' }),
    )
    expect(await sheet.findByRole('img', { name: '고른 사진' })).toBeInTheDocument()
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'photo_food', body: '짜장면이 맛있었어요.', uploadId: 'voice-upload-1' },
      ]),
    )
    expect(calls).toContain('CreateVoicePhotoUpload')
    // The next prompt starts without the photo the last one was answered on.
    expect(await sheet.findByText(/가격이나 양, 가성비/)).toBeInTheDocument()
    expect(sheet.queryByRole('img', { name: '고른 사진' })).not.toBeInTheDocument()
  })

  // VOICE-65: a refused answer keeps the sheet on its prompt, text and photo kept.
  it('keeps a refused answer on its prompt', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    vi.mocked(putPhoto).mockRejectedValueOnce(new Error('offline'))
    renderLegacyMaterials({
      answers,
      voices: MADE,
      samples: [{ id: 'a0', label: '', kind: 'answer', promptKey: 'opening_greeting' }],
    })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByText(/음식이나 음료 사진/)).toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '짜장면이 맛있었어요.')
    await user.upload(
      sheet.getByLabelText('사진 고르기'),
      new File(['x'], 'food.heic', { type: 'image/heic' }),
    )
    await sheet.findByRole('img', { name: '고른 사진' })
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    expect(
      await sheet.findByText('사진을 올리지 못했어요. 다시 시도해 주세요.'),
    ).toBeInTheDocument()
    expect(sheet.getByText(/음식이나 음료 사진/)).toBeInTheDocument()
    expect(sheet.getByLabelText('답')).toHaveValue('짜장면이 맛있었어요.')
    expect(sheet.getByRole('img', { name: '고른 사진' })).toBeInTheDocument()
    expect(sheet.getByRole('status')).toHaveTextContent('')
    expect(answers).toEqual([])
  })
})
