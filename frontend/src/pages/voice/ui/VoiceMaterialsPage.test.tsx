import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt, type RenderAppOptions } from '@/test/app'
import type { FakeVoiceOptions } from '@/test/voice'

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

describe('the 학습 글 tab', () => {
  // VOICE-32: the meter counts sentences and names a missing part; 100% says so.
  it('shows the readiness meter with its missing parts', async () => {
    renderMaterials({
      samples: [
        { id: 'a', label: '', kind: 'answer', promptKey: 'opening_greeting', body: sentences(30) },
      ],
    })
    expect(await screen.findByText('말투 학습에 필요한 정보')).toBeInTheDocument()
    expect(screen.getByText('50% 확보')).toBeInTheDocument()
    expect(screen.getByText('아직 없는 부분: 본문 · 마무리')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 만들기' })).toBeDisabled()
    expect(
      screen.getByText('말투 학습에 필요한 정보가 100%가 되면 누를 수 있어요.'),
    ).toBeInTheDocument()
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

  // VOICE-20: 글 붙여넣기 takes 200 characters, then the 학습 글 lists the post; nothing starts.
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
  })

  // VOICE-60: 문항 풀기 lists the prompts in their groups, answers one, and marks it answered.
  it('answers a prompt and marks it answered', async () => {
    const user = userEvent.setup()
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials({ answers })

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    for (const group of ['글머리', '본문', '마무리']) {
      expect(await sheet.findByRole('heading', { name: group })).toBeInTheDocument()
    }
    await user.click(sheet.getByRole('button', { name: /글을 마무리할 때 쓰는 끝인사/ }))
    await user.type(sheet.getByLabelText('답'), '다음에 또 만나요!')
    await user.click(sheet.getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers).toEqual([
        { promptKey: 'closing_greeting', body: '다음에 또 만나요!', uploadId: '' },
      ]),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: '문항 풀기' }))
    const again = within(await screen.findByRole('dialog'))
    expect(
      await again.findByRole('button', { name: /글을 마무리할 때 쓰는 끝인사.*답함/ }),
    ).toBeDisabled()
  })

  // VOICE-60: a photo prompt is answered on the owner's own photo, converted and PUT first.
  it('answers a photo prompt on a picked photo', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const answers: NonNullable<FakeVoiceOptions['answers']> = []
    renderMaterials({ answers }, calls)

    await user.click(await screen.findByRole('button', { name: '문항 풀기' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.click(await sheet.findByRole('button', { name: /음식이나 음료 사진/ }))
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
  })
})
