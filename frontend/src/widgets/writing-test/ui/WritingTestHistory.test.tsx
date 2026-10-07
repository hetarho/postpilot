import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, expect, it } from 'vitest'
import {
  ModelExperimentService,
  WritingTestService as Service,
  VoiceService,
  WritingTestFactor,
  WritingTestStage,
  WritingTestStatus,
  WritingTestCandidateStatus,
  BlockType,
  contentLanguageToProto,
  Stage,
  ExperimentStatus,
  ExperimentSource,
  ExperimentOrigin,
  ProtoVoiceCheckStatus,
  ProtoConfigurationKind,
} from '@/shared/api'
import { writingTestI18n } from '@/features/writing-test'
import { createTestQueryClient, withProviders } from '@/test/session'
import { WritingTestHistory } from './WritingTestHistory'

i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
i18next.addResourceBundle('en', 'writingTests', writingTestI18n.en, true, true)
afterEach(cleanup)
function test(id: string, revision: number, name: string, createdAt: string) {
  return {
    id,
    revision,
    factor: WritingTestFactor.MODEL,
    modelStage: WritingTestStage.WRITE,
    count: 2,
    status: WritingTestStatus.COMPLETED,
    targetLanguage: contentLanguageToProto('ko'),
    createdAt,
    candidates: ['left', 'right'].map((side) => ({
      id: `${id}-${side}`,
      status: WritingTestCandidateStatus.SUCCEEDED,
      output: { title: name, blocks: [{ type: BlockType.TEXT, content: 'Stored full post' }] },
      identity: {
        label: `${name} ${side}`,
        source: {
          source: {
            case: 'model' as const,
            value: { providerId: 'provider', modelId: `${id}-${side}` },
          },
        },
      },
    })),
    matches: [
      {
        id: `${id}-match`,
        round: 1,
        index: 0,
        leftCandidateId: `${id}-left`,
        rightCandidateId: `${id}-right`,
        winnerCandidateId: `${id}-left`,
      },
    ],
    winnerCandidateId: `${id}-left`,
    revealed: true,
  }
}
function registerEmptyLegacy(
  rpc: Parameters<Parameters<typeof createRouterTransport>[0]>[0]['rpc'],
) {
  rpc(ModelExperimentService.method.listExperiments, () =>
    create(ModelExperimentService.method.listExperiments.output, {}),
  )
}
it('retains all loaded pages, deduplicates overlaps monotonically, and uses real resume URLs without starting work', async () => {
  const pages: string[] = []
  let voiceReads = 0
  const transport = createRouterTransport(({ rpc }) => {
    registerEmptyLegacy(rpc)
    rpc(VoiceService.method.listVoiceChecks, () => {
      voiceReads++
      return create(VoiceService.method.listVoiceChecks.output, {})
    })
    rpc(Service.method.listWritingTests, (request) => {
      pages.push(request.pageToken)
      return create(
        Service.method.listWritingTests.output,
        request.pageToken
          ? {
              tests: [
                test('older', 2, 'Older', '2026-10-06T00:00:00Z'),
                test('newer', 1, 'Stale', '2026-10-07T00:00:00Z'),
              ],
            }
          : {
              tests: [test('newer', 5, 'Current', '2026-10-07T00:00:00Z')],
              nextPageToken: 'next-page',
            },
      )
    })
  })
  render(<WritingTestHistory ownerId="alice" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(await screen.findByText('Current left')).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: '기록 더 보기' }))
  expect(await screen.findByText('Older left')).toBeVisible()
  expect(screen.getByText('Current left')).toBeVisible()
  expect(screen.queryByText('Stale left')).not.toBeInTheDocument()
  expect(
    screen
      .getAllByRole('link', { name: '테스트 이어보기' })
      .map((link) => link.getAttribute('href')),
  ).toEqual(['/tests/newer?entry=%2Ftests%2Fhistory', '/tests/older?entry=%2Ftests%2Fhistory'])
  expect(pages).toEqual(['', 'next-page'])
  expect(voiceReads).toBe(0)
})
it('shows durable champion and expired metadata beside paid legacy records without inventing legacy brackets', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.listWritingTests, () => {
      const record = test('expired', 5, 'Frozen champion', '2026-10-07T00:00:00Z')
      return create(Service.method.listWritingTests.output, {
        tests: [
          {
            ...record,
            contentExpiresAt: '2020-01-01T00:00:00Z',
            candidates: record.candidates.map((candidate) => ({ ...candidate, output: undefined })),
          },
        ],
      })
    })
    rpc(ModelExperimentService.method.listExperiments, () =>
      create(ModelExperimentService.method.listExperiments.output, {
        experiments: [
          {
            id: 'old-three-way',
            stage: Stage.WRITE,
            status: ExperimentStatus.DECIDED,
            source: ExperimentSource.POST,
            origin: ExperimentOrigin.LAB,
            templateName: 'Old paid template',
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
      }),
    )
  })
  render(<WritingTestHistory ownerId="alice" testHref={(id) => `/tests/${id}?from=settings`} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(await screen.findByText('Frozen champion left')).toBeVisible()
  expect(await screen.findByText('Old paid template')).toBeVisible()
  expect(screen.getByText(/글 결과의 보관 기간이 지났어요/)).toBeVisible()
  expect(screen.getByRole('link', { name: '테스트 이어보기' })).toHaveAttribute(
    'href',
    '/tests/expired?from=settings',
  )
  expect(screen.getByRole('link', { name: '계속 보기' })).toHaveAttribute(
    'href',
    '/tests/records/old-three-way?entry=%2Ftests%2Fhistory',
  )
  expect(screen.getAllByText('1 / 1번 선택 완료')).toHaveLength(1)
  expect(
    screen.queryByRole('button', { name: /승자|활성 모델|저장|다시 만들기/ }),
  ).not.toBeInTheDocument()
})
it('loads historical voice checks only for one explicit voice and keeps the result read-only', async () => {
  const voiceIds: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    registerEmptyLegacy(rpc)
    rpc(Service.method.listWritingTests, () => create(Service.method.listWritingTests.output, {}))
    rpc(VoiceService.method.listVoiceChecks, (request) => {
      voiceIds.push(request.voiceId)
      return create(VoiceService.method.listVoiceChecks.output, {
        checks: [
          {
            id: 'old-check',
            status: ProtoVoiceCheckStatus.DONE,
            piece: 'Paid historical writing',
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
      })
    })
  })
  render(<WritingTestHistory ownerId="alice" voiceId="one-voice" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(await screen.findByText('Paid historical writing')).toBeVisible()
  expect(voiceIds).toEqual(['one-voice'])
  expect(screen.queryByRole('button', { name: /검증|재분석|다시 만들기/ })).not.toBeInTheDocument()
})
it('distinguishes read failures from empty history and refreshes while preserving the action label', async () => {
  let fail = true
  const transport = createRouterTransport(({ rpc }) => {
    registerEmptyLegacy(rpc)
    rpc(Service.method.listWritingTests, () => {
      if (fail) throw new ConnectError('temporary', Code.Unavailable)
      return create(Service.method.listWritingTests.output, {
        tests: [test('recovered', 5, 'Recovered', '2026-10-07T00:00:00Z')],
      })
    })
  })
  render(<WritingTestHistory ownerId="alice" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(await screen.findByRole('alert')).toHaveTextContent('요청을 완료하지 못했어요')
  fail = false
  await userEvent.click(screen.getAllByRole('button', { name: '현재 결과 확인' })[0])
  expect(await screen.findByText('Recovered left')).toBeVisible()
})
it('clears prior-owner pages immediately and fences late reads on an account change', async () => {
  let activeOwner = 'alice'
  let started = false
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    registerEmptyLegacy(rpc)
    rpc(Service.method.listWritingTests, async () => {
      const captured = activeOwner
      if (captured === 'alice') {
        started = true
        await gate
      }
      return create(Service.method.listWritingTests.output, {
        tests: [test(captured, 1, captured, '2026-10-07T00:00:00Z')],
      })
    })
  })
  const wrapper = withProviders(transport, createTestQueryClient())
  const mounted = render(<WritingTestHistory ownerId="alice" />, { wrapper })
  await waitFor(() => expect(started).toBe(true))
  activeOwner = 'bob'
  mounted.rerender(<WritingTestHistory ownerId="bob" />)
  expect(await screen.findByText('bob left')).toBeVisible()
  await act(async () => {
    release()
    await gate
  })
  expect(screen.queryByText('alice left')).not.toBeInTheDocument()
  expect(screen.getByText('bob left')).toBeVisible()
})

