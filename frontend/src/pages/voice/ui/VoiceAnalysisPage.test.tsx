import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'

/** A made voice's analysis with one unknown item and the owner's own examples (VOICE-63). */
const ANALYSIS = {
  counted: {
    sentences: 120,
    endings: {
      da: 0.28,
      haeyo: 0.31,
      seumnida: 0.07,
      other: 0.34,
      suffixes: [{ text: '더라구요', count: 9 }],
    },
    marks: {
      exclaim: 0.32,
      question: 0.05,
      tilde: 0.1,
      period: 0.4,
      repeat: 0.08,
      example: { sentence: '정말 맛있었어요!', materialId: 'sample-1' },
    },
    emoji: { unknown: true },
    shape: { averageChars: 24, paragraphMin: 1, paragraphMax: 3, ownLine: true },
    openings: { openings: ['안녕하세요!'], closings: ['다음에 또 만나요~'] },
    adverbs: { none: true },
    person: {
      jeo: 6,
      dominant: '저',
      example: { sentence: '저는 또 갈 거예요.', materialId: 'gone' },
    },
    headings: { count: 4, emojiShare: 0.5, questionShare: 0.25, marker: '-' },
  },
  ai: {
    impression: '들뜬 목소리로 친구에게 말하듯 써요.',
    tics: [{ phrase: '진짜', when: '맛에 감탄할 때' }],
    signaturePhrases: ['완전 추천'],
    examples: [{ field: 1, sentence: '진짜 대박이었어요.', materialId: 'sample-1' }],
  },
  materialCount: 3,
}

const DEFAULT = '/voices/voice-default'

afterEach(() => initializeI18n('ko'))

describe('the 말투 분석 tab', () => {
  // VOICE-63: 숫자로 본 습관 in eight rows, 알 수 없음 where unknown, each with its example, then
  // AI가 읽은 인상; nothing to edit.
  it("reads the analysis back in two groups with the owner's own sentences", async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: { analysis: ANALYSIS, samples: [{ id: 'sample-1', label: '국숫집' }] },
    })

    const counted = within(await screen.findByRole('region', { name: '숫자로 본 습관' }))
    const rows = counted.getAllByRole('listitem')
    expect(rows).toHaveLength(8)
    expect(rows[1]).toHaveTextContent('문장의 32%를 느낌표로')
    expect(rows[1]).toHaveTextContent('“정말 맛있었어요!”')
    expect(rows[2]).toHaveTextContent('알 수 없음')
    expect(rows[5]).toHaveTextContent('눈에 띄게 반복하는 부사는 없어요.')
    // VOICE-21: the 학습 글 the first-person example came from is gone, and so is its sentence.
    expect(rows[6]).toHaveTextContent('저')
    expect(rows[6]).not.toHaveTextContent('저는 또 갈 거예요.')
    expect(rows[0]).toHaveTextContent("'~다'")
    expect(rows[0]).toHaveTextContent('~더라구요')
    const ai = within(screen.getByRole('region', { name: 'AI가 읽은 인상' }))
    expect(ai.getByText('들뜬 목소리로 친구에게 말하듯 써요.')).toBeInTheDocument()
    expect(ai.getByText('“진짜 대박이었어요.”')).toBeInTheDocument()
    expect(ai.getByText('‘진짜’ — 맛에 감탄할 때')).toBeInTheDocument()
    // Read-only: no field to edit, no provenance badge.
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByText('측정값')).not.toBeInTheDocument()
  })

  // VOICE-63: until the voice is made, the meter, the way to 학습 글 and 말투 만들기.
  it('shows the meter and the way to 학습 글 for a voice not yet made', async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: { voices: [{ id: 'voice-default', name: '기본 말투', made: false }] },
    })

    expect(await screen.findByText('말투 학습에 필요한 정보')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '학습 글 모으기' })).toHaveAttribute(
      'href',
      `${DEFAULT}/materials`,
    )
    expect(screen.getByRole('button', { name: '말투 만들기' })).toBeDisabled()
    expect(screen.queryByRole('region', { name: '숫자로 본 습관' })).not.toBeInTheDocument()
  })

  // VOICE-21: the notice sits above the groups with 다시 분석.
  it('says the 학습 글 changed, with 다시 분석, above the groups', async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: { analysis: ANALYSIS, notice: { kind: 'added', count: 2 } },
    })
    expect(await screen.findByText('새 학습 글 2편')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 분석' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '이전 분석으로 되돌리기' })).not.toBeInTheDocument()
  })

  // VOICE-30: 이전 분석으로 되돌리기 returns to the previous analysis once.
  it('returns to the previous analysis after the sheet says there is no redo', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      calls,
      voice: {
        analysis: ANALYSIS,
        previousAnalysis: { ...ANALYSIS, ai: { ...ANALYSIS.ai, impression: '담담한 말투예요.' } },
      },
    })

    await user.click(await screen.findByRole('button', { name: '이전 분석으로 되돌리기' }))
    const sheet = await screen.findByRole('dialog', { name: '이전 분석으로 되돌릴까요?' })
    await user.click(within(sheet).getByRole('button', { name: '이전 분석으로 되돌리기' }))

    await waitFor(() => expect(calls).toContain('RestorePreviousVoiceAnalysis'))
    expect(await screen.findByText('담담한 말투예요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '이전 분석으로 되돌리기' })).not.toBeInTheDocument()
  })

  it('says so for a voice the account does not have', async () => {
    renderAppAt('/voices/voice-missing', { user: { id: 'alice' } })
    expect(await screen.findByText('없는 말투예요.')).toBeInTheDocument()
  })

  it("keeps a deleted voice's analysis readable without its undo", async () => {
    renderAppAt('/voices/voice-old', {
      user: { id: 'alice' },
      voice: {
        voices: [
          { id: 'voice-default', name: '기본 말투' },
          { id: 'voice-old', name: '옛 말투', deleted: true },
        ],
      },
    })
    expect(await screen.findByRole('region', { name: '숫자로 본 습관' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '이전 분석으로 되돌리기' })).not.toBeInTheDocument()
  })
})

