import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { PublishingAgent } from '@/entities/publishing-agent'
import type { PublishJob } from '@/entities/publish-job'
import { PublishStage, PublishStatus, PublishVisibility } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { PublishPostForm } from './PublishPostForm'

function agent(
  id: string,
  categoryId: string,
  categoryName: string,
  visibility: PublishVisibility,
): PublishingAgent {
  return {
    id,
    label: `${id} Mac`,
    platformAccountId: `${id}-blog`,
    platformAccountLabel: `${id} 블로그`,
    browserLabel: 'Google Chrome',
    categories: [{ id: categoryId, name: categoryName }],
    defaultCategoryId: categoryId,
    defaultVisibility: visibility,
    lastSeenAt: '2026-08-30T12:00:00Z',
    revokedAt: '',
    ready: true,
  }
}

describe('PublishPostForm', () => {
  it('uses the replacement agent defaults when the selected agent disappears', async () => {
    const startRequests: Array<{
      expectedContentRevision: bigint
      agentId: string
      categoryId: string
      visibility: number
    }> = []
    const transport = createFakeAuthTransport({ publishing: { startRequests } })
    const wrapper = withProviders(transport, createTestQueryClient())
    const first = agent('first', 'daily', '일상', PublishVisibility.PUBLIC)
    const replacement = agent('replacement', 'travel', '여행', PublishVisibility.PRIVATE)
    const props = {
      ownerId: 'alice',
      postSlug: 'post',
      contentRevision: 3n,
      finalizedRevision: 3n,
      finalized: true,
      beforePublish: vi.fn().mockResolvedValue(3n),
      observedAt: new Date('2026-08-30T12:00:01Z').getTime(),
      job: undefined,
    }
    const view = render(<PublishPostForm {...props} agents={[first]} />, { wrapper })

    expect(screen.getByLabelText('카테고리')).toHaveValue('daily')
    expect(screen.getByLabelText('공개 설정')).toHaveValue(String(PublishVisibility.PUBLIC))

    view.rerender(<PublishPostForm {...props} agents={[replacement]} />)
    await waitFor(() => expect(screen.getByLabelText('Mac 연결')).toHaveValue('replacement'))
    expect(screen.getByLabelText('카테고리')).toHaveValue('travel')
    expect(screen.getByLabelText('공개 설정')).toHaveValue(String(PublishVisibility.PRIVATE))

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: '네이버에 발행' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: '네이버에 발행',
      }),
    )
    await waitFor(() => expect(startRequests).toHaveLength(1))
    expect(startRequests[0]).toEqual({
      expectedContentRevision: 3n,
      agentId: 'replacement',
      categoryId: 'travel',
      visibility: PublishVisibility.PRIVATE,
    })
  })

  it('never substitutes another blog for an attention retry bound to an unavailable agent', async () => {
    const transport = createFakeAuthTransport()
    const wrapper = withProviders(transport, createTestQueryClient())
    const replacement = agent('replacement', 'travel', '여행', PublishVisibility.PRIVATE)
    const props = {
      ownerId: 'alice',
      postSlug: 'post',
      contentRevision: 3n,
      finalizedRevision: 3n,
      finalized: true,
      beforePublish: vi.fn().mockResolvedValue(3n),
      observedAt: new Date('2026-08-30T12:00:01Z').getTime(),
    }
    const view = render(<PublishPostForm {...props} agents={[replacement]} job={undefined} />, {
      wrapper,
    })
    expect(screen.getByRole('button', { name: '네이버에 발행' })).toBeInTheDocument()

    const retryJob: PublishJob = {
      id: 'attention',
      postSlug: 'post',
      agentId: 'original',
      status: PublishStatus.NEEDS_ATTENTION,
      stage: PublishStage.OPENING_EDITOR,
      contentRevision: 3n,
      categoryId: 'daily',
      visibility: PublishVisibility.PUBLIC,
      errorCode: 'login_expired',
      errorMessage: '',
      platformPostUrl: '',
      updatedAt: '2026-08-30T12:00:00Z',
    }
    view.rerender(<PublishPostForm {...props} agents={[replacement]} job={retryJob} />)

    expect(screen.queryByRole('button', { name: '안전하게 다시 시도' })).not.toBeInTheDocument()
    expect(screen.queryByText('replacement 블로그')).not.toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('다른 Mac으로 바꾸어 재시도하지 않았어요')
    expect(screen.getByRole('button', { name: '발행 취소' })).toBeInTheDocument()
  })
})
