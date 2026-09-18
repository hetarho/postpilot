import { screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { renderAppAt } from '@/test/app'
import { observedClipFixture } from '@/test/clip-observations'

afterEach(() => vi.unstubAllGlobals())

it.each([true, false])(
  'offers the probed kind without holding any original (supported=%s)',
  async (supported) => {
    const video = vi.fn(async () => ({ supported }))
    const audio = vi.fn(async () => ({ supported: false }))
    vi.stubGlobal('VideoEncoder', { isConfigSupported: video })
    vi.stubGlobal('AudioEncoder', { isConfigSupported: audio })
    const project = observedClipFixture()
    project.result = undefined
    project.renderedPlanRevision = 0
    project.editing!.plan.sourceAudio = project.editing!.plan.cuts.map((cut) => ({
      sourceId: cut.sourceId,
      fingerprint: cut.fingerprint,
      retainOriginalAudio: false,
    }))
    const calls: string[] = []
    renderAppAt('/clips/project', { user: { id: 'alice' }, clips: { projects: [project], calls } })
    if (supported) {
      expect(await screen.findByRole('button', { name: '렌더하기 · 브라우저' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '서버로 변경' })).toBeInTheDocument()
    } else {
      expect(
        await screen.findByText(/이 브라우저는 필요한 영상·음성 인코딩을 지원하지/),
      ).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '렌더하기 · 서버' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: '브라우저로 변경' })).not.toBeInTheDocument()
    }
    expect(video).toHaveBeenCalledWith(
      expect.objectContaining({ width: 1080, height: 1920, framerate: 30 }),
    )
    expect(audio).not.toHaveBeenCalled()
    expect(calls).not.toContain('StartClipRender')
  },
)