describe('the voice tab row', () => {
  // VOICE-54.
  it('gives every tab an address under the voice and marks the current one', async () => {
    const { router } = renderAppAt(DEFAULT, { user: { id: 'alice' } })

    const tabs = within(await screen.findByRole('navigation', { name: '말투 설정' })).getAllByRole(
      'link',
    )
    // Three tabs: the 규칙 and 검증 tabs left with contrast rules and profile validation.
    expect(tabs.map((tab) => tab.getAttribute('href'))).toEqual([DEFAULT, `${DEFAULT}/materials`])
    expect(tabs[0]).toHaveAttribute('aria-current', 'page')
    // THEME-29, the mechanical half: the row scrolls instead of wrapping or crushing its Korean
    // labels, and every tab keeps the 44px floor.
    expect(screen.getByRole('navigation', { name: '말투 설정' })).toHaveClass('overflow-x-auto')
    tabs.forEach((tab) => {
      expect(tab).toHaveClass('min-h-10', 'pointer-coarse:min-h-11')
      expect(tab).toHaveClass('whitespace-nowrap')
    })

    await userEvent.setup().click(tabs[1])
    await waitFor(() => expect(router.state.location.pathname).toBe(`${DEFAULT}/materials`))
    expect(await screen.findByRole('heading', { level: 2, name: '학습 글' })).toBeInTheDocument()

    router.history.back()
    await waitFor(() => expect(router.state.location.pathname).toBe(DEFAULT))
  })

  it.each([[`${DEFAULT}/materials`, '학습 글']])(
    'renders %s as its own screen on reload',
    async (path, heading) => {
      const { router } = renderAppAt(path, { user: { id: 'alice' } })

      expect(await screen.findByRole('heading', { level: 2, name: heading })).toBeInTheDocument()
      expect(router.state.location.pathname).toBe(path)
      expect(screen.queryByRole('region', { name: '숫자로 본 습관' })).not.toBeInTheDocument()
    },
  )
})

