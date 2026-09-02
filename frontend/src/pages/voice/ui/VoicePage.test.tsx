import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { BlockType, VoiceLayer, VoiceRuleStatus, VoiceValueSource } from '@/shared/api'
import { renderAppAt } from '@/test/app'

/** A learned profile whose axes are partly unanswered — the state the analysis produces once it
 *  stops fabricating a neutral 0 for an axis the model never addressed. */
const LEARNED = {
  empty: false,
  meta: { version: 3n, sourceCount: 2 },
  lexical: {
    description: { value: '담백한 어휘', source: VoiceValueSource.ANALYZED, unknown: false },
  },
  endings: {
    baseRegister: { value: '해요체', source: VoiceValueSource.MEASURED, unknown: false },
  },
  axes: { involvement: 2 },
}

const DEFAULT = '/voices/voice-default'

afterEach(() => initializeI18n('ko'))

describe('the 프로필 tab', () => {
  // Change 04 A1, now under one voice: the layout names the voice, the tab keeps its title.
  it('renders the profile and none of the other tabs’ panels', async () => {
    renderAppAt(DEFAULT, { user: { id: 'alice' }, voice: { structured: LEARNED } })

    expect(await screen.findByRole('heading', { level: 1, name: '기본 말투' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { level: 2, name: '프로필' })).toBeInTheDocument()
    expect(screen.getByText('현재 말투 프로필')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '검증 시작' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '복원' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('문체 규칙')).not.toBeInTheDocument()
    expect(screen.queryByText('학습 샘플')).not.toBeInTheDocument()
  })

  // Change 04 A4: the three detail lists belong to the tabs that display them.
  it('issues no version, confirmation or validation request on mount', async () => {
    const calls: string[] = []
    renderAppAt(DEFAULT, { user: { id: 'alice' }, calls, voice: { structured: LEARNED } })

    await screen.findByText('현재 말투 프로필')
    await waitFor(() => expect(calls).toContain('GetVoiceProfile'))
    expect(calls).not.toContain('ListVoiceProfileVersions')
    expect(calls).not.toContain('ListRuleConfirmations')
    expect(calls).not.toContain('ListVoiceProfileValidations')
  })

  // Change 04 A11, frontend half: an axis the analysis never answered is not a measurement.
  it('shows an unanswered axis as 알 수 없음 rather than 0', async () => {
    renderAppAt(DEFAULT, { user: { id: 'alice' }, voice: { structured: LEARNED } })

    const axes = (await screen.findByText('여섯 성향 (-3~3)')).closest('section')!
    expect(within(axes).getByText('관여도').nextElementSibling).toHaveTextContent('2')
    expect(within(axes).getByText('서사성').nextElementSibling).toHaveTextContent('알 수 없음')
    expect(within(axes).queryByText('0')).not.toBeInTheDocument()
  })

  it('keeps Korean syntax measurement in characters', async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: {
        structured: { ...LEARNED, syntax: { averageSentenceChars: 14 } },
      },
    })

    const label = await screen.findByText('평균 문장 길이(글자)')
    expect(label.nextElementSibling).toHaveTextContent('14자')
    expect(screen.getByText('주 종결어미')).toBeInTheDocument()
  })

  it('shows English syntax measurement in words and preserves unknown presence', async () => {
    const englishVoice = [
      {
        id: 'voice-default',
        name: 'English voice',
        isDefault: true,
        sourceLanguage: 'en' as const,
      },
    ]
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: {
        voices: englishVoice,
        structured: {
          ...LEARNED,
          syntax: { averageSentenceChars: 99, averageSentenceWords: 12.5 },
        },
      },
    })

    const measured = await screen.findByText('평균 문장 길이(단어)')
    expect(measured.nextElementSibling).toHaveTextContent('12.5단어')
    expect(screen.getByText('기본 문체 격식')).toBeInTheDocument()
    expect(screen.queryByText('주 종결어미')).not.toBeInTheDocument()

    cleanup()
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: {
        voices: englishVoice,
        structured: { ...LEARNED, syntax: { averageSentenceChars: 99 } },
      },
    })

    const unknown = await screen.findByText('평균 문장 길이(단어)')
    expect(unknown.nextElementSibling).toHaveTextContent('알 수 없음')
    expect(unknown.nextElementSibling).not.toHaveTextContent('99')
  })

  // Plan 10 A2: another voice of the same account is genuinely empty.
  it('shows a second voice as empty even while the default has learned', async () => {
    renderAppAt('/voices/voice-review', {
      user: { id: 'alice' },
      voice: {
        structured: LEARNED,
        voices: [
          { id: 'voice-default', name: '기본 말투', isDefault: true },
          { id: 'voice-review', name: '리뷰' },
        ],
      },
    })

    expect(await screen.findByRole('heading', { level: 1, name: '리뷰' })).toBeInTheDocument()
    expect(await screen.findByText(/아직 배운 말투가 없어요/)).toBeInTheDocument()
    expect(screen.queryByText('담백한 어휘')).not.toBeInTheDocument()
  })

  it('says so for a voice the account does not have', async () => {
    renderAppAt('/voices/nope', { user: { id: 'alice' } })

    expect(await screen.findByRole('alert')).toHaveTextContent('없는 말투예요.')
    expect(screen.queryByRole('navigation', { name: '말투 설정' })).not.toBeInTheDocument()
  })

  it('keeps a deleted profile readable but removes its edit affordances', async () => {
    renderAppAt('/voices/voice-default', {
      user: { id: 'alice' },
      voice: {
        structured: LEARNED,
        voices: [{ id: 'voice-default', name: '옛 말투', isDefault: true, deleted: true }],
      },
    })

    expect(await screen.findByText('현재 말투 프로필')).toBeInTheDocument()
    expect(screen.getByText('담백한 어휘')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /수정$/ })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '복원' })).toBeInTheDocument()
  })
})

