import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { emptyVoice, type VoiceProfile, type VoicePrompt, type VoiceSample } from '@/entities/voice'
import { Button } from '@/shared/ui'
import { VoiceQuestionnaire } from './VoiceQuestionnaire'
import { AnswerForm } from './AnswerForm'

const fake = vi.hoisted(() => ({
  prompts: [] as VoicePrompt[],
  answer: vi.fn(),
  upload: vi.fn(),
  put: vi.fn(),
}))
vi.mock('@/entities/voice', async (original) => ({
  ...(await original<typeof import('@/entities/voice')>()),
  useVoicePrompts: () => ({
    prompts: fake.prompts,
    isPending: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useAnswerVoicePrompt: () => ({
    answer: fake.answer,
    isPending: false,
    isError: false,
    errorMessage: '',
  }),
  useVoicePhotoUpload: () => ({ create: fake.upload }),
  useVoiceSample: () => ({ detail: undefined, isError: false, refetch: vi.fn() }),
}))
vi.mock('../api/photo', () => ({
  preparePhoto: vi.fn(async () => ({ blob: new Blob(['jpeg']), width: 1024, height: 768 })),
  putPhoto: fake.put,
}))

function profile(saved: string[] = [], made = false): VoiceProfile {
  const samples: VoiceSample[] = saved.map((key, index) => ({
    id: `sample-${index}`,
    kind: 'answer',
    label: '',
    promptKey: key,
    hasPhoto: false,
    chars: 12,
    createdAt: '',
  }))
  return {
    voice: { ...emptyVoice(), id: 'voice', made },
    made,
    readiness: {
      percent: saved.length >= 10 ? 100 : 0,
      answeredQuestions: saved.length,
      requiredQuestions: 10,
      sentences: 0,
      needed: 60,
      missingParts: [],
    },
    samples,
    activeJobId: '',
    hasPrevious: false,
    notice: { kind: 'none', count: 0 },
  }
}

beforeEach(() => {
  initializeI18n('ko')
  fake.prompts = Array.from({ length: 240 }, (_, index) => ({
    key: `question-${index}`,
    text: `${index + 1}번째 상황을 친구에게 말해 볼까요?`,
    scene: '친구가 오늘 기분이 어땠는지 물었어요.',
    hint: '기억에 남은 장면 하나만 편하게 이야기해 주세요.',
    part: index % 3 === 0 ? 'opening' : index % 3 === 1 ? 'description' : 'closing',
    photo: false,
    starter: index < 10,
  }))
  fake.answer.mockReset().mockImplementation(async (input) => ({
    sample: { id: 'saved-answer', promptKey: input.promptKey },
  }))
  fake.upload.mockReset().mockResolvedValue({
    uploadId: 'private-upload',
    putUrl: 'https://bucket.test',
    contentType: 'image/jpeg',
  })
  fake.put.mockReset().mockResolvedValue(undefined)
})
afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

function mount(current = profile(), ownerId = 'alice', onAnalyze = vi.fn()) {
  const props = {
    ownerId,
    voiceId: 'voice',
    samples: current.samples,
    profile: current,
    renderMakeVoice: () => (
      <Button variant="cta" onClick={onAnalyze}>
        말투 만들기
      </Button>
    ),
  }
  return { ...render(<VoiceQuestionnaire {...props} />), props, onAnalyze }
}
async function answer(
  user: ReturnType<typeof userEvent.setup>,
  text = '친구에게 평소처럼 말했어요.',
) {
  const field = screen.getByLabelText('답')
  await user.clear(field)
  await user.type(field, text)
  await user.click(screen.getByRole('button', { name: '답하기' }))
}

describe('reusable voice questionnaire', () => {
  it('hands ten confirmed answers to review without exposing or calling analysis in the collection step', async () => {
    const current = profile(Array.from({ length: 10 }, (_, index) => `question-${index}`))
    const review = vi.fn()
    const analyze = vi.fn()
    render(
      <VoiceQuestionnaire
        ownerId="alice"
        voiceId="voice"
        samples={current.samples}
        profile={current}
        onReview={review}
        renderMakeVoice={() => <Button onClick={analyze}>말투 만들기</Button>}
      />,
    )
    const user = userEvent.setup()
    expect(await screen.findByText('열 개의 답변이 모였어요.')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('button', { name: '말투 만들기' })).toBeNull()
    expect(screen.queryByRole('button', { name: '질문 10개 더 답하기' })).toBeNull()
    await user.click(screen.getByRole('button', { name: '저장한 답변 확인하기' }))
    expect(review).toHaveBeenCalledOnce()
    expect(analyze).not.toHaveBeenCalled()
    expect(fake.answer).not.toHaveBeenCalled()
  })

  it('keeps the actor and unsaved answer when its collection view is hidden and shown again', async () => {
    const { props, rerender } = mount()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('답'), '돌아와서 이어 쓸 내 답변이에요.')
    rerender(<VoiceQuestionnaire {...props} active={false} />)
    rerender(<VoiceQuestionnaire {...props} active />)
    expect(screen.getByLabelText('답')).toHaveValue('돌아와서 이어 쓸 내 답변이에요.')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(fake.answer).not.toHaveBeenCalled()
  })

  it('shows friendly scene/hint and stops at ten instead of rendering or exhausting hundreds of forms', async () => {
    const current = profile()
    current.readiness.percent = 100
    const { onAnalyze } = mount(current)
    const user = userEvent.setup()
    expect(await screen.findByText('친구가 오늘 기분이 어땠는지 물었어요.')).toBeInTheDocument()
    expect(screen.getByText('기억에 남은 장면 하나만 편하게 이야기해 주세요.')).toBeInTheDocument()
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
    expect(screen.queryByRole('button', { name: '말투 만들기' })).toBeNull()
    expect(document.activeElement?.tagName).not.toBe('TEXTAREA')
    for (let index = 0; index < 10; index++) await answer(user)
    expect(await screen.findByText('열 개의 답변이 모였어요.')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '10')
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(fake.answer).toHaveBeenCalledTimes(10)
    expect(onAnalyze).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: '말투 만들기' }))
    expect(onAnalyze).toHaveBeenCalledOnce()
  })

  it('resumes personal learning from saved keys and starts another ten without replacing earlier answers', async () => {
    mount(profile(['question-0', 'question-1', 'question-2']))
    const user = userEvent.setup()
    expect(await screen.findByText('4번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '3')
    for (let index = 0; index < 7; index++) await answer(user)
    await user.click(await screen.findByRole('button', { name: '답변을 더 보태거나 고치기' }))
    await user.click(screen.getByRole('button', { name: '질문 10개 더 답하기' }))
    expect(screen.getByText('11번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(fake.answer.mock.calls.map(([input]) => input.promptKey)).toEqual(
      Array.from({ length: 7 }, (_, index) => `question-${index + 3}`),
    )
  })

  it('uses ten unanswered questions for a made voice without treating existing answers as this session', async () => {
    mount(
      profile(
        Array.from({ length: 15 }, (_, index) => `question-${index}`),
        true,
      ),
    )
    const user = userEvent.setup()
    expect(await screen.findByText('16번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
    for (let index = 0; index < 10; index++) await answer(user)
    expect(await screen.findByText('열 개의 답변이 모였어요.')).toBeInTheDocument()
    expect(fake.answer.mock.calls.map(([input]) => input.promptKey)).toEqual(
      Array.from({ length: 10 }, (_, index) => `question-${index + 15}`),
    )
  })

  it('retains unsaved text across Back and counts an explicitly changed answer only once', async () => {
    mount()
    const user = userEvent.setup()
    await screen.findByLabelText('답')
    await answer(user, '첫 답변이에요.')
    await user.type(screen.getByLabelText('답'), '둘째 답변을 쓰는 중이에요.')
    await user.click(screen.getByRole('button', { name: '이전 질문' }))
    expect(screen.getByLabelText('답')).toHaveValue('첫 답변이에요.')
    await user.type(screen.getByLabelText('답'), ' 문장을 보탰어요.')
    await user.click(screen.getByRole('button', { name: '답 고치기' }))
    expect(screen.getByLabelText('답')).toHaveValue('둘째 답변을 쓰는 중이에요.')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1')
  })

  it('saves once under double submit, blocks Back/skip while pending and preserves a refused answer', async () => {
    let reject!: (reason: Error) => void
    fake.answer.mockImplementationOnce(
      () =>
        new Promise((_, refuse) => {
          reject = refuse
        }),
    )
    mount()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('답'), '연결이 끊겨도 남아 있어요.')
    await user.dblClick(screen.getByRole('button', { name: '답하기' }))
    expect(fake.answer).toHaveBeenCalledOnce()
    expect(screen.getByRole('button', { name: '다른 질문으로 바꾸기' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '이전 질문' })).toBeDisabled()
    reject(new Error('offline'))
    await waitFor(() => expect(screen.getByRole('button', { name: '답하기' })).toBeEnabled())
    expect(screen.getByLabelText('답')).toHaveValue('연결이 끊겨도 남아 있어요.')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    await user.click(screen.getByRole('button', { name: '답하기' }))
    expect(await screen.findByText('2번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
  })

  it('replaces a skipped question without an RPC or a completed-answer count', async () => {
    mount()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('답'), '아직 떠오르지 않아요.')
    await user.click(screen.getByRole('button', { name: '다른 질문으로 바꾸기' }))
    expect(screen.getByText('4번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(fake.answer).not.toHaveBeenCalled()
  })

  it('never advances on an empty or mismatched server acknowledgement', async () => {
    fake.answer.mockResolvedValueOnce({
      sample: { id: 'another-answer', promptKey: 'question-99' },
    })
    mount()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('답'), '저장 확인 전에는 남아 있어요.')
    await user.click(screen.getByRole('button', { name: '답하기' }))
    expect(await screen.findByText(/답변이 저장됐는지 확인하지 못했어요/)).toBeInTheDocument()
    expect(screen.getByLabelText('답')).toHaveValue('저장 확인 전에는 남아 있어요.')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(screen.getByText('1번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
  })

  it('ignores a saved response after the inline form moves to another account', async () => {
    let resolve!: () => void
    fake.answer.mockImplementationOnce(
      () =>
        new Promise<void>((done) => {
          resolve = done
        }),
    )
    const { rerender, props } = mount()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('답'), '앨리스의 답이에요.')
    await user.click(screen.getByRole('button', { name: '답하기' }))
    rerender(<VoiceQuestionnaire {...props} ownerId="bob" />)
    await screen.findByLabelText('답')
    resolve()
    await waitFor(() =>
      expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0'),
    )
    expect(screen.getByLabelText('답')).toHaveValue('')
    expect(screen.getByText('1번째 상황을 친구에게 말해 볼까요?')).toBeInTheDocument()
  })

  it('retains the chosen private photo and text after a legacy photo upload fails', async () => {
    fake.put.mockRejectedValueOnce(new Error('offline'))
    const done = vi.fn()
    render(
      <AnswerForm
        ownerId="alice"
        voiceId="voice"
        prompt={{ ...fake.prompts[0]!, photo: true }}
        onBack={vi.fn()}
        onDone={done}
      />,
    )
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('답'), '사진 속 빵이 맛있어 보여요.')
    await user.upload(
      screen.getByLabelText('사진 고르기'),
      new File(['x'], 'bread.heic', { type: 'image/heic' }),
    )
    await screen.findByRole('img', { name: '고른 사진' })
    await user.click(screen.getByRole('button', { name: '답하기' }))
    expect(
      await screen.findByText('사진을 올리지 못했어요. 다시 시도해 주세요.'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('답')).toHaveValue('사진 속 빵이 맛있어 보여요.')
    expect(screen.getByRole('img', { name: '고른 사진' })).toBeInTheDocument()
    expect(fake.answer).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: '답하기' }))
    await waitFor(() => expect(done).toHaveBeenCalledOnce())
    expect(fake.answer).toHaveBeenCalledWith(
      expect.objectContaining({
        promptKey: 'question-0',
        photo: { uploadId: 'private-upload', width: 1024, height: 768 },
      }),
    )
  })
})
