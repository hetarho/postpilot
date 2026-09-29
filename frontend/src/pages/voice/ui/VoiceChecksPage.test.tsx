import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { initializeI18n } from '@/app/providers/i18n'
import {
  ProtoFingerprintFacetUnit,
  ProtoFingerprintItem,
  ProtoVoiceCheckStatus,
  Stage,
  VoicePromptPart,
} from '@/shared/api'
import { renderAppAt } from '@/test/app'

const CHECKS = '/voices/voice-default/checks'

/** A writer that does not read images, selected for the write stage. */
const WRITER = {
  models: [{ providerId: 'p', modelId: 'w', label: 'Writer', stages: [Stage.WRITE] }],
  selections: [{ stage: Stage.WRITE, providerId: 'p', modelId: 'w' }],
}

const ANSWERED = [
  {
    id: 'answer-1',
    label: '',
    kind: 'answer' as const,
    promptKey: 'closing_greeting',
    body: '다음에 또 만나요~',
  },
]

const share = (value: number) => ({ value: { case: 'number' as const, value } })
const GREETING = {
  key: 'opening_greeting',
  part: VoicePromptPart.OPENING,
  text: '블로그 글을 시작할 때 쓰는 첫인사를 평소처럼 2~5문장으로 써 보세요.',
}

afterEach(() => initializeI18n('ko'))

