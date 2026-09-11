import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeClipsOptions } from '@/test/clips'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { discardClipDraftQueues } from '@/features/edit-clip-project'
import { putBlobWithProgress } from '@/shared/lib/upload'

vi.mock('@/features/upload-clip-sources/model/manifest', async (original) => ({
  ...(await original<object>()),
  readSourceManifest: vi.fn(),
}))
vi.mock('@/shared/lib/upload', async (original) => ({
  ...(await original<object>()),
  putBlobWithProgress: vi.fn(),
}))
const template = {
  id: 'template',
  name: '여행',
  informationFields: [{ label: '장소', prompt: '어디인가요?' }],
  cutGuidance: '',
  copyStyles: ['clean'] as const,
  accent: '' as const,
  preset: 'restaurant' as const,
}
const project: ClipProjectDraft & { id: string } = {
  id: 'project',
  title: '제주 여행',
  videoTemplateId: template.id,
  ratio: 'vertical',
  targetDurationMs: 30000,
  disclosure: 'ad',
  cta: '',
  answers: [{ label: '장소', text: '제주도' }],
}
const mount = (path: string, clips: FakeClipsOptions = {}) =>
  renderAppAt(path, {
    user: { id: 'alice' },
    clips: {
      templates: [{ ...template, copyStyles: [...template.copyStyles] }],
      projects: [project],
      ...clips,
    },
  })
