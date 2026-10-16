import { afterEach, beforeEach, expect, it } from 'vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClientProvider } from '@tanstack/react-query'
import { TransportProvider } from '@connectrpc/connect-query'
import { initializeI18n } from '@/app/providers/i18n'
import { emptyClipProject, type ClipProject } from '@/entities/clip-project'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport, createTestQueryClient } from '@/test/session'
import { finalizedPostRow } from '@/test/posts'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import { discardClipDraftQueues } from '../model/clip-draft-queue'
import { ClipLinkedPost } from './ClipLinkedPost'

const project: FakeClipProject = {
  ...emptyClipProject(),
  id: 'clip',
  title: '카페',
  targetDurationMs: 15000,
  disclosure: 'self',
}
const post = (slug: string, title: string) => finalizedPostRow({ slug, title })

beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  discardClipDraftQueues()
  sessionStorage.clear()
})

it('links a post from the picker with title matches first', async () => {
  const calls: string[] = []
  const requests: NonNullable<FakeClipsOptions['projectRequests']> = []
  const listRequests: Array<{
    query: string
    pageSize: number
    pageToken: string
    status: string
  }> = []
  const tagOnly = post('tag', '다른 체험')
  tagOnly.content = { ...tagOnly.content!, tags: ['카페'] }
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    calls,
    posts: {
      posts: [
        post('other', '최근 여행'),
        post('match-2', '카페 나들이'),
        tagOnly,
        post('match-1', '성수 카페'),
        { slug: 'empty', title: '카페 메모' },
      ],
      listRequests,
    },
    clips: { projects: [{ ...project }], projectRequests: requests },
  })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '글 연결: 없음' }))
  const sheet = await screen.findByRole('dialog', { name: '글 연결' })
  expect(within(sheet).getByRole('searchbox', { name: '글 검색' })).toHaveValue('카페')
  await within(sheet).findByRole('button', { name: '성수 카페' })
  expect(
    within(sheet)
      .getAllByRole('listitem')
      .map((item) => item.textContent),
  ).toEqual(['카페 나들이', '성수 카페', '최근 여행', '다른 체험'])
  expect(listRequests.map((request) => request.query)).toEqual(expect.arrayContaining(['카페', '']))
  await user.dblClick(within(sheet).getByRole('button', { name: '카페 나들이' }))
  await screen.findByRole('button', { name: '글 연결: 카페 나들이' })
  await waitFor(() =>
    expect(screen.queryByRole('dialog', { name: '글 연결' })).not.toBeInTheDocument(),
  )
  expect(requests).toEqual([
    { id: 'clip', linkedPostSlug: 'match-2', fields: ['id', 'linkedPostSlug'] },
  ])
  expect(calls.filter((name) => /^(Start|Generate|Analyze|Create|SavePost)/.test(name))).toEqual([])
  expect(screen.getByRole('textbox', { name: '클립 제목' })).toHaveValue('카페')
})

it('unlinks', async () => {
  const requests: NonNullable<FakeClipsOptions['projectRequests']> = []
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    posts: { posts: [post('old', '연결한 글')] },
    clips: {
      projects: [
        {
          ...project,
          linkedPost: { slug: 'old', title: '연결한 글', updatedAt: '2026-10-15T00:00:00Z' },
        },
      ],
      projectRequests: requests,
    },
  })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '글 연결: 연결한 글' }))
  const sheet = await screen.findByRole('dialog', { name: '글 연결' })
  await user.click(within(sheet).getByRole('button', { name: '연결 해제' }))
  await screen.findByRole('button', { name: '글 연결: 없음' })
  expect(requests).toEqual([{ id: 'clip', linkedPostSlug: '', fields: ['id', 'linkedPostSlug'] }])
})

it('hides posts without content', async () => {
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    posts: { posts: [{ slug: 'empty', title: '카페 메모' }, post('ready', '완성한 카페 글')] },
    clips: { projects: [{ ...project }] },
  })
  await userEvent.click(await screen.findByRole('button', { name: '글 연결: 없음' }))
  const sheet = await screen.findByRole('dialog', { name: '글 연결' })
  await within(sheet).findByRole('button', { name: '완성한 카페 글' })
  expect(within(sheet).queryByRole('button', { name: '카페 메모' })).not.toBeInTheDocument()
})

it('reads the linked title without controls for a finalized project', () => {
  const calls: string[] = []
  const value: ClipProject = {
    ...emptyClipProject(),
    id: 'clip',
    title: '카페',
    createdAt: '',
    updatedAt: '',
    editPlanRevision: 0,
    renderedPlanRevision: 0,
    linkedPost: { slug: 'post', title: '내 체험', updatedAt: '2026-10-15T00:00:00Z' },
  }
  render(
    <TransportProvider transport={createFakeAuthTransport({ user: { id: 'alice' }, calls })}>
      <QueryClientProvider client={createTestQueryClient()}>
        <ClipLinkedPost ownerId="alice" project={value} disabled={false} readOnly />
      </QueryClientProvider>
    </TransportProvider>,
  )
  expect(screen.getByText('내 체험')).toBeVisible()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(calls).toEqual([])
})
