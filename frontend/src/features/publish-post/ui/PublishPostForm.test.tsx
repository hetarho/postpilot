import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import type { PublishingAgent } from '@/entities/publishing-agent'
import type { PublishJob } from '@/entities/publish-job'
import { PublishStage, PublishStatus, PublishVisibility } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { connectAppError } from '@/test/app-error'
import { PublishPostForm } from './PublishPostForm'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

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
  it.each([
    {
      locale: 'ko' as const,
      action: '네이버에 발행',
      message: '다른 화면에서 글이 바뀌었어요. 최신 글을 불러온 뒤 다시 시도해 주세요.',
    },
    {
      locale: 'en' as const,
      action: 'Publish to Naver',
      message: 'This post changed in another screen. Load the latest version and try again.',
    },
  ])(
    'renders a structured beforePublish refusal without raw prose in $locale',
    async ({ locale, action, message }) => {
      initializeI18n(locale)
      const user = userEvent.setup()
      const transport = createFakeAuthTransport()
      render(
        <PublishPostForm
          ownerId="alice"
          postSlug="post"
          contentRevision={3n}
          finalizedRevision={3n}
          finalized
          beforePublish={vi
            .fn()
            .mockRejectedValue(connectAppError('POST_CONTENT_STALE', Code.Aborted))}
          agents={[agent('first', 'daily', 'Daily', PublishVisibility.PUBLIC)]}
          observedAt={new Date('2026-08-30T12:00:01Z').getTime()}
          job={undefined}
        />,
        { wrapper: withProviders(transport, createTestQueryClient()) },
      )

      await user.click(screen.getByRole('button', { name: action }))

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[aborted]')
    },
  )

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
      failure: { reason: 'PUBLISH_NEEDS_ATTENTION', params: {} },
      platformPostUrl: '',
      updatedAt: '2026-08-30T12:00:00Z',
      targetLanguage: 'ko',
      contentLanguage: 'ko',
      voiceSourceLanguage: 'ko',
    }
    view.rerender(<PublishPostForm {...props} agents={[replacement]} job={retryJob} />)

    expect(screen.queryByRole('button', { name: '안전하게 다시 시도' })).not.toBeInTheDocument()
    expect(screen.queryByText('replacement 블로그')).not.toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('다른 Mac으로 바꾸어 재시도하지 않았어요')
    expect(screen.getByRole('button', { name: '발행 취소' })).toBeInTheDocument()
  })
})
