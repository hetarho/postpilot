import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { renderAppAt } from '@/test/app'
import { spokenVoiceFixture, SPOKEN_TEST_DRAFT, SPOKEN_TEST_OPERATION } from '@/test/spoken-voices'
import { initializeI18n } from '@/app/providers/i18n'
let audio: HTMLMediaElement | undefined
beforeEach(() => {
  initializeI18n('ko')
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(() => {
    audio = document.querySelector<HTMLMediaElement>('audio[src]') ?? undefined
    return Promise.resolve()
  })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValue(
        new Response(new Blob(['private mp3']), { headers: { 'content-type': 'audio/mpeg' } }),
      ),
  )
  vi.stubGlobal('URL', URL)
  URL.createObjectURL = vi.fn(() => 'blob:private')
  URL.revokeObjectURL = vi.fn()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  audio = undefined
})
describe('spoken voice creation and reuse', () => {
  it('keeps a failed approved request available for an explicit same-key retry', async () => {
    const f = spokenVoiceFixture(),
      user = userEvent.setup()
    f.setFailStart(true)
    renderAppAt('/spoken-voices/new', { transport: f.transport })
    await screen.findByRole('textbox', { name: '목소리 이름' })
    await user.type(screen.getByRole('textbox', { name: '목소리 이름' }), '내 목소리')
    await user.type(
      screen.getByRole('textbox', { name: '원하는 목소리' }),
      '차분하고 따뜻하게 한국어를 읽는 성인 목소리입니다.',
    )
    await user.click(screen.getByRole('combobox', { name: /목소리 생성 모델/ }))
    await user.click(screen.getByRole('option', { name: /Korean design → Korean synthesis/ }))
    await user.click(screen.getByRole('button', { name: '후보 만들기 비용 확인' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: '승인하고 시작' }))
    await screen.findByText(/같은 요청을 다시 전송할 수 있어요/)
    expect(f.calls.filter((c) => c === 'start-design')).toHaveLength(1)
    f.setFailStart(false)
    await user.click(screen.getByRole('button', { name: '승인하고 시작' }))
    await screen.findByRole('radio', { name: '후보 1 선택' })
    expect(f.startKeys).toHaveLength(2)
    expect(f.startKeys[1]).toBe(f.startKeys[0])
    expect(f.calls.filter((c) => c === 'quote-design')).toHaveLength(1)
  })
  it('cancels running work only on confirmation and retains previous candidates', async () => {
    const f = spokenVoiceFixture({ preset: 'candidates', operationState: 'queued' }),
      user = userEvent.setup()
    renderAppAt(
      `/spoken-voices/new?draft=${SPOKEN_TEST_DRAFT}&operation=${SPOKEN_TEST_OPERATION}`,
      { transport: f.transport },
    )
    await screen.findByRole('button', { name: '작업 취소' })
    await user.click(screen.getByRole('button', { name: '작업 취소' }))
    const dialog = await screen.findByRole('dialog')
    expect(f.calls).not.toContain('cancel')
    await user.click(within(dialog).getByRole('button', { name: '작업 취소' }))
    await waitFor(() => expect(screen.getByRole('button', { name: '후보 1 듣기' })).toBeEnabled())
    expect(screen.getAllByRole('radio')).toHaveLength(3)
    expect(f.calls).toEqual(['cancel'])
  })
  it('explicitly chooses a model, approves one generation, listens/selects/confirms and retains the name/sample', async () => {
    const f = spokenVoiceFixture(),
      user = userEvent.setup(),
      view = renderAppAt('/spoken-voices/new', { transport: f.transport })
    await screen.findByRole('textbox', { name: '목소리 이름' })
    expect(screen.getByRole('combobox', { name: /목소리 생성 모델/ })).toHaveTextContent(
      '모델을 선택하세요',
    )
    const preview = screen.getByRole('textbox', { name: '미리 들어볼 대본' })
    expect((preview as HTMLTextAreaElement).value).toContain('12,500원')
    await user.type(screen.getByRole('textbox', { name: '목소리 이름' }), '내 더빙 목소리')
    await user.type(
      screen.getByRole('textbox', { name: '원하는 목소리' }),
      '차분하고 따뜻하게 한국어를 읽는 성인 목소리입니다.',
    )
    await user.click(screen.getByRole('combobox', { name: /목소리 생성 모델/ }))
    await user.click(screen.getByRole('option', { name: /Korean design → Korean synthesis/ }))
    await user.click(screen.getByRole('tab', { name: '후보 듣기' }))
    expect(f.calls).toEqual([])
    await user.click(screen.getByRole('tab', { name: '목소리 정보' }))
    expect(screen.getByRole('textbox', { name: '목소리 이름' })).toHaveValue('내 더빙 목소리')
    expect(f.calls.filter((c) => c.startsWith('start'))).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: '후보 만들기 비용 확인' }))
    await screen.findByRole('dialog')
    expect(f.calls).toEqual(['create', 'quote-design'])
    await user.click(screen.getByRole('button', { name: '승인하고 시작' }))
    await screen.findByRole('radio', { name: '후보 1 선택' })
    expect(screen.getByRole('button', { name: '선택한 목소리 확정' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: '후보 2 듣기' }))
    await waitFor(() => expect(audio).toBeDefined())
    expect(f.calls).not.toContain('acknowledge')
    fireEvent.playing(audio!)
    await waitFor(() => expect(f.calls).toContain('acknowledge'))
    await waitFor(() => expect(screen.getByRole('radio', { name: '후보 2 선택' })).toBeEnabled())
    await user.click(screen.getByRole('radio', { name: '후보 2 선택' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '선택한 목소리 확정' })).toBeEnabled(),
    )
    await user.click(screen.getByRole('button', { name: '선택한 목소리 확정' }))
    await screen.findByText('선택한 목소리를 저장합니다. 확정 비용은 0 크레딧이에요.')
    await user.click(screen.getByRole('button', { name: '승인하고 시작' }))
    await screen.findByRole('link', { name: '내 목소리 목록' })
    expect(f.calls.filter((c) => c === 'start-design')).toHaveLength(1)
    expect(f.calls.filter((c) => c === 'start-confirm')).toHaveLength(1)
    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining('/spoken/audio/'),
      expect.objectContaining({ credentials: 'include', cache: 'no-store' }),
    )
    await user.click(screen.getByRole('link', { name: '내 목소리 목록' }))
    await screen.findByRole('heading', { name: '내 더빙 목소리' })
    expect(f.voices.values().next().value?.sampleAssetId).toBe('5'.repeat(32))
    const starts = f.calls.filter((c) => c.startsWith('start')).length
    await user.click(screen.getByRole('button', { name: '내 더빙 목소리 듣기' }))
    await waitFor(() => expect(f.calls.filter((c) => c === 'sample')).toHaveLength(2))
    expect(f.calls.filter((c) => c.startsWith('start'))).toHaveLength(starts)
    view.unmount()
  })
  it('renames and removes metadata without synthesis and creates a separate sound draft', async () => {
    const f = spokenVoiceFixture({ preset: 'confirmed' }),
      user = userEvent.setup()
    renderAppAt('/spoken-voices', { transport: f.transport })
    await screen.findByRole('heading', { name: '나의 목소리' })
    await user.click(screen.getByRole('button', { name: '이름 바꾸기' }))
    const field = await screen.findByRole('textbox', { name: '목소리 이름' })
    await user.clear(field)
    await user.type(field, '새 이름')
    await user.click(screen.getByRole('button', { name: '이름 저장' }))
    await screen.findByRole('heading', { name: '새 이름' })
    expect(f.calls).toContain('rename')
    expect(f.calls.filter((c) => c.startsWith('start'))).toHaveLength(0)
    await user.click(screen.getByRole('link', { name: '새 목소리로 만들기' }))
    await screen.findByRole('textbox', { name: '목소리 이름' })
    expect(screen.getByRole('textbox', { name: '목소리 이름' })).toHaveValue('새 이름')
    expect(screen.getByRole('combobox', { name: /목소리 생성 모델/ })).toHaveTextContent(
      '모델을 선택하세요',
    )
    expect(f.calls).not.toContain('create')
    await user.click(screen.getByRole('link', { name: '내 목소리' }))
    await screen.findByRole('button', { name: '목록에서 제거' })
    await user.click(screen.getByRole('button', { name: '목록에서 제거' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('이미 만들어진 음성과 영상은 그대로 남아요.')
    await user.click(within(dialog).getByRole('button', { name: '목록에서 제거' }))
    await screen.findByText('아직 확정한 목소리가 없어요.')
    expect(f.calls.filter((c) => c.startsWith('start'))).toHaveLength(0)
  })
  it.each(['unresolved', 'received', 'cancelled'] as const)(
    'reloads %s work and existing samples without automatic generation',
    async (state) => {
      const f = spokenVoiceFixture({
        preset: 'candidates',
        operationState: state,
        unavailable: state === 'cancelled',
      })
      renderAppAt(
        `/spoken-voices/new?draft=${SPOKEN_TEST_DRAFT}&operation=${SPOKEN_TEST_OPERATION}`,
        { transport: f.transport },
      )
      await screen.findByRole('radio', { name: '후보 1 선택' })
      if (state === 'received')
        await screen.findByRole('button', { name: '확정된 목소리 저장 다시 시도' })
      if (state === 'unresolved')
        await screen.findByText('목소리 확정 결과를 확인해야 해요. 자동으로 다시 만들지 않습니다.')
      expect(f.calls.filter((c) => c.startsWith('start'))).toHaveLength(0)
      expect(screen.getByRole('button', { name: '후보 1 듣기' })).toBeEnabled()
      expect(screen.getByRole('button', { name: '선택한 목소리 확정' })).toBeDisabled()
    },
  )
})
