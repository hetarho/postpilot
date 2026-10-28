import { create } from '@bufbuild/protobuf'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipSourceBatchSchema } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { observedClipFixture } from '@/test/clip-observations'

const readiness = vi.hoisted(() => ({ qualified: true }))

// Qualified test readiness exercises the browser kind without qualifying production.
vi.mock('@/entities/clip-design', async (original) => {
  const actual = await original<typeof import('@/entities/clip-design')>()
  return {
    ...actual,
    CLIP_BROWSER_COMPOSITION: {
      ...actual.CLIP_BROWSER_COMPOSITION,
      get qualified() {
        return readiness.qualified
      },
    },
  }
})

afterEach(() => {
  vi.unstubAllGlobals()
  readiness.qualified = true
})

async function openRenderChoice() {
  const project = observedClipFixture()
  project.result = undefined
  project.renderedPlanRevision = 0
  project.editing!.plan.sourceAudio = project.editing!.plan.cuts.map((cut) => ({
    sourceId: cut.sourceId,
    fingerprint: cut.fingerprint,
    retainOriginalAudio: false,
  }))
  // Retained originals, so the render trigger is open and the choice behind it can be read.
  const batch = create(ClipSourceBatchSchema, {
    id: 'retained',
    projectId: project.id,
    state: 'ready',
    current: true,
    expiresAt: '2099-01-01T00:00:00Z',
    sources: project.editing!.sources.map((source) => ({
      id: source.id,
      state: 'ready',
      availability: 'available',
      retentionExpiresAt: '2099-01-01T00:00:00Z',
      metadata: { ...source, contentType: 'video/mp4', bytes: 5n },
    })),
  })
  const calls: string[] = []
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [project], retainedBatches: [batch], calls },
  })
  const render = await screen.findByRole('button', { name: '렌더하기' })
  await waitFor(() => expect(render).toBeEnabled())
  await userEvent.click(render)
  const choice = within(await screen.findByRole('dialog', { name: '렌더하기' }))
  return { choice, calls }
}

it('refuses the browser kind by name while its release readiness is closed (ARCH-68)', async () => {
  // A device that could encode still cannot start a render the server refuses: the server
  // leads, and the browser option stays, refused, with its reason (CLIP-155).
  readiness.qualified = false
  vi.stubGlobal('VideoEncoder', { isConfigSupported: vi.fn(async () => ({ supported: true })) })
  vi.stubGlobal('AudioEncoder', { isConfigSupported: vi.fn(async () => ({ supported: true })) })
  const { choice, calls } = await openRenderChoice()
  expect(choice.getAllByRole('button', { name: /에서 렌더$/ }).map((b) => b.textContent)).toEqual([
    '서버에서 렌더',
    '브라우저에서 렌더',
  ])
  expect(await choice.findByText('브라우저 렌더는 아직 준비 중이에요.')).toBeInTheDocument()
  expect(choice.getByRole('button', { name: '브라우저에서 렌더' })).toBeDisabled()
  expect(choice.getByRole('button', { name: '서버에서 렌더' })).toBeEnabled()
  expect(calls).not.toContain('StartClipBrowserCompositionRender')
})

it.each([true, false])(
  'offers the probed kind without holding any original (supported=%s)',
  async (supported) => {
    const video = vi.fn(async () => ({ supported }))
    const audio = vi.fn(async () => ({ supported: false }))
    vi.stubGlobal('VideoEncoder', { isConfigSupported: video })
    vi.stubGlobal('AudioEncoder', { isConfigSupported: audio })
    const { choice, calls } = await openRenderChoice()
    if (supported) {
      // The browser leads on a project with no render yet, and both kinds are open.
      expect(
        choice.getAllByRole('button', { name: /에서 렌더$/ }).map((b) => b.textContent),
      ).toEqual(['브라우저에서 렌더', '서버에서 렌더'])
      expect(choice.getByRole('button', { name: '브라우저에서 렌더' })).toBeEnabled()
      expect(choice.getByRole('button', { name: '서버에서 렌더' })).toBeEnabled()
    } else {
      // A browser that cannot render keeps its option, refused, with the reason beside it
      // (CLIP-155); the server leads.
      expect(
        choice.getAllByRole('button', { name: /에서 렌더$/ }).map((b) => b.textContent),
      ).toEqual(['서버에서 렌더', '브라우저에서 렌더'])
      expect(
        await choice.findByText(/이 브라우저는 필요한 영상·음성 인코딩을 지원하지/),
      ).toBeInTheDocument()
      expect(choice.getByRole('button', { name: '브라우저에서 렌더' })).toBeDisabled()
      expect(choice.getByRole('button', { name: '서버에서 렌더' })).toBeEnabled()
    }
    expect(video).toHaveBeenCalledWith(
      expect.objectContaining({ width: 1080, height: 1920, framerate: 30 }),
    )
    expect(audio).not.toHaveBeenCalled()
    expect(calls).not.toContain('StartClipRender')
  },
)
