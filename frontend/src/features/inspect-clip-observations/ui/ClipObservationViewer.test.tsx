import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { observedClipFixture } from '@/test/clip-observations'
import { ClipObservationViewer } from './ClipObservationViewer'

afterEach(() => vi.restoreAllMocks())

describe('clip observation inspection', () => {
  it('shows recorded summary, reveals exact ranges and maps actual use without source media', async () => {
    const user = userEvent.setup()
    render(<ClipObservationViewer project={observedClipFixture()} localSources={[]} />)
    expect(screen.getByRole('heading', { name: 'AI가 관찰한 내용' })).toBeInTheDocument()
    expect(screen.getByText(/사용 구간은 완성 영상에 반영된 편집안 기준/)).toBeVisible()
    expect(screen.getByText(/관찰 내용은 원본 없이 볼 수 있어요/)).toBeVisible()
    expect(screen.getByText('맛있어요')).not.toBeVisible()
    const details = screen.getByRole('button', { name: '관찰 구간 2개 자세히 보기' })
    details.focus()
    await user.keyboard('{Enter}')
    expect(details).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('맛있어요')).toBeVisible()
    expect(screen.getByText('음식, 접시')).toBeVisible()
    expect(screen.getByText('선명하고 흔들림이 적음')).toBeVisible()
    expect(screen.getByText('1번 컷에 사용 · 원본 0:03.500–0:10')).toBeVisible()
    expect(screen.getByText('이 관찰 구간은 편집안에 사용되지 않았어요.')).toBeVisible()
    expect(screen.queryByRole('button', { name: /원본 .* 구간 보기/ })).not.toBeInTheDocument()
    await user.click(details)
    expect(screen.getByText('맛있어요')).not.toBeVisible()
  })

  it('selects sources by keyboard and reports a source with no observed ranges', async () => {
    const user = userEvent.setup()
    render(<ClipObservationViewer project={observedClipFixture()} localSources={[]} />)
    const second = screen.getByRole('button', { name: 'source-b.mp4 선택' })
    second.focus()
    await user.keyboard(' ')
    expect(second).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText(/선택 2 \/ 2/)).toBeVisible()
    expect(screen.getByText('이 영상에 기록된 관찰 구간이 없어요.')).toBeVisible()
    expect(screen.queryByRole('button', { name: /자세히 보기/ })).not.toBeInTheDocument()
  })

  it('labels usage as a saved plan and reflects saved reorder, trim and deletion', async () => {
    const project = observedClipFixture()
    const { rerender } = render(<ClipObservationViewer project={project} localSources={[]} />)
    await userEvent.click(screen.getByRole('button', { name: /자세히 보기/ }))
    project.editPlanRevision = 2
    project.editing!.plan.cuts.reverse()
    project.editing!.plan.cuts[1]!.endMs = 5000
    rerender(<ClipObservationViewer project={{ ...project }} localSources={[]} />)
    expect(screen.getByText(/사용 구간은 저장된 편집안 기준/)).toBeVisible()
    expect(screen.getByText('2번 컷에 사용 · 원본 0:03.500–0:05')).toBeVisible()
    project.editing!.plan.cuts.pop()
    rerender(<ClipObservationViewer project={{ ...project }} localSources={[]} />)
    expect(screen.queryByText(/번 컷에 사용/)).not.toBeInTheDocument()
  })

  it('only previews a matching local source and releases the player when media disappears', async () => {
    const project = observedClipFixture()
    const { rerender } = render(
      <ClipObservationViewer
        project={project}
        localSources={[{ fingerprint: 'wrong', url: 'blob:wrong' }]}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: /자세히 보기/ }))
    expect(screen.queryByRole('button', { name: /원본 .* 구간 보기/ })).not.toBeInTheDocument()
    rerender(
      <ClipObservationViewer
        project={project}
        localSources={[{ fingerprint: 'a'.repeat(64), url: 'blob:original' }]}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: '원본 0:03.500–0:10.500 구간 보기' }))
    const dialog = screen.getByRole('dialog', { name: '관찰 구간 원본 보기' })
    const video = within(dialog).getByLabelText('관찰 구간 원본 보기') as HTMLVideoElement
    expect(video).toHaveAttribute('src', 'blob:original')
    expect(video).toHaveAttribute('tabindex', '0')
    expect(video).not.toHaveAttribute('autoplay')
    fireEvent.loadedMetadata(video)
    expect(video.currentTime).toBe(3.5)
    const pause = vi.spyOn(video, 'pause').mockImplementation(() => {})
    video.currentTime = 10.5
    fireEvent.timeUpdate(video)
    expect(pause).toHaveBeenCalledOnce()
    rerender(<ClipObservationViewer project={project} localSources={[]} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByText('맛있어요')).toBeVisible()
  })

  it.each([
    undefined,
    { status: 'empty' as const, sources: [] },
    { status: 'unavailable' as const, sources: [] },
  ])('explains missing observations: %j', (observations) => {
    render(
      <ClipObservationViewer
        project={{ ...observedClipFixture(), observations }}
        localSources={[]}
      />,
    )
    expect(
      screen.getByText(
        observations?.status === 'unavailable'
          ? /저장된 관찰 결과를 읽을 수 없어요/
          : /아직 저장된 관찰 결과가 없어요/,
      ),
    ).toBeVisible()
  })
})