describe('the 검증 tab', () => {
  // VOICE-54: the third tab, at /checks.
  it('is the third tab of the voice', async () => {
    renderAppAt(CHECKS, { user: { id: 'alice' }, providers: WRITER })

    const link = await screen.findByRole('link', { name: '검증' })
    expect(link).toHaveAttribute('aria-current', 'page')
    // The tab row, in order, ends with 검증.
    const tabs = screen
      .getAllByRole('link')
      .map((tab) => tab.getAttribute('href') ?? '')
      .filter((href) => href.startsWith('/voices/voice-default'))
    expect(tabs).toEqual(['/voices/voice-default', '/voices/voice-default/materials', CHECKS])
    expect(await screen.findByText('아직 검증한 결과가 없어요.')).toBeInTheDocument()
  })

  // VOICE-43: answered prompts come first, and one starts the check at once on the write model.
  it('starts a check from an answered prompt', async () => {
    const user = userEvent.setup()
    const checkStarts: Array<{ voiceId: string; promptKey: string; model: string }> = []
    renderAppAt(CHECKS, {
      user: { id: 'alice' },
      providers: WRITER,
      voice: { samples: ANSWERED, checkStarts },
    })

    await user.click(await screen.findByRole('button', { name: '검증하기' }))
    const sheet = await screen.findByRole('dialog', { name: '검증할 문항' })
    const prompts = within(sheet).getAllByRole('listitem')
    expect(prompts[0]).toHaveTextContent('글을 마무리할 때')
    expect(prompts[0]).toHaveTextContent('답함')

    await user.click(within(prompts[0]).getByRole('button'))
    await waitFor(() =>
      expect(checkStarts).toEqual([
        { voiceId: 'voice-default', promptKey: 'closing_greeting', model: 'p/w' },
      ]),
    )
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(await screen.findByText('검증하는 중이에요.')).toBeInTheDocument()
  })

  // VOICE-43: an unanswered prompt opens its answer form; the check starts once it is saved.
  it('answers an unanswered prompt before checking it', async () => {
    const user = userEvent.setup()
    const checkStarts: Array<{ voiceId: string; promptKey: string; model: string }> = []
    const answers: Array<{ promptKey: string; body: string; uploadId: string }> = []
    renderAppAt(CHECKS, {
      user: { id: 'alice' },
      providers: WRITER,
      voice: { samples: ANSWERED, checkStarts, answers },
    })

    await user.click(await screen.findByRole('button', { name: '검증하기' }))
    const sheet = await screen.findByRole('dialog', { name: '검증할 문항' })
    await user.click(within(sheet).getByRole('button', { name: /블로그 글을 시작할 때/ }))
    await user.type(within(sheet).getByLabelText('답'), '안녕하세요! 오늘도 맛집 이야기예요.')
    await user.click(within(sheet).getByRole('button', { name: '답하기' }))

    await waitFor(() =>
      expect(answers.map((answer) => answer.promptKey)).toEqual(['opening_greeting']),
    )
    await waitFor(() =>
      expect(checkStarts.map((start) => start.promptKey)).toEqual(['opening_greeting']),
    )
  })

  // VOICE-43: a photo prompt is listed disabled, with the reason, while the write model cannot
  // read images.
  it('disables a photo prompt on a write model without vision', async () => {
    const user = userEvent.setup()
    renderAppAt(CHECKS, { user: { id: 'alice' }, providers: WRITER, voice: { samples: ANSWERED } })

    await user.click(await screen.findByRole('button', { name: '검증하기' }))
    const sheet = await screen.findByRole('dialog', { name: '검증할 문항' })
    const photo = within(sheet).getByRole('button', { name: /음식이나 음료 사진/ })
    expect(photo).toBeDisabled()
    expect(photo).toHaveAccessibleDescription('사진을 읽는 작성 모델에서만 검증할 수 있어요.')
  })

  // QUOTA-13: a shared entitlement refusal starts nothing and reads as its typed reason.
  it('renders a refused start as its reason', async () => {
    const user = userEvent.setup()
    renderAppAt(CHECKS, {
      user: { id: 'alice' },
      providers: WRITER,
      voice: {
        samples: ANSWERED,
        checkStartRefusal: {
          reason: 'INSUFFICIENT_CREDITS',
          code: Code.ResourceExhausted,
          params: { required: '3', balance: '1', renews_at: '2026-10-01T00:00:00Z' },
        },
      },
    })

    await user.click(await screen.findByRole('button', { name: '검증하기' }))
    const sheet = await screen.findByRole('dialog', { name: '검증할 문항' })
    await user.click(within(sheet).getByRole('button', { name: /글을 마무리할 때/ }))
    expect(await within(sheet).findByText(/크레딧/)).toBeInTheDocument()
    expect(screen.queryByText('검증하는 중이에요.')).toBeNull()
  })

  // VOICE-55: without a usable write selection, 검증하기 says why in place.
  it('says why without a write model', async () => {
    renderAppAt(CHECKS, { user: { id: 'alice' }, providers: { models: WRITER.models } })

    expect(await screen.findByRole('button', { name: '검증하기' })).toBeDisabled()
    expect(await screen.findByText(/검증에 쓸 작성 모델을 먼저 골라 주세요\./)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'AI 모델 고르기' })).toHaveAttribute(
      'href',
      '/ai-models',
    )
  })

  // VOICE-43, VOICE-44: a result shows 내 답 and AI 글 with the comparison labelled AI 글, an older
  // one is marked, and a failure retries as a new check.
  it('lays out a result, marks an older one and retries a failure', async () => {
    const user = userEvent.setup()
    const checkRetries: Array<{ checkId: string; model: string }> = []
    renderAppAt(CHECKS, {
      user: { id: 'alice' },
      providers: WRITER,
      voice: {
        samples: [
          {
            id: 'answer-2',
            label: '',
            kind: 'answer',
            promptKey: 'opening_greeting',
            body: '안녕하세요',
          },
        ],
        checkRetries,
        checks: [
          {
            id: 'check-failed',
            prompt: GREETING,
            status: ProtoVoiceCheckStatus.FAILED,
            failure: { reason: 'MODEL_RATE_LIMITED' },
          },
          {
            id: 'check-done',
            prompt: GREETING,
            answer: '안녕하세요, 동네 빵집 이야기예요.',
            status: ProtoVoiceCheckStatus.DONE,
            piece: '안녕하세요! 오늘은 빵집이에요.',
            stale: true,
            comparison: [
              {
                item: ProtoFingerprintItem.ENDINGS,
                distance: 0.4,
                headline: '해요',
                facets: [
                  {
                    key: '해요',
                    unit: ProtoFingerprintFacetUnit.SHARE,
                    voice: share(0.9),
                    text: share(0.5),
                  },
                ],
              },
            ],
          },
        ],
      },
    })

    const results = await screen.findAllByRole('article')
    expect(results).toHaveLength(2)
    const done = within(results[1])
    expect(done.getByText('이전 분석으로 검증')).toBeInTheDocument()
    expect(done.getByRole('region', { name: '내 답' })).toHaveTextContent('동네 빵집 이야기예요.')
    expect(done.getByRole('region', { name: 'AI 글' })).toHaveTextContent('오늘은 빵집이에요.')
    expect(done.getByRole('region', { name: '말투 지문' })).toHaveTextContent(
      "'~해요' 내 말투 90% · AI 글 50%",
    )
    expect(within(results[0]).queryByText('이전 분석으로 검증')).toBeNull()

    await user.click(within(results[0]).getByRole('button', { name: '다시 검증' }))
    await waitFor(() => expect(checkRetries).toEqual([{ checkId: 'check-failed', model: 'p/w' }]))
  })

  // VOICE-31: a reload resumes polling the running check through active_job_id.
  it('resumes polling a running check', async () => {
    const calls: string[] = []
    renderAppAt(CHECKS, {
      user: { id: 'alice' },
      calls,
      providers: WRITER,
      voice: { activeCheckJobId: 'check-job' },
      jobs: {
        jobs: [
          {
            id: 'check-job',
            kind: 'check_voice',
            status: 'running',
            stage: 'write',
            progressDone: 0,
            progressTotal: 1,
          },
        ],
      },
    })

    expect(await screen.findByRole('region', { name: '검증 상태' })).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('GetGeneration'))
    expect(screen.getByRole('button', { name: '검증하기' })).toBeDisabled()
  })
})
