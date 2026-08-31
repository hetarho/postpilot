import { create } from '@bufbuild/protobuf'
import { Code } from '@connectrpc/connect'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import {
  PublishJobSchema,
  PublishStatus,
  PublishVisibility,
  PublishingAgentSchema,
} from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { PublishingAgentsPage } from './PublishingAgentsPage'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

const AGENT = create(PublishingAgentSchema, {
  id: 'agent-alice',
  label: 'Bedroom Mac',
  platformAccountId: 'alice-blog',
  platformAccountLabel: 'Alice blog',
  browserLabel: 'Google Chrome',
  categories: [{ id: 'daily', name: 'Daily' }],
  defaultCategoryId: 'daily',
  defaultVisibility: PublishVisibility.PUBLIC,
  lastSeenAt: '2026-08-30T12:00:00Z',
  ready: true,
})

const RETAINED_JOB = create(PublishJobSchema, {
  id: 'publish-job-retained',
  postSlug: 'deleted-post',
  agentId: AGENT.id,
  status: PublishStatus.NEEDS_ATTENTION,
})

describe('publishing mutation failures', () => {
  it.each([
    {
      locale: 'ko' as const,
      labels: {
        heading: '발행 Mac',
        pair: '연결 코드 만들기',
        configure: '기본값 저장',
        revoke: '연결 해제',
        revokeTitle: 'Mac 연결을 해제할까요?',
        retry: '로그인 복구 후 다시 시도',
        cancel: '복구 작업 취소',
        cancelTitle: '고정해 둔 발행 작업을 취소할까요?',
        cancelConfirm: '작업 취소',
        dismiss: '취소',
      },
      messages: {
        pair: '만들 수 있는 Mac 연결 코드 수를 넘었어요.',
        configure: '발행 설정을 확인해 주세요.',
        revoke: '연결이 해제된 Mac이에요.',
        retry: '현재 발행 상태에서는 이 변경을 할 수 없어요.',
        cancel: '최종 발행이 시작됐을 수 있어 자동으로 다시 시도하지 않아요.',
      },
    },
    {
      locale: 'en' as const,
      labels: {
        heading: 'Publishing Macs',
        pair: 'Create connection code',
        configure: 'Save defaults',
        revoke: 'Disconnect',
        revokeTitle: 'Disconnect this Mac?',
        retry: 'Retry after restoring login',
        cancel: 'Cancel recovery job',
        cancelTitle: 'Cancel this retained publishing job?',
        cancelConfirm: 'Cancel job',
        dismiss: 'Cancel',
      },
      messages: {
        pair: 'Too many Mac pairing codes are active.',
        configure: 'Check the publishing settings.',
        revoke: 'This Mac connection has been revoked.',
        retry: 'That change is not available in the current publishing state.',
        cancel: 'Final publishing may have started, so the job will not retry automatically.',
      },
    },
  ])(
    'localizes every management mutation reason and hides backend prose in $locale',
    async ({ locale, labels, messages }) => {
      initializeI18n(locale)
      const user = userEvent.setup()
      const transport = createFakeAuthTransport({
        user: { id: 'alice' },
        publishing: {
          agents: [AGENT],
          jobs: [RETAINED_JOB],
          mutationFailures: {
            pair: { reason: 'PUBLISH_PAIRING_LIMIT', code: Code.ResourceExhausted },
            configure: { reason: 'PUBLISH_REQUEST_INVALID', code: Code.InvalidArgument },
            revoke: { reason: 'PUBLISH_AGENT_REVOKED', code: Code.FailedPrecondition },
            retry: { reason: 'PUBLISH_TRANSITION_INVALID', code: Code.Aborted },
            cancel: { reason: 'PUBLISH_COMMIT_FENCE', code: Code.FailedPrecondition },
          },
        },
      })
      render(<PublishingAgentsPage />, {
        wrapper: withProviders(transport, createTestQueryClient()),
      })

      expect(await screen.findByRole('heading', { level: 1, name: labels.heading })).toBeVisible()

      await user.click(screen.getByRole('button', { name: labels.pair }))
      expect(await screen.findByText(messages.pair)).toBeInTheDocument()

      await user.click(await screen.findByRole('button', { name: labels.configure }))
      expect(await screen.findByText(messages.configure)).toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: labels.revoke }))
      const revokeDialog = screen.getByRole('dialog', { name: labels.revokeTitle })
      await user.click(within(revokeDialog).getByRole('button', { name: labels.revoke }))
      expect(await within(revokeDialog).findByText(messages.revoke)).toBeInTheDocument()
      await user.click(within(revokeDialog).getByRole('button', { name: labels.dismiss }))

      await user.click(screen.getByRole('button', { name: labels.retry }))
      expect(await screen.findByText(messages.retry)).toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: labels.cancel }))
      const cancelDialog = screen.getByRole('dialog', { name: labels.cancelTitle })
      await user.click(within(cancelDialog).getByRole('button', { name: labels.cancelConfirm }))
      expect(await within(cancelDialog).findByText(messages.cancel)).toBeInTheDocument()

      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[resource_exhausted]')
      expect(document.body).not.toHaveTextContent('[invalid_argument]')
      expect(document.body).not.toHaveTextContent('[failed_precondition]')
      expect(document.body).not.toHaveTextContent('[aborted]')
    },
  )
})