afterEach(() => {
  vi.restoreAllMocks()
  // The settings autosave queue is module state that outlives its form on purpose (CLIP-39), so
  // an unsent draft would leak into the next test the way it would leak into the next session.
  discardClipDraftQueues()
})
async function fillSetup() {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('클립 제목'), ' 새 경험 ')
  await user.click(screen.getByRole('combobox', { name: /^영상 템플릿/ }))
  await user.click(await screen.findByRole('option', { name: '여행' }))
  await user.type(await screen.findByLabelText('장소'), '서울')
  // A clip carries its ad disclosure throughout, so its campaign type is part
  // of a complete setup (CDS-5).
  await user.click(screen.getByRole('combobox', { name: /^체험단 유형/ }))
  await user.click(await screen.findByRole('option', { name: '광고' }))
  return user
}
describe('clip directory and setup', () => {
  it('guards authenticated routes and does not load clips without a session', async () => {
    const calls: string[] = []
    const { router } = renderAppAt('/clips', { calls })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(calls).not.toContain('ListClipProjects')
  })
  it('lists the persisted metadata and navigates to a ratio-immutable project', async () => {
    const user = userEvent.setup()
    mount('/clips')
    const list = within(await screen.findByRole('list', { name: '저장된 클립' }))
    const link = await list.findByRole('link', { name: /제주 여행/ })
    // The row carries the badge, the template and the ratio as metadata, and a relative time
    // (CLIP-41). The target duration lives on the project's own screen.
    expect(link).toHaveTextContent('초안')
    expect(link).toHaveTextContent('세로 9:16')
    await waitFor(() => expect(link).toHaveTextContent('여행'))
    expect(screen.getAllByRole('link', { name: '새 클립' })).toHaveLength(1)
    await user.click(link)
    expect(await screen.findByLabelText('클립 제목')).toHaveValue('제주 여행')
    expect(screen.queryByRole('combobox', { name: /^화면 비율/ })).not.toBeInTheDocument()
    expect(screen.getByText(/화면 비율은 바꿀 수 없어요/)).toBeInTheDocument()
  })
  it('handles empty and failed directories', async () => {
    const empty = mount('/clips', { projects: [] })
    expect(await screen.findByText('아직 저장된 클립이 없어요')).toBeInTheDocument()
    empty.unmount()
    mount('/clips', { projectListFails: true })
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
  it('validates required setup, creates exactly once, then enables source selection', async () => {
    const calls: string[] = []
    const projectWrites: ClipProjectDraft[] = []
    const { router } = mount('/clips/new', { calls, projectWrites })
    expect(await screen.findByRole('button', { name: '클립 만들기' })).toBeDisabled()
    expect(screen.queryByLabelText('원본 영상 선택')).not.toBeInTheDocument()
    const user = await fillSetup()
    const duration = screen.getByLabelText('목표 길이 (초)')
    await user.clear(duration)
    await user.type(duration, '91')
    expect(screen.getByRole('button', { name: '클립 만들기' })).toBeDisabled()
    await user.clear(duration)
    await user.type(duration, '15')
    await user.click(screen.getByRole('combobox', { name: /^화면 비율/ }))
    await user.click(screen.getByRole('option', { name: '정방형 1:1' }))
    await user.dblClick(screen.getByRole('button', { name: '클립 만들기' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/clips/clip-1'))
    expect(calls.filter((c) => c === 'CreateClipProject')).toHaveLength(1)
    expect(projectWrites[0]).toMatchObject({
      title: '새 경험',
      ratio: 'square',
      targetDurationMs: 15000,
      disclosure: 'ad',
      cta: '',
      answers: [{ label: '장소', text: '서울' }],
    })
    expect(await screen.findByLabelText('원본 영상 선택')).toBeEnabled()
    // The status line reports the project's own state now; the picker's own button is what says
    // to select sources (CLIP-38).
    expect(await screen.findByRole('status', { name: '클립 상태' })).toHaveTextContent('초안')
  })
  it('gives /clips/new the workspace top row and no lifecycle', async () => {
    mount('/clips/new')
    expect(await screen.findByRole('link', { name: '클립 목록' })).toBeInTheDocument()
    // The page's ONE status region is mounted before there is a project to have a status
    // (CLIP-37), and a draft with no lifecycle shows no step bar and nothing to delete.
    expect(screen.getByRole('status', { name: '클립 상태' })).toBeInTheDocument()
    expect(screen.queryByRole('tablist', { name: '클립 단계' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '삭제' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '클립 만들기' })).toBeInTheDocument()
  })
  it('keeps every unsaved setup value after a localized save failure', async () => {
    const { router } = mount('/clips/new', { projectSaveFails: true })
    const user = await fillSetup()
    await user.click(screen.getByRole('button', { name: '클립 만들기' }))
    expect(
      await screen.findByText('클립 또는 영상 템플릿의 입력값과 제한을 확인해 주세요.'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('클립 제목')).toHaveValue(' 새 경험 ')
    expect(screen.getByLabelText('장소')).toHaveValue('서울')
    expect(router.state.location.pathname).toBe('/clips/new')
  })
  it('updates title and answers without changing ratio, and preserves stale-template answers', async () => {
    const projectWrites: ClipProjectDraft[] = []
    mount('/clips/project', {
      projectWrites,
      projects: [
        { ...project, answers: [...project.answers, { label: '이전 질문', text: '보존' }] },
      ],
    })
    const user = userEvent.setup()
    const title = await screen.findByLabelText('클립 제목')
    await user.type(title, ' 기록')
    // Nothing is pressed: the settings save themselves a beat after the typing stops (CLIP-39),
    // and the picker simply waits for the server to have them.
    expect(screen.queryByRole('button', { name: '설정 저장' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('원본 영상 선택')).toBeDisabled()
    expect(await screen.findByText('저장됨', undefined, { timeout: 4000 })).toBeInTheDocument()
    expect(projectWrites[0]).toMatchObject({
      title: '제주 여행 기록',
      ratio: 'vertical',
      answers: expect.arrayContaining([{ label: '이전 질문', text: '보존' }]),
    })
    expect(screen.getByLabelText('원본 영상 선택')).toBeEnabled()
  })
  it('lets the owner leave an autosaved project, and still guards an unminted one', async () => {
    const user = userEvent.setup()
    const { router, unmount } = mount('/clips/project')
    await user.type(await screen.findByLabelText('클립 제목'), ' 기록')
    // No dialog: there is nothing to lose by leaving a project that saves itself (CLIP-39).
    await user.click(screen.getByRole('link', { name: '클립 목록' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/clips'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    unmount()

    const created = mount('/clips/new')
    await user.type(await screen.findByLabelText('클립 제목'), '새 클립')
    await user.click(screen.getByRole('link', { name: '클립 목록' }))
    // `/clips/new` is the one screen whose input is not queued anywhere yet.
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(created.router.state.location.pathname).toBe('/clips/new')
  })
  it.each(['unknown', 'foreign'])('does not expose %s project metadata', async (id) => {
    mount(`/clips/${id}`, { projects: [{ ...project, id: 'foreign', ownerId: 'bob' }] })
    expect(await screen.findByText('클립 또는 영상 템플릿을 찾을 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByLabelText('클립 제목')).not.toBeInTheDocument()
  })
})
describe('clip page local upload lifecycle', () => {
  async function uploadFixture(options: FakeClipsOptions = {}) {
    const file = new File(['clip'], 'clip.mp4', { type: 'video/mp4' })
    vi.mocked(readSourceManifest).mockResolvedValue([
      {
        filename: file.name,
        contentType: file.type,
        bytes: file.size,
        width: 1920,
        height: 1080,
        durationMs: 1000,
        fingerprint: 'a'.repeat(64),
      },
    ])
    vi.mocked(putBlobWithProgress).mockImplementation(async (_u, _h, _b, progress) => {
      progress(50)
    })
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:local-clip')
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const view = mount('/clips/project', options)
    const input = await screen.findByLabelText('원본 영상 선택')
    await waitFor(() => expect(input).toBeEnabled())
    const disclosure = screen.getByText(/선택한 영상과 들리는 말이 담긴 압축 사본을 OpenRouter/)
    expect(
      disclosure.compareDocumentPosition(input) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    const user = userEvent.setup()
    await user.upload(input, file)
    return { ...view, revoke, user, file }
  }
  it('uploads directly, keeps cache byte-free, discards on cancel and revokes previews', async () => {
    const calls: string[] = []
    const sourceRequests: unknown[] = []
    const { queryClient, user, revoke, file } = await uploadFixture({ calls, sourceRequests })
    await screen.findByText('업로드 준비 완료')
    expect(putBlobWithProgress).toHaveBeenCalledWith(
      expect.stringContaining('https://storage.test/'),
      { 'Content-Type': 'video/mp4', 'If-None-Match': '*' },
      file,
      expect.any(Function),
      expect.any(AbortSignal),
    )
    const json = JSON.stringify(
      queryClient
        .getQueryCache()
        .getAll()
        .map((q) => q.state.data),
    )
    expect(json).not.toMatch(/blob:|clip.mp4|storage.test/)
    expect(calls.filter((c) => c === 'CreateClipSourceBatch')).toHaveLength(1)
    expect(calls.filter((c) => c === 'ConfirmClipSource')).toHaveLength(1)
    expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: '선택 취소' }))
    // Back to idle: the status line drops to the project's own state and the picker asks for a
    // fresh selection rather than a replacement.
    expect(await screen.findByRole('status', { name: '클립 상태' })).toHaveTextContent('초안')
    expect(screen.getByLabelText('원본 영상 선택')).toBeInTheDocument()
    expect(calls).toContain('DiscardClipSourceBatch')
    expect(revoke).toHaveBeenCalledWith('blob:local-clip')
  })
  it('preserves saved settings after confirmation failure and clears the transient batch', async () => {
    const calls: string[] = []
    const { revoke } = await uploadFixture({ confirmFails: true, calls })
    await screen.findByRole('alert')
    expect(screen.getByLabelText('클립 제목')).toHaveValue(project.title)
    expect(screen.getByLabelText('장소')).toHaveValue('제주도')
    expect(calls).toContain('DiscardClipSourceBatch')
    expect(revoke).toHaveBeenCalledWith('blob:local-clip')
  })
  it('shows reselection after remount and never deletes asynchronously on unload', async () => {
    const calls: string[] = []
    const { unmount, revoke } = await uploadFixture({ calls })
    await screen.findByText('업로드 준비 완료')
    unmount()
    expect(revoke).toHaveBeenCalledWith('blob:local-clip')
    expect(calls).not.toContain('DiscardClipSourceBatch')
    mount('/clips/project')
    expect(await screen.findByRole('status', { name: '클립 상태' })).toHaveTextContent('초안')
    expect(screen.queryByText('clip.mp4')).not.toBeInTheDocument()
  })
})