it('applies source and stage to real records across pages and passes matching legacy read filters', async () => {
  const pages: string[] = []
  const legacyReads: Array<{ stage: Stage; source: ExperimentSource }> = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.listWritingTests, (request) => {
      pages.push(request.pageToken)
      return create(Service.method.listWritingTests.output, {
        tests: request.pageToken
          ? [
              {
                ...test('match', 1, 'Matching source', '2026-10-06T00:00:00Z'),
                modelStage: WritingTestStage.OBSERVE,
                sourcePostSlug: 'owned-source',
              },
            ]
          : [
              {
                ...test('wrong-stage', 1, 'Wrong stage', '2026-10-07T00:00:00Z'),
                sourcePostSlug: 'owned-source',
              },
              {
                ...test('wrong-source', 1, 'Wrong source', '2026-10-07T00:00:00Z'),
                modelStage: WritingTestStage.OBSERVE,
                sourcePostSlug: 'different-source',
              },
            ],
        nextPageToken: request.pageToken ? '' : 'next',
      })
    })
    rpc(ModelExperimentService.method.listExperiments, (request) => {
      legacyReads.push({ stage: request.stage, source: request.source })
      return create(ModelExperimentService.method.listExperiments.output, {
        experiments: [
          {
            id: 'paid-match',
            stage: Stage.OBSERVE,
            source: ExperimentSource.POST,
            status: ExperimentStatus.DECIDED,
            origin: ExperimentOrigin.LAB,
            postSlug: 'owned-source',
            templateName: 'Matching paid source',
          },
          {
            id: 'paid-other',
            stage: Stage.OBSERVE,
            source: ExperimentSource.POST,
            status: ExperimentStatus.DECIDED,
            origin: ExperimentOrigin.LAB,
            postSlug: 'another-source',
            templateName: 'Different paid source',
          },
        ],
      })
    })
  })
  render(<WritingTestHistory ownerId="alice" stage="observe" sourcePostSlug="owned-source" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await screen.findByRole('button', { name: '기록 더 보기' })
  expect(screen.queryByText('Wrong stage left')).not.toBeInTheDocument()
  expect(screen.queryByText('Wrong source left')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '기록 더 보기' }))
  expect(await screen.findByText('Matching source left')).toBeVisible()
  expect(await screen.findByText('Matching paid source')).toBeVisible()
  expect(screen.queryByText('Different paid source')).not.toBeInTheDocument()
  expect(pages).toEqual(['', 'next'])
  expect(legacyReads).toEqual([{ stage: Stage.OBSERVE, source: ExperimentSource.POST }])
  const href = screen.getByRole('link', { name: '테스트 이어보기' }).getAttribute('href')!
  expect(new URL(href, 'https://fixture').searchParams.get('entry')).toBe(
    '/tests/history?stage=observe&source=owned-source',
  )
  const legacyHref = screen.getByRole('link', { name: '계속 보기' }).getAttribute('href')!
  expect(new URL(legacyHref, 'https://fixture').pathname).toBe('/tests/records/paid-match')
  expect(new URL(legacyHref, 'https://fixture').searchParams.get('entry')).toBe(
    '/tests/history?stage=observe&source=owned-source',
  )
})