describe('the voice tab row', () => {
  // Change 04 A2 / A3.
  it('gives every tab an address under the voice and marks the current one', async () => {
    const { router } = renderAppAt(DEFAULT, { user: { id: 'alice' } })

    const tabs = within(await screen.findByRole('navigation', { name: '말투 설정' })).getAllByRole(
      'link',
    )
    expect(tabs.map((tab) => tab.getAttribute('href'))).toEqual([
      DEFAULT,
      `${DEFAULT}/versions`,
      `${DEFAULT}/import`,
      `${DEFAULT}/rules`,
      `${DEFAULT}/validations`,
    ])
    expect(tabs[0]).toHaveAttribute('aria-current', 'page')
    // Change 04 A5, the mechanical half: the row scrolls instead of wrapping or crushing its five
    // Korean labels, and every tab keeps the 44px floor.
    expect(screen.getByRole('navigation', { name: '말투 설정' })).toHaveClass('overflow-x-auto')
    tabs.forEach((tab) => {
      expect(tab).toHaveClass('min-h-11')
      expect(tab).toHaveClass('whitespace-nowrap')
    })

    await userEvent.setup().click(tabs[3])
    await waitFor(() => expect(router.state.location.pathname).toBe(`${DEFAULT}/rules`))
    expect(await screen.findByRole('heading', { level: 2, name: '대조 규칙' })).toBeInTheDocument()

    router.history.back()
    await waitFor(() => expect(router.state.location.pathname).toBe(DEFAULT))
  })

  it.each([
    [`${DEFAULT}/versions`, '버전 기록'],
    [`${DEFAULT}/import`, '기존 글 가져오기'],
    [`${DEFAULT}/rules`, '대조 규칙'],
    [`${DEFAULT}/validations`, '프로필 검증'],
  ])('renders %s as its own screen on reload', async (path, heading) => {
    const { router } = renderAppAt(path, { user: { id: 'alice' } })

    expect(await screen.findByRole('heading', { level: 2, name: heading })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe(path)
    expect(screen.queryByText('현재 말투 프로필')).not.toBeInTheDocument()
  })
})

describe('localized durable voice records', () => {
  it.each([
    { locale: 'ko' as const, label: 'v7 · 완료 · 33%' },
    { locale: 'en' as const, label: 'v7 · Done · 33%' },
  ])('localizes validation status and Intl percentage in $locale', async ({ locale, label }) => {
    initializeI18n(locale)
    renderAppAt(`${DEFAULT}/validations`, {
      user: { id: 'alice' },
      voice: {
        validations: [
          {
            id: 'validation-1',
            voiceId: 'voice-default',
            profileVersion: 7n,
            status: 'done',
            judgeEnabled: true,
            totalCount: 3,
            yCount: 1,
          },
        ],
      },
    })

    expect(await screen.findByRole('link', { name: label })).toBeInTheDocument()
    expect(screen.queryByText('done')).not.toBeInTheDocument()
  })

  it.each([
    { locale: 'ko' as const, layer: '어휘', evidence: '활성 · 근거 1' },
    { locale: 'en' as const, layer: 'Lexical', evidence: 'Active · 1 evidence' },
  ])('localizes the normalized rule layer in $locale', async ({ locale, layer, evidence }) => {
    initializeI18n(locale)
    renderAppAt(`${DEFAULT}/rules`, {
      user: { id: 'alice' },
      voice: {
        structured: {
          ...LEARNED,
          contrastRules: [
            {
              id: 'rule-1',
              statement: 'Keep sentences concise.',
              layer: VoiceLayer.LEXICAL,
              evidenceCount: 1,
              status: VoiceRuleStatus.ACTIVE,
            },
          ],
        },
      },
    })

    expect(await screen.findByText(layer)).toBeInTheDocument()
    expect(screen.getByText(evidence)).toBeInTheDocument()
    expect(screen.queryByText('lexical')).not.toBeInTheDocument()
  })
})

describe('the legacy /voice address', () => {
  // Plan 10: old links resolve the server default; nothing is created on the way.
  it.each([
    ['/voice', DEFAULT],
    ['/voice/rules', `${DEFAULT}/rules`],
    ['/voice/import', `${DEFAULT}/import`],
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

describe('the 기존 글 가져오기 tab', () => {
  it('resumes polling the active analysis exposed by the profile', async () => {
    const calls: string[] = []
    renderAppAt(`${DEFAULT}/import`, {
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
        analysisAfterAnalysis: '# 종결어미\n~다를 자주 사용',
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

    // The analysis lands in the structured profile's lexical description now — there is no
    // free-text styleguide field left for it to appear in (change 16).
    await waitFor(() => expect(screen.getByText(/~다를 자주 사용/)).toBeInTheDocument())
  })

  // Change 16 A9: the 이전 수동 안내 section and both of its editors are gone from every tab.
  it('offers no free-text guidance editors anywhere on the voice screens', async () => {
    renderAppAt(`${DEFAULT}/import`, { user: { id: 'alice' } })
    await screen.findByLabelText('내가 쓴 글')
    expect(screen.queryByText('이전 수동 안내')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('문체 규칙')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('추가 규칙')).not.toBeInTheDocument()
  })

  // Change 16 A7: the paste form's first field says what it is.
  it('labels the imported piece 제목 and keeps it optional', async () => {
    renderAppAt(`${DEFAULT}/import`, { user: { id: 'alice' } })
    expect(await screen.findByLabelText('제목 (선택)')).toBeInTheDocument()
    expect(screen.queryByLabelText('라벨 (선택)')).not.toBeInTheDocument()
  })

  // Change 16 A4/A5/A6: a version is READ before it is taken, and the preview is the confirmation.
  it('opens a version, previews what it wrote, and adopts it without a dialog', async () => {
    const calls: string[] = []
    renderAppAt(`${DEFAULT}/versions`, {
      user: { id: 'alice' },
      calls,
      voice: {
        structured: { meta: { version: 3n }, empty: false },
        versions: [
          { version: 3n, origin: 'analysis', hasSample: true },
          { version: 2n, origin: 'manual', hasSample: true },
          { version: 1n, origin: 'analysis', hasSample: false },
        ],
        versionSamples: {
          '2': {
            title: '비 오는 제주',
            blocks: [{ type: BlockType.TEXT, content: '우산을 두고 나왔다.' }],
          },
        },
      },
    })
    const user = userEvent.setup()

    // The list itself carries no post bodies — presence only.
    await screen.findByRole('button', { name: /v3 · 분석/ })
    expect(calls).not.toContain('GetVoiceProfileVersionSample')

    // The head is openable and offers no way to adopt itself.
    await user.click(screen.getByRole('button', { name: /v3 · 분석/ }))
    expect(screen.queryByRole('button', { name: '이 버전으로 변경' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /v2 · 직접 수정/ }))
    expect(await screen.findByText('비 오는 제주')).toBeInTheDocument()
    expect(screen.getByText('우산을 두고 나왔다.')).toBeInTheDocument()

    // No confirmation dialog stands between the preview and the change: the preview IS it.
    await user.click(screen.getByRole('button', { name: '이 버전으로 변경' }))
    await waitFor(() => expect(calls).toContain('RestoreVoiceProfile'))

    // A version that never produced a post says so, with no empty preview box.
    await user.click(screen.getByRole('button', { name: /v1 · 분석/ }))
    expect(await screen.findByText('이 버전으로 쓴 글이 아직 없어요.')).toBeInTheDocument()
  })

  // Change 14 A7: the tab a described create lands on reports the seeding run.
  it('shows the seeding run started by a described creation', async () => {
    const calls: string[] = []
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      calls,
      voice: { activeJobId: 'seed-job' },
      jobs: {
        jobs: [
          {
            id: 'seed-job',
            kind: 'seed_voice',
            status: 'running',
            stage: 'seed',
            progressDone: 0,
            progressTotal: 1,
          },
        ],
      },
    })

    await screen.findByRole('heading', { level: 2, name: '프로필' })
    await waitFor(() => expect(calls).toContain('GetGeneration'))
    expect(screen.getByRole('region', { name: '문체 분석 상태' })).toBeInTheDocument()
  })

  // Change 14 A11: a failed seed leaves a usable voice and says why, on that same tab.
  it('reports a failed seed without losing the voice', async () => {
    renderAppAt(DEFAULT, {
      user: { id: 'alice' },
      voice: { activeJobId: 'seed-job' },
      jobs: {
        jobs: [
          {
            id: 'seed-job',
            kind: 'seed_voice',
            status: 'failed',
            failureReason: 'PROVIDER_DISABLED',
          },
        ],
      },
    })

    expect(await screen.findByRole('heading', { level: 1, name: '기본 말투' })).toBeInTheDocument()
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    // The profile is simply empty; nothing partial was written.
    expect(screen.getByText('현재 말투 프로필')).toBeInTheDocument()
  })

  // Change 14 A5: renaming lives on the voice, not on the directory row that leads here.
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

  // Change 14 A5: a tombstone stays renameable, which is how a restore conflict is resolved.
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

  // Plan 10 A5/A7: a tombstone is readable, and the import is refused before the paste.
  it('shows a deleted voice as a tombstone and blocks importing into it', async () => {
    renderAppAt('/voices/voice-old/import', {
      user: { id: 'alice' },
      voice: {
        voices: [
          { id: 'voice-default', name: '기본 말투', isDefault: true },
          { id: 'voice-old', name: '옛 말투', deleted: true },
        ],
      },
    })

    expect(await screen.findByRole('heading', { level: 1, name: '옛 말투' })).toBeInTheDocument()
    expect(screen.getByText('삭제됨')).toBeInTheDocument()
    expect(screen.getByText(/삭제된 말투예요\. 기록은 볼 수 있지만/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '복원' })).toBeInTheDocument()
    expect(await screen.findByText(/삭제된 말투에는 글을 가져올 수 없어요/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '학습' })).toBeDisabled()
  })
})
