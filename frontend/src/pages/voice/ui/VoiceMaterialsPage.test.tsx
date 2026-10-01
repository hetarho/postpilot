import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
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

function renderMaterials(
  voice: FakeVoiceOptions = {},
  calls: string[] = [],
  providers: RenderAppOptions['providers'] = WITH_ANALYZE,
) {
  return renderAppAt(MATERIALS, {
    user: { id: 'alice' },
    calls,
    providers,
    voice: { voices: UNMADE, ...voice },
  })
}

describe('the 학습 데이터 screen', () => {
  // VOICE-32: the meter counts sentences, says how many are still needed and names a missing
  // part; 100% says so.
  it('shows the readiness meter with its missing parts', async () => {
    renderMaterials({
      samples: [
        { id: 'a', label: '', kind: 'answer', promptKey: 'opening_greeting', body: sentences(30) },
      ],
    })
    expect(await screen.findByText('말투 학습에 필요한 정보')).toBeInTheDocument()
    expect(screen.getByText('50% 확보')).toBeInTheDocument()
    expect(
      screen.getByText('30문장이 더 필요해요. 아직 없는 부분: 본문 · 마무리'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 만들기' })).toBeDisabled()
    expect(
      screen.getByText('말투 학습에 필요한 정보가 100%가 되면 누를 수 있어요.'),
    ).toBeInTheDocument()
  })

  it('shows live quiz progress and offers 말투 만들기 at 100%', async () => {
    const user = userEvent.setup()
    renderMaterials({ samples: [{ id: 'p', label: '글', body: sentences(48) }] })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByRole('meter')).toHaveAttribute('aria-valuenow', '80')
    expect(sheet.getByText('거의 다 왔어요')).toBeInTheDocument()
    await user.click(sheet.getByLabelText('답'))
    await user.paste(sentences(12))
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() => expect(sheet.getByRole('meter')).toHaveAttribute('aria-valuenow', '100'))
    expect(sheet.queryByText('거의 다 왔어요')).not.toBeInTheDocument()
    expect(sheet.getByRole('button', { name: '말투 만들기' })).toBeEnabled()
  })

  // VOICE-23, VOICE-55: at 100% 말투 만들기 starts one analysis on the active analyze selection.
  it('makes the voice at 100% on the active analyze selection', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const analyses: NonNullable<FakeVoiceOptions['analyses']> = []
    renderMaterials({ samples: [{ id: 'p', label: '글', body: sentences(60) }], analyses }, calls)

    expect(await screen.findByText('이제 말투를 만들 수 있어요')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '말투 만들기' }))
    await waitFor(() =>
      expect(analyses).toEqual([{ voiceId: 'voice-default', model: 'stub/analyze' }]),
    )
    expect(await screen.findByRole('region', { name: '문체 분석 상태' })).toBeInTheDocument()
  })

  it('reveals the three tabs when the first analysis publishes', async () => {
    const user = userEvent.setup()
    renderMaterials({
      samples: [{ id: 'p', label: '글', body: sentences(60) }],
      analysisAfterAnalysis: '차분한 말투예요.',
    })

    expect(await screen.findByRole('heading', { level: 2, name: '말투 학습' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: '말투 설정' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '말투 만들기' }))
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
    expect(await screen.findByText(/말투 분석에 쓸 AI 모델을 먼저 골라 주세요/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'AI 모델 고르기' })).toHaveAttribute(
      'href',
      '/ai-models',
    )
    expect(screen.getByRole('button', { name: '말투 만들기' })).toBeDisabled()
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
    renderMaterials({}, calls)

    await user.click(await screen.findByRole('button', { name: '글 붙여넣기' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.type(sheet.getByLabelText('제목 (선택)'), '제주 여행')
    await user.type(sheet.getByLabelText('내가 쓴 글'), '가'.repeat(199))
    expect(sheet.getByRole('button', { name: '추가' })).toBeDisabled()
    expect(sheet.getByText('1자 더 필요해요')).toBeInTheDocument()
    await user.type(sheet.getByLabelText('내가 쓴 글'), '가')
    await user.click(sheet.getByRole('button', { name: '추가' }))

    await waitFor(() => expect(calls).toContain('AddVoiceSample'))
    expect(await screen.findByText('제주 여행')).toBeInTheDocument()
    expect(calls).not.toContain('AnalyzeVoice')

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(sheet.getByRole('status')).toHaveTextContent('글을 추가했어요')
    expect(sheet.getByLabelText('제목 (선택)')).toHaveValue('')
    expect(sheet.getByLabelText('제목 (선택)')).toHaveFocus()
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
    renderMaterials({ addError: 'too short' })

    await user.click(await screen.findByRole('button', { name: '글 붙여넣기' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.click(sheet.getByLabelText('내가 쓴 글'))
    await user.paste('가'.repeat(200))
    await user.click(sheet.getByRole('button', { name: '추가' }))

    expect(await sheet.findByText(/200자/, { selector: '[id$="-error"]' })).toBeInTheDocument()
    expect(sheet.getByLabelText('내가 쓴 글')).toHaveValue('가'.repeat(200))
    expect(sheet.getByRole('status')).toHaveTextContent('')
    expect(sheet.getByRole('button', { name: '취소' })).toBeInTheDocument()
  })

  // VOICE-65: the sheet starts with the first unanswered prompt, then advances after a save.
  it('answers a prompt and moves on to the next one', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials({ answers })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(await sheet.findByText(/블로그 글을 시작할 때 쓰는 첫인사/)).toBeInTheDocument()
    expect(sheet.getByRole('meter')).toHaveAttribute('aria-valuenow', '0')
    expect(sheet.queryByRole('button', { name: '건너뛰기' })).not.toBeInTheDocument()
    expect(sheet.queryByRole('button', { name: '문항 목록' })).not.toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '안녕하세요!')
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'opening_greeting', body: '안녕하세요!', uploadId: '' },
      ]),
    )
    expect(await sheet.findByText(/음식이나 음료 사진/)).toBeInTheDocument()
    expect(sheet.getByRole('status')).toHaveTextContent('답을 저장했어요')
    expect(sheet.getByLabelText('답')).toHaveValue('')
    expect(sheet.getByLabelText('답')).toHaveFocus()
    await user.click(sheet.getByRole('button', { name: '닫기' }))
    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    expect(
      await within(screen.getByRole('dialog')).findByText(/음식이나 음료 사진/),
    ).toBeInTheDocument()
  })

  // VOICE-65: at 100%, the last answer shows completion and a close action.
  it('shows completion after the last unanswered prompt at 100%', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials({
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

  // VOICE-65: after the last answer below 100%, continue with a prefilled answer to extend.
  it('continues into answered prompts when readiness is short', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials({
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
    expect(sheet.queryByRole('button', { name: '건너뛰기' })).not.toBeInTheDocument()
    expect(sheet.queryByRole('button', { name: '문항 목록' })).not.toBeInTheDocument()
    await user.type(sheet.getByLabelText('답'), '다음에 만나요.')
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'closing_greeting', body: '다음에 만나요.', uploadId: '' },
      ]),
    )
    expect(
      await sheet.findByText('모든 문항에 답했어요. 지금부터 답에 문장을 더 보태 볼게요.'),
    ).toBeInTheDocument()
    expect(sheet.getByText(/블로그 글을 시작할 때 쓰는 첫인사/)).toBeInTheDocument()
    expect(await sheet.findByLabelText('답')).toHaveValue('안녕하세요.')
    expect(sheet.getByRole('button', { name: '답 고치기' })).toBeInTheDocument()
    expect(sheet.getByText(/문장이 더 필요해요\./)).toBeInTheDocument()
  })

  // VOICE-60, VOICE-32: an answered prompt reopens on its answer; the rewrite replaces it — a
  // photo answer on its own photo — and the meter counts the added sentences.
  it('rewrites an answered prompt with more sentences', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials({
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
    expect(await sheet.findByText('13% 확보')).toBeInTheDocument()
    const field = await sheet.findByLabelText('답')
    expect(field).toHaveValue('안녕하세요. 반가워요.')
    expect(field).toHaveFocus()
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
    expect(await sheet.findByText('15% 확보')).toBeInTheDocument()
    expect(await sheet.findByText(/음식이나 음료 사진/)).toBeInTheDocument()
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
    expect(await sheet.findByText(/가격이나 양, 가성비/)).toBeInTheDocument()
  })

  // VOICE-60: a photo prompt is answered on the owner's own photo, converted and PUT first.
  it('answers a photo prompt on a picked photo', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials(
      {
        answers,
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
    renderMaterials({
      answers,
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
