import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { WritingTestOutput } from '@/shared/api'
import { writingTestI18n } from '@/features/writing-test'
import type { UseKeyedPhotoUploadsArgs } from '@/features/upload-photos/model/useKeyedPhotoUploads'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { createTestQueryClient, withProviders } from '@/test/session'
import { PhotoTestsPage } from './PhotoTestsPage'

// jsdom decodes no image, so the batch's convert step is replaced by a handshake through the
// fixture's real test photo RPCs; everything else in the setup is the real actor.
vi.mock('@/features/upload-photos', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/features/upload-photos')>()
  return {
    ...actual,
    useKeyedPhotoUploads: (args: UseKeyedPhotoUploadsArgs) => ({
      items: [],
      completed: 0,
      retry: () => {},
      dismiss: () => {},
      addFiles: (files: File[]) => {
        for (const file of files.filter((each) => !/\.(mp4|mov|m4v|webm)$/i.test(each.name)))
          void (async () => {
            const upload = await args.handshake.createUpload('', file.name)
            const confirmed = await args.handshake.confirmUpload(upload.uploadId, {
              width: 800,
              height: 600,
            })
            if (confirmed.kind === 'photo') args.onConfirmed(confirmed.image)
          })()
      },
    }),
  }
})

beforeEach(() => {
  sessionStorage.clear()
  initializeI18n('ko')
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
})
afterEach(cleanup)

function mount(path: string, fixture: ReturnType<typeof createWritingTestStudioFixture>) {
  const root = createRootRoute({ component: Outlet })
  const photos = createRoute({
    getParentRoute: () => root,
    path: '/tests/photos',
    component: PhotoTestsPage,
  })
  const detail = createRoute({
    getParentRoute: () => root,
    path: '/tests/$testId',
    component: () => <p>running test</p>,
  })
  const router = createRouter({
    routeTree: root.addChildren([photos, detail]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  render(<RouterProvider router={router} />, {
    wrapper: withProviders(fixture.transport, createTestQueryClient()),
  })
  return router
}

it('checks observe models, uploads test photos, confirms the cost and opens the started test', async () => {
  const fixture = createWritingTestStudioFixture()
  const router = mount('/tests/photos?entry=%2Fai-models', fixture)
  expect(
    await screen.findByRole('heading', { name: '비교할 사진 보는 AI를 골라 주세요' }),
  ).toBeInTheDocument()
  expect(router.state.location.search).toMatchObject({ entry: '/ai-models' })
  expect((router.state.location.search as { draft?: string }).draft).toBeTruthy()
  for (const forbidden of [/어떤 내용으로 글을 써 볼까요/, /글감/, /글 쓰는 AI/, /말투/, /템플릿/])
    expect(screen.queryByText(forbidden)).toBeNull()

  // A refused 다음 says why in the step bar and moves to the list (MODEL-99).
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(await screen.findByText('서로 다른 후보 2개를 선택해 주세요.')).toBeInTheDocument()
  await userEvent.click(await screen.findByRole('checkbox', { name: 'Model 2' }))
  await userEvent.click(screen.getByRole('checkbox', { name: 'Model 3' }))
  await userEvent.click(screen.getByRole('button', { name: '다음' }))

  expect(
    await screen.findByRole('heading', { name: '비교할 사진을 올려 주세요' }),
  ).toBeInTheDocument()
  expect(screen.getByText('필수')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '관찰 비용 확인' }))
  expect(await screen.findByText('‘비교할 사진’ 항목을 확인해 주세요.')).toBeInTheDocument()
  const group = screen.getByRole('group', { name: /비교할 사진/ })
  expect(group).toHaveAttribute('aria-invalid', 'true')
  await waitFor(() => expect(group).toHaveFocus())

  const input = document.querySelector('input[type="file"]') as HTMLInputElement
  expect(input.accept).not.toMatch(/video|mp4/)
  await userEvent.upload(input, [
    new File(['a'], 'a.jpg', { type: 'image/jpeg' }),
    new File(['b'], 'b.heic', { type: 'image/heic' }),
  ])
  expect(await screen.findByText('2장 준비됨')).toBeInTheDocument()
  expect(fixture.calls.filter((call) => call === 'ConfirmTestPhotoUpload')).toHaveLength(2)

  await userEvent.click(screen.getByRole('button', { name: '관찰 비용 확인' }))
  expect(
    await screen.findByRole('heading', { name: 'AI 2개로 사진을 관찰할까요?' }),
  ).toBeInTheDocument()
  expect(screen.getByText('관찰 2회')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '관찰 2회 시작' }))
  await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/tests\/[^/]+$/))
  expect(router.state.location.pathname).not.toBe('/tests/photos')
  expect(router.state.location.search).toMatchObject({ entry: '/ai-models' })
  const admitted = fixture.admissions.at(-1)!.plan!
  expect(admitted.output).toBe(WritingTestOutput.OBSERVATION)
  expect(admitted.context?.photoIds).toHaveLength(2)
  expect(admitted.context?.material?.text).toBe('')
  expect(admitted.entrants.map((entrant) => entrant.source.value)).toHaveLength(2)
})

it('resumes the setup on reload and starts over on a new entry', async () => {
  const fixture = createWritingTestStudioFixture()
  const router = mount('/tests/photos', fixture)
  await userEvent.click(await screen.findByRole('checkbox', { name: 'Model 1' }))
  await userEvent.click(screen.getByRole('checkbox', { name: 'Model 2' }))
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(
    await screen.findByRole('heading', { name: '비교할 사진을 올려 주세요' }),
  ).toBeInTheDocument()
  const href = router.state.location.href
  const kept = Object.fromEntries(
    Array.from({ length: sessionStorage.length }, (_, index) => {
      const key = sessionStorage.key(index)!
      return [key, sessionStorage.getItem(key)!]
    }),
  )
  cleanup()
  for (const [key, value] of Object.entries(kept)) sessionStorage.setItem(key, value)
  mount(href, fixture)
  expect(
    await screen.findByRole('heading', { name: '비교할 사진을 올려 주세요' }),
  ).toBeInTheDocument()
  cleanup()
  // Leaving discarded the setup, and a new entry has its own key anyway.
  mount('/tests/photos', fixture)
  expect(
    await screen.findByRole('heading', { name: '비교할 사진 보는 AI를 골라 주세요' }),
  ).toBeInTheDocument()
})