// VOICE-12, VOICE-13, VOICE-54: the title row carries the rename, 기본으로 설정 or 기본 해제 on
// a made voice, and 삭제 confirmed by a sheet saying what stays.
describe('the voice title row', () => {
  const VOICES = [
    { id: 'voice-default', name: '기본 말투', isDefault: true },
    { id: 'voice-review', name: '리뷰' },
    { id: 'voice-new', name: '새 말투', made: false },
  ]

  it('makes a made voice the 기본 and clears it again', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/voices/voice-review', { user: { id: 'alice' }, calls, voice: { voices: VOICES } })

    await screen.findByRole('heading', { level: 1, name: '리뷰' })
    await user.click(screen.getByRole('button', { name: '기본으로 설정' }))
    await waitFor(() => expect(calls).toContain('SetDefaultVoice'))
    await user.click(await screen.findByRole('button', { name: '기본 해제' }))
    await waitFor(() => expect(calls.filter((call) => call === 'SetDefaultVoice')).toHaveLength(2))
    expect(await screen.findByRole('button', { name: '기본으로 설정' })).toBeInTheDocument()
  })

  it('offers no 기본 on a voice not yet made, but still deletes it', async () => {
    renderAppAt('/voices/voice-new', { user: { id: 'alice' }, voice: { voices: VOICES } })

    await screen.findByRole('heading', { level: 1, name: '새 말투' })
    expect(screen.queryByRole('button', { name: '기본으로 설정' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '기본 해제' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투 삭제' })).toBeInTheDocument()
    // No language badge anywhere on the title row (VOICE-10).
    expect(screen.queryByText('한국어')).not.toBeInTheDocument()
  })

  it('deletes the 기본 after the sheet says what stays', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(DEFAULT, { user: { id: 'alice' }, calls, voice: { voices: VOICES } })

    await screen.findByRole('heading', { level: 1, name: '기본 말투' })
    expect(screen.getByRole('button', { name: '기본 해제' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '기본 말투 삭제' }))
    const sheet = await screen.findByRole('dialog')
    expect(sheet).toHaveTextContent(
      '글은 그대로 남고 이 말투는 삭제된 말투로 표시돼요. 학습 글과 분석은 함께 보관되고, 복원하면 다시 쓸 수 있어요.',
    )
    await user.click(within(sheet).getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(calls).toContain('DeleteVoice'))
    expect(await screen.findByRole('button', { name: '복원' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '기본 해제' })).not.toBeInTheDocument()
  })

  it('explains a refused delete instead of erasing anything', async () => {
    const user = userEvent.setup()
    renderAppAt('/voices/voice-review', {
      user: { id: 'alice' },
      voice: { voices: VOICES, busyVoices: ['voice-review'] },
    })

    await user.click(await screen.findByRole('button', { name: '리뷰 삭제' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '삭제' }),
    )

    expect(await screen.findByRole('alert')).toHaveTextContent('지금은 삭제할 수 없어요')
    expect(screen.getByRole('alert')).toHaveTextContent('이 말투에서 작업이 진행 중이에요.')
    expect(screen.queryByRole('button', { name: '복원' })).not.toBeInTheDocument()
  })
})

describe('the legacy /voice address', () => {
  // VOICE-54: old links resolve the server default; nothing is created on the way.
  it.each([
    ['/voice', DEFAULT],
    ['/voice/materials', `${DEFAULT}/materials`],
    // The old 가져오기 and 버전 기록 tabs are gone and land on 말투 분석, like any that never existed.
    ['/voice/import', DEFAULT],
    ['/voice/versions', DEFAULT],
    // A tab that no longer exists lands on the profile, like one that never did.
    ['/voice/rules', DEFAULT],
    ['/voice/whatever', DEFAULT],
  ])('redirects %s to %s', async (from, to) => {
    const calls: string[] = []
    const { router } = renderAppAt(from, { user: { id: 'alice' }, calls })

    await waitFor(() => expect(router.state.location.pathname).toBe(to))
    expect(calls).not.toContain('CreateVoice')
    expect(calls).not.toContain('SetDefaultVoice')
  })

  it('follows the account’s actual default, not the first voice', async () => {
    const { router } = renderAppAt('/voice', {
      user: { id: 'alice' },
      voice: {
        voices: [
          { id: 'voice-a', name: '가' },
          { id: 'voice-b', name: '나', isDefault: true },
        ],
      },
    })

    await waitFor(() => expect(router.state.location.pathname).toBe('/voices/voice-b'))
  })
})

describe('the 학습 글 tab', () => {
  it('resumes polling the active analysis exposed by the profile', async () => {
    const calls: string[] = []
    renderAppAt(`${DEFAULT}/materials`, {
      user: { id: 'alice' },
      calls,
      voice: { activeJobId: 'voice-job' },
      jobs: {
        jobs: [
          {
            id: 'voice-job',
            kind: 'analyze_voice',
            status: 'running',
            stage: 'analyze',
            progressDone: 0,
            progressTotal: 1,
          },
        ],
      },
    })

    expect(await screen.findByText('문체 분석 중')).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('GetGeneration'))
  })

  it('refreshes the profile when the resumed analysis is already done', async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: {
        activeJobId: 'voice-job',
        analysisAfterAnalysis: '~다를 자주 쓰는 담백한 말투예요.',
      },
      jobs: {
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
    })

    // The finished analysis is read back once the job is done (VOICE-31).
    await waitFor(() =>
      expect(screen.getByText('~다를 자주 쓰는 담백한 말투예요.')).toBeInTheDocument(),
    )
  })

  // VOICE-6: the 이전 수동 안내 section and both of its editors are gone from every tab.
  it('offers no free-text guidance editors anywhere on the voice screens', async () => {
    renderAppAt(`${DEFAULT}/materials`, { user: { id: 'alice' } })
    await screen.findByRole('heading', { level: 2, name: '학습 글' })
    expect(screen.queryByText('이전 수동 안내')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('문체 규칙')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('추가 규칙')).not.toBeInTheDocument()
  })

  // VOICE-64: the paste form's first field says what it is.
  it('labels the pasted piece 제목 and keeps it optional', async () => {
    const user = userEvent.setup()
    renderAppAt(`${DEFAULT}/materials`, { user: { id: 'alice' } })
    await user.click(await screen.findByRole('button', { name: '글 붙여넣기' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(sheet.getAllByRole('textbox')[0]).toBe(sheet.getByLabelText('제목 (선택)'))
    expect(sheet.queryByLabelText('라벨 (선택)')).not.toBeInTheDocument()
  })

  // VOICE-31: the voice's queued or running analysis reports on its tab.
  it('shows the running analysis of the voice', async () => {
    const calls: string[] = []
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      calls,
      voice: { activeJobId: 'analysis-job' },
      jobs: {
        jobs: [
          {
            id: 'analysis-job',
            kind: 'analyze_voice',
            status: 'running',
            stage: 'analyze',
            progressDone: 0,
            progressTotal: 1,
          },
        ],
      },
    })

    await screen.findByRole('heading', { level: 2, name: '말투 분석' })
    await waitFor(() => expect(calls).toContain('GetGeneration'))
    expect(screen.getByRole('region', { name: '문체 분석 상태' })).toBeInTheDocument()
  })

  // VOICE-45: a failed analysis leaves the voice as it was and says why, on that same tab.
  it('reports a failed analysis without losing the voice', async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: { activeJobId: 'analysis-job' },
      jobs: {
        jobs: [
          {
            id: 'analysis-job',
            kind: 'analyze_voice',
            status: 'failed',
            failureReason: 'PROVIDER_DISABLED',
          },
        ],
      },
    })

    expect(await screen.findByRole('heading', { level: 1, name: '기본 말투' })).toBeInTheDocument()
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    // The analysis stays as it was; nothing partial was published.
    expect(screen.getByRole('region', { name: '숫자로 본 습관' })).toBeInTheDocument()
  })

  // VOICE-54: renaming lives on the voice, not on the directory row that leads here.
  it('renames the voice from its own screen', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(DEFAULT, { user: { id: 'alice' }, calls })

    await screen.findByRole('heading', { level: 1, name: '기본 말투' })
    await user.click(screen.getByRole('button', { name: '기본 말투 이름 바꾸기' }))
    const field = screen.getByLabelText('말투 이름')
    expect(field).toHaveValue('기본 말투')
    await user.clear(field)
    await user.type(field, '일상 말투')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('RenameVoice'))
    expect(await screen.findByRole('heading', { level: 1, name: '일상 말투' })).toBeInTheDocument()
    expect(screen.queryByLabelText('말투 이름')).not.toBeInTheDocument()
  })

  // VOICE-54: a tombstone stays renameable, which is how a restore conflict is resolved.
  it('keeps a deleted voice renameable', async () => {
    renderAppAt('/voices/voice-old', {
      user: { id: 'alice' },
      voice: {
        voices: [
          { id: 'voice-default', name: '기본 말투', isDefault: true },
          { id: 'voice-old', name: '옛 말투', deleted: true },
        ],
      },
    })

    expect(await screen.findByRole('button', { name: '옛 말투 이름 바꾸기' })).toBeInTheDocument()
  })

  // VOICE-15, VOICE-54: a tombstone is readable, and the import is refused before the paste.
  it('shows a deleted voice as a tombstone and blocks adding 학습 글 to it', async () => {
    renderAppAt('/voices/voice-old/materials', {
      user: { id: 'alice' },
      voice: {
        voices: [
          { id: 'voice-default', name: '기본 말투', isDefault: true },
          { id: 'voice-old', name: '옛 말투', deleted: true },
        ],
      },
    })

    expect(await screen.findByRole('heading', { level: 1, name: '옛 말투' })).toBeInTheDocument()
    expect(screen.getByText(/삭제된 말투예요\. 기록은 볼 수 있지만/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '복원' })).toBeInTheDocument()
    expect(await screen.findByText(/삭제된 말투에는 학습 글을 더할 수 없어요/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '글 붙여넣기' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '문항 풀기' })).toBeDisabled()
  })
})
