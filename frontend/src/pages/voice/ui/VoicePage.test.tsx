import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { BlockType, VoiceValueSource } from '@/shared/api'
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
  // VOICE-54: the layout names the voice, the tab keeps its title.
  it('renders the profile and none of the other tabs’ panels', async () => {
    renderAppAt(DEFAULT, { user: { id: 'alice' }, voice: { structured: LEARNED } })

    expect(await screen.findByRole('heading', { level: 1, name: '기본 말투' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { level: 2, name: '프로필' })).toBeInTheDocument()
    expect(screen.getByText('현재 말투 프로필')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '복원' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('문체 규칙')).not.toBeInTheDocument()
    expect(screen.queryByText('학습 샘플')).not.toBeInTheDocument()
  })

  // VOICE-54: the version list belongs to the tab that displays it.
  it('issues no version request on mount', async () => {
    const calls: string[] = []
    renderAppAt(DEFAULT, { user: { id: 'alice' }, calls, voice: { structured: LEARNED } })

    await screen.findByText('현재 말투 프로필')
    await waitFor(() => expect(calls).toContain('GetVoiceProfile'))
    expect(calls).not.toContain('ListVoiceProfileVersions')
  })

  // VOICE-27, frontend half: an axis the analysis never answered is not a measurement.
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

  // VOICE-10: another voice of the same account is genuinely empty.
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
  // VOICE-54.
  it('gives every tab an address under the voice and marks the current one', async () => {
    const { router } = renderAppAt(DEFAULT, { user: { id: 'alice' } })

    const tabs = within(await screen.findByRole('navigation', { name: '말투 설정' })).getAllByRole(
      'link',
    )
    // Three tabs: the 규칙 and 검증 tabs left with contrast rules and profile validation.
    expect(tabs.map((tab) => tab.getAttribute('href'))).toEqual([
      DEFAULT,
      `${DEFAULT}/versions`,
      `${DEFAULT}/import`,
    ])
    expect(tabs[0]).toHaveAttribute('aria-current', 'page')
    // THEME-29, the mechanical half: the row scrolls instead of wrapping or crushing its Korean
    // labels, and every tab keeps the 44px floor.
    expect(screen.getByRole('navigation', { name: '말투 설정' })).toHaveClass('overflow-x-auto')
    tabs.forEach((tab) => {
      expect(tab).toHaveClass('min-h-10', 'pointer-coarse:min-h-11')
      expect(tab).toHaveClass('whitespace-nowrap')
    })

    await userEvent.setup().click(tabs[2])
    await waitFor(() => expect(router.state.location.pathname).toBe(`${DEFAULT}/import`))
    expect(
      await screen.findByRole('heading', { level: 2, name: '기존 글 가져오기' }),
    ).toBeInTheDocument()

    router.history.back()
    await waitFor(() => expect(router.state.location.pathname).toBe(DEFAULT))
  })

  it.each([
    [`${DEFAULT}/versions`, '버전 기록'],
    [`${DEFAULT}/import`, '기존 글 가져오기'],
  ])('renders %s as its own screen on reload', async (path, heading) => {
    const { router } = renderAppAt(path, { user: { id: 'alice' } })

    expect(await screen.findByRole('heading', { level: 2, name: heading })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe(path)
    expect(screen.queryByText('현재 말투 프로필')).not.toBeInTheDocument()
  })
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
    ['/voice/import', `${DEFAULT}/import`],
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
    // free-text styleguide field left for it to appear in (VOICE-25).
    await waitFor(() => expect(screen.getByText(/~다를 자주 사용/)).toBeInTheDocument())
  })

  // VOICE-6: the 이전 수동 안내 section and both of its editors are gone from every tab.
  it('offers no free-text guidance editors anywhere on the voice screens', async () => {
    renderAppAt(`${DEFAULT}/import`, { user: { id: 'alice' } })
    await screen.findByLabelText('내가 쓴 글')
    expect(screen.queryByText('이전 수동 안내')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('문체 규칙')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('추가 규칙')).not.toBeInTheDocument()
  })

  // VOICE-54: the paste form's first field says what it is.
  it('labels the imported piece 제목 and keeps it optional', async () => {
    renderAppAt(`${DEFAULT}/import`, { user: { id: 'alice' } })
    expect(await screen.findByLabelText('제목 (선택)')).toBeInTheDocument()
    expect(screen.queryByLabelText('라벨 (선택)')).not.toBeInTheDocument()
  })

  // VOICE-30: a version is READ before it is taken, and the preview is the confirmation.
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

    await screen.findByRole('heading', { level: 2, name: '프로필' })
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
    // The profile is simply empty; nothing partial was written.
    expect(screen.getByText('현재 말투 프로필')).toBeInTheDocument()
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
    expect(screen.getByText(/삭제된 말투예요\. 기록은 볼 수 있지만/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '복원' })).toBeInTheDocument()
    expect(await screen.findByText(/삭제된 말투에는 글을 가져올 수 없어요/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '학습' })).toBeDisabled()
  })
})
