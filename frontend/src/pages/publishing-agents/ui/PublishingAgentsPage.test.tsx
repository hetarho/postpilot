import { create } from '@bufbuild/protobuf'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { publishingAgentsQueryKey } from '@/entities/publishing-agent'
import { retryablePublishJobsQueryKey } from '@/entities/publish-job'
import {
  PublishJobSchema,
  PublishStatus,
  PublishVisibility,
  PublishingAgentSchema,
} from '@/shared/api'
import { renderAppAt } from '@/test/app'

const AGENT = create(PublishingAgentSchema, {
  id: 'agent-alice',
  label: '침실 Mac',
  platformAccountId: 'alice-blog',
  platformAccountLabel: '앨리스 블로그',
  browserLabel: 'Google Chrome',
  categories: [{ id: 'daily', name: '일상' }],
  defaultCategoryId: 'daily',
  defaultVisibility: PublishVisibility.PUBLIC,
  lastSeenAt: '2026-08-30T12:00:00Z',
  ready: true,
})

describe('publishing agent management', () => {
  it('shows safe connection metadata and creates a one-time pairing code', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/publishing-agents', {
      user: { id: 'alice' },
      calls,
      publishing: { agents: [AGENT] },
    })

    expect(await screen.findByRole('heading', { name: '발행 Mac' })).toBeInTheDocument()
    expect(await screen.findByText(/앨리스 블로그/)).toHaveTextContent('Google Chrome')
    expect(document.body).not.toHaveTextContent('agent-token')

    await user.click(screen.getByRole('button', { name: '연결 코드 만들기' }))
    expect(await screen.findByText('ABCD-1234-EF56')).toBeInTheDocument()
    expect(calls).toContain('CreateAgentPairing')
  })

  it('requires confirmation before revoking only the selected connection', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/publishing-agents', {
      user: { id: 'alice' },
      calls,
      publishing: { agents: [AGENT] },
    })

    await user.click(await screen.findByRole('button', { name: '연결 해제' }))
    expect(calls).not.toContain('RevokePublishingAgent')
    const dialog = screen.getByRole('dialog', { name: 'Mac 연결을 해제할까요?' })
    expect(dialog).toHaveTextContent('침실 Mac의 발행 토큰이 즉시 무효화됩니다')
    await user.click(within(dialog).getByRole('button', { name: '연결 해제' }))
    await waitFor(() => expect(calls).toContain('RevokePublishingAgent'))
  })

  it('keeps publishing-agent cache keys account scoped and serialisable', () => {
    expect(publishingAgentsQueryKey('alice')).not.toEqual(publishingAgentsQueryKey('bob'))
    // TanStack hashes keys with JSON.stringify, so a segment that serialises to {} — a
    // Connect transport, for one — partitions nothing and only looks like it does.
    expect(publishingAgentsQueryKey('alice')).toEqual(['publishing-agents', 'alice'])
  })

  it('retries a retained needs-attention job outside the deleted post route', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const retainedJob = create(PublishJobSchema, {
      id: 'publish-job-retained',
      postSlug: 'deleted-post',
      agentId: AGENT.id,
      status: PublishStatus.NEEDS_ATTENTION,
      errorMessage: '네이버 로그인이 필요합니다.',
    })
    renderAppAt('/publishing-agents', {
      user: { id: 'alice' },
      calls,
      publishing: { agents: [AGENT], jobs: [retainedJob] },
    })

    expect(await screen.findByText('deleted-post')).toBeInTheDocument()
    expect(screen.getByText('네이버 로그인이 필요합니다.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '로그인 복구 후 다시 시도' }))
    await waitFor(() => expect(calls).toContain('RetryPublish'))
  })

  it('cancels a deleted-source retained job even after its agent identity is revoked', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const revokedAgent = create(PublishingAgentSchema, {
      ...AGENT,
      platformAccountId: 'changed-blog',
      revokedAt: '2026-08-31T00:00:00Z',
      ready: false,
    })
    const retainedJob = create(PublishJobSchema, {
      id: 'publish-job-orphaned',
      postSlug: 'already-deleted-post',
      agentId: AGENT.id,
      status: PublishStatus.NEEDS_ATTENTION,
    })
    renderAppAt('/publishing-agents', {
      user: { id: 'alice' },
      calls,
      publishing: { agents: [revokedAgent], jobs: [retainedJob] },
    })

    expect(await screen.findByText('already-deleted-post')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '복구 작업 취소' }))
    const dialog = screen.getByRole('dialog', { name: '고정해 둔 발행 작업을 취소할까요?' })
    expect(dialog).toHaveTextContent('고정된 글과 임시 사진을 삭제')
    await user.click(within(dialog).getByRole('button', { name: '작업 취소' }))
    await waitFor(() => expect(calls).toContain('CancelPublish'))
    await waitFor(() => expect(screen.queryByText('already-deleted-post')).not.toBeInTheDocument())
  })

  it('keeps retryable publish-job cache keys account scoped and serialisable', () => {
    expect(retryablePublishJobsQueryKey('alice')).not.toEqual(retryablePublishJobsQueryKey('bob'))
    expect(retryablePublishJobsQueryKey('alice')).toEqual(['retryable-publish-jobs', 'alice'])
  })
})
