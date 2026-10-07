import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRouterTransport, Code } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ProviderService, Stage } from '@/shared/api'
import { createFakeProviderTransport } from '@/test/providers'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { chooseOption } from '@/test/listbox'
import { ActiveModelForm } from './ActiveModelForm'

const models = [
  { providerId: 'openrouter', modelId: 'a', label: 'Writer A' },
  { providerId: 'openrouter', modelId: 'b', label: 'Writer B' },
]
it('names the saved active model and a pending replacement, then confirms the actual saved model', async () => {
  let finishSave!: () => void
  const calls: string[] = []
  render(<ActiveModelForm stage="write" />, {
    wrapper: withProviders(
      createFakeProviderTransport({
        models,
        calls,
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'a' }],
        saveGate: new Promise<void>((resolve) => {
          finishSave = resolve
        }),
      }),
      createTestQueryClient(),
    ),
  })
  await screen.findByText('글 작성에 “Writer A” 모델을 사용하고 있어요.')
  const select = screen.getByRole('combobox', { name: /작성 모델/ })
  await chooseOption(userEvent.setup(), select, 'Writer B')
  await screen.findByText('글 작성 모델을 “Writer B”으로 저장하고 있어요.')
  expect(select).toBeDisabled()
  await act(async () => finishSave())
  await screen.findByText('글 작성 모델을 “Writer B”으로 저장했어요.')
  expect(calls.filter((name) => /^(Save|Start|Estimate)/.test(name))).toEqual(['SaveSelection'])
})

it('distinguishes a loading selection from a failed read and offers read-only retry', async () => {
  let finishRead!: () => void
  let failing = true
  const calls: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.listModels, () =>
      create(ProviderService.method.listModels.output, { models: [] }),
    )
    rpc(ProviderService.method.getSelections, async () => {
      calls.push('GetSelections')
      await new Promise<void>((resolve) => {
        finishRead = resolve
      })
      if (failing) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
      return create(ProviderService.method.getSelections.output, { selections: [] })
    })
  })
  render(<ActiveModelForm stage="write" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(await screen.findByText('글 작성 모델 설정을 확인하고 있어요.')).toBeInTheDocument()
  expect(screen.queryByText('글 작성에 사용할 모델을 아직 선택하지 않았어요.')).toBeNull()
  await waitFor(() => expect(calls).toHaveLength(1))
  await act(async () => finishRead())
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '글 작성 모델 설정을 불러오지 못했어요.',
  )
  expect(screen.queryByText('글 작성에 사용할 모델을 아직 선택하지 않았어요.')).toBeNull()
  failing = false
  await userEvent.click(screen.getByRole('button', { name: '모델 설정 다시 확인하기' }))
  await waitFor(() => expect(calls).toHaveLength(2))
  await act(async () => finishRead())
  expect(
    await screen.findByText('글 작성에 사용할 모델을 아직 선택하지 않았어요.'),
  ).toBeInTheDocument()
  expect(calls).toEqual(['GetSelections', 'GetSelections'])
})

it('keeps an unavailable saved model named until an explicit replacement is chosen', async () => {
  const calls: string[] = []
  render(<ActiveModelForm stage="write" />, {
    wrapper: withProviders(
      createFakeProviderTransport({
        calls,
        models: [
          {
            ...models[0]!,
            access: {
              [Stage.WRITE]: {
                grade: 'premium',
                requiredPlan: 'pro',
                entitled: false,
                unavailableReason: 'MODEL_PLAN_REQUIRED',
              },
            },
          },
        ],
        selections: [
          {
            stage: Stage.WRITE,
            providerId: 'openrouter',
            modelId: 'a',
            requiredPlan: 'pro',
            unavailableReason: 'MODEL_PLAN_REQUIRED',
          },
        ],
      }),
      createTestQueryClient(),
    ),
  })
  await screen.findByText(/저장한 글 작성 모델 “Writer A”은 지금 사용할 수 없어요/)
  expect(screen.getByRole('combobox')).toHaveTextContent('Writer A')
  expect(calls).not.toContain('SaveSelection')
})