it('filters one voice only from revealed identities and never infers a blind candidate voice', async () => {
  const voiceRecord = (id: string, voiceId: string) => {
    const record = test(id, 1, id, '2026-10-07T00:00:00Z')
    return {
      ...record,
      factor: WritingTestFactor.VOICE,
      candidates: record.candidates.map((candidate, index) => ({
        ...candidate,
        identity: {
          label: `${id} ${index === 0 ? 'left' : 'right'}`,
          source: {
            source: {
              case: 'setting' as const,
              value: {
                kind: ProtoConfigurationKind.WRITING_VOICE,
                id: index ? `${voiceId}-other` : voiceId,
                revision: 'frozen-1',
              },
            },
          },
        },
      })),
    }
  }
  const voiceReads: string[] = []
  const legacyReads: Array<{ stage: Stage; source: ExperimentSource }> = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.listWritingTests, () => {
      const blind = voiceRecord('blind', 'chosen')
      return create(Service.method.listWritingTests.output, {
        tests: [
          voiceRecord('match', 'chosen'),
          voiceRecord('other', 'another'),
          {
            ...blind,
            status: WritingTestStatus.REVIEW,
            revealed: false,
            winnerCandidateId: '',
            matches: blind.matches.map((match) => ({ ...match, winnerCandidateId: '' })),
          },
        ],
      })
    })
    rpc(ModelExperimentService.method.listExperiments, (request) => {
      legacyReads.push({ stage: request.stage, source: request.source })
      return create(ModelExperimentService.method.listExperiments.output, {
        experiments: [
          {
            id: 'legacy-chosen',
            stage: Stage.WRITE,
            source: ExperimentSource.VOICE,
            voiceId: 'chosen',
            status: ExperimentStatus.DECIDED,
            origin: ExperimentOrigin.LAB,
            templateName: 'Chosen paid voice',
          },
          {
            id: 'legacy-other',
            stage: Stage.WRITE,
            source: ExperimentSource.VOICE,
            voiceId: 'another',
            status: ExperimentStatus.DECIDED,
            origin: ExperimentOrigin.LAB,
            templateName: 'Other paid voice',
          },
        ],
      })
    })
    rpc(VoiceService.method.listVoiceChecks, (request) => {
      voiceReads.push(request.voiceId)
      return create(VoiceService.method.listVoiceChecks.output, {})
    })
  })
  render(<WritingTestHistory ownerId="alice" stage="voice" voiceId="chosen" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(await screen.findByText('match left')).toBeVisible()
  expect(await screen.findByText('Chosen paid voice')).toBeVisible()
  expect(screen.queryByText('other left')).not.toBeInTheDocument()
  expect(screen.queryByText('Other paid voice')).not.toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: '테스트 이어보기' })).toHaveLength(1)
  expect(legacyReads).toEqual([{ stage: Stage.WRITE, source: ExperimentSource.VOICE }])
  await waitFor(() => expect(voiceReads).toEqual(['chosen']))
})
