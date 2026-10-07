import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import {
  InspectionStatus,
  ProtoAuthoringMode,
  ProtoConfigurationKind,
  RequestInspectionSchema,
  WritingInspectionService as Service,
} from '@/shared/api'
import type { RequestInspectionTarget } from '../model/types'
import { createRequestInspectionReader } from './client'
import { capturedInspection, postTarget } from './fixtures.test-support'

const selection = { stage: 'post-writing', status: 'captured' } as const

describe('request inspection read client', () => {
  it('covers every generated authoring kind/mode and every selectable inspection status at the real RPC boundary', async () => {
    const kinds = new Set<number>()
    const modes = new Set<number>()
    const statuses = new Set<number>()
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getAuthoringRequestInspection, (request) => {
        kinds.add(request.kind)
        modes.add(request.mode)
        statuses.add(request.status)
        return create(Service.method.getAuthoringRequestInspection.output, {})
      })
    })
    const read = createRequestInspectionReader(transport)
    for (const authoringKind of [
      'post-template',
      'video-template',
      'post-guideline',
      'video-guideline',
      'writing-voice',
    ] as const)
      for (const mode of ['recommend', 'refine'] as const)
        for (const status of ['current', 'prepared', 'captured'] as const)
          await read(
            {
              ownerId: 'alice',
              kind: 'authoring',
              sessionId: 'session',
              authoringKind,
              revision: 0,
              mode,
              candidateCount: 1,
            },
            { stage: 'setting-authoring', status },
          )
    const values = (enumeration: Record<string, string | number>) =>
      Object.values(enumeration).filter((value): value is number => typeof value === 'number')
    expect([...kinds].sort()).toEqual(
      values(ProtoConfigurationKind)
        .filter((value) => value !== ProtoConfigurationKind.UNSPECIFIED)
        .sort(),
    )
    expect([...modes].sort()).toEqual(
      values(ProtoAuthoringMode)
        .filter((value) => value !== ProtoAuthoringMode.UNSPECIFIED)
        .sort(),
    )
    // Unavailable is a read result; neither it nor unspecified can select a preview/history.
    expect([...statuses].sort()).toEqual(
      values(InspectionStatus)
        .filter(
          (value) =>
            value !== InspectionStatus.UNSPECIFIED && value !== InspectionStatus.UNAVAILABLE,
        )
        .sort(),
    )
  })

  it('preserves all actual safe captured calls in server order with the same latest identity', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getPostRequestInspection, (request) => {
        expect(request).toMatchObject({
          postSlug: 'owned-post',
          stage: 'post-writing',
          status: InspectionStatus.CAPTURED,
        })
        return create(Service.method.getPostRequestInspection.output, {
          inspection: capturedInspection('second'),
          inspections: [capturedInspection('first'), capturedInspection('second')],
        })
      })
    })
    const result = await createRequestInspectionReader(transport)(postTarget, selection)
    expect(result.inspections.map((call) => call.callId)).toEqual(['first', 'second'])
    expect(result.inspection).toEqual(result.inspections[1])
    expect(result.inspection.fragments[0]?.text).toBe('owner private memo')
    expect(result.inspection.conditions?.model).toEqual({
      providerId: 'fake',
      modelId: 'private-model',
    })
    expect(result.inspection).not.toHaveProperty('$typeName')
  })

  it('keeps missing old history and unsupported payloads unavailable without inventing past requests', async () => {
    let reads = 0
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getPostRequestInspection, () => {
        reads++
        if (reads === 1) return create(Service.method.getPostRequestInspection.output, {})
        const malformed = capturedInspection()
        malformed.version = 999
        return create(Service.method.getPostRequestInspection.output, { inspection: malformed })
      })
    })
    const read = createRequestInspectionReader(transport)
    for (const result of [await read(postTarget, selection), await read(postTarget, selection)]) {
      expect(result.inspections).toHaveLength(1)
      expect(result.inspection.status).toBe('unavailable')
      expect(result.inspection.fragments).toEqual([])
      expect(result.inspection.callId).toBeUndefined()
      expect(result.inspection.conditions).toBeUndefined()
    }
  })

  it.each(['current', 'prepared'] as const)(
    'preserves a truthful %s preview without an issued identity',
    async (status) => {
      const transport = createRouterTransport(({ rpc }) => {
        rpc(Service.method.getPostRequestInspection, () => {
          const inspection = capturedInspection()
          inspection.status =
            status === 'current' ? InspectionStatus.CURRENT : InspectionStatus.PREPARED
          inspection.callId = ''
          inspection.issuedAt = undefined
          return create(Service.method.getPostRequestInspection.output, { inspection })
        })
      })
      const read = createRequestInspectionReader(transport)
      expect((await read(postTarget, { ...selection, status })).inspection.status).toBe(status)
      const history = await read(postTarget, selection)
      expect(history.inspection.status).toBe('unavailable')
      expect(history.inspection.callId).toBeUndefined()
    },
  )

  it('sends exact authoring owner-session inputs and does not use the legacy conflicting alias', async () => {
    const target: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'authoring',
      sessionId: 'session-1',
      authoringKind: 'post-template',
      revision: 7,
      mode: 'refine',
      prompt: 'owner explicit instruction',
      model: { providerId: 'fake', modelId: 'chosen' },
      candidateCount: 3,
    }
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getAuthoringRequestInspection, (request) => {
        expect(request).toMatchObject({
          draftId: '',
          sessionId: 'session-1',
          kind: ProtoConfigurationKind.POST_TEMPLATE,
          revision: 7,
          mode: ProtoAuthoringMode.REFINE,
          prompt: 'owner explicit instruction',
          model: target.model,
          candidateCount: 3,
          stage: 'setting-authoring',
          status: InspectionStatus.PREPARED,
        })
        return create(Service.method.getAuthoringRequestInspection.output, {})
      })
    })
    await createRequestInspectionReader(transport)(target, {
      stage: 'setting-authoring',
      status: 'prepared',
    })
  })

  it('consumes authoritative blind denial and drops every deep private field from that unavailable projection', async () => {
    const target: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'test',
      testId: 'blind-test',
      candidateId: 'candidate-a',
      revision: 2,
      blind: true,
    }
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getWritingTestRequestInspection, () => {
        // Represents a denied server projection with extraneous old private fields:
        // unavailable decoding must never recover those fields as material.
        const denied = capturedInspection('private-job')
        denied.status = InspectionStatus.UNAVAILABLE
        denied.mode = 'writing-test'
        denied.unavailableReason = 'blind_test_identity_hidden_until_reveal'
        denied.nativeFields = [
          {
            $typeName: 'postpilot.v1.RequestNativeField',
            id: 'private-style',
            authorship: 2,
            materialRole: 'private-voice',
            text: 'PRIVATE_VOICE_ID',
            sourceRefs: ['PRIVATE_TEMPLATE_ID'],
            sourceFiles: [],
            activation: 'PRIVATE_GUIDELINE_ID',
          },
        ]
        return create(Service.method.getWritingTestRequestInspection.output, {
          inspection: denied,
          inspections: [denied],
        })
      })
    })
    const result = await createRequestInspectionReader(transport)(target, selection)
    expect(result.inspection.status).toBe('unavailable')
    expect(result.inspection.unavailableReason).toBe('blind_test_identity_hidden_until_reveal')
    expect(result.inspection.mode).toBe('writing-test')
    expect(result.inspection.fragments).toEqual([])
    expect(result.inspection.nativeFields).toBeUndefined()
    expect(result.inspection.conditions).toBeUndefined()
    expect(result.inspection.callId).toBeUndefined()
    expect(JSON.stringify(result)).not.toMatch(
      /PRIVATE_|private-model|owner private memo|private-job/,
    )
  })

  it('allows safe captured test detail only after an explicit revealed target identity', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getWritingTestRequestInspection, () =>
        create(Service.method.getWritingTestRequestInspection.output, {
          inspection: capturedInspection(),
          inspections: [capturedInspection()],
        }),
      )
    })
    const target: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'test',
      testId: 'test',
      candidateId: 'entrant',
      revision: 3,
      blind: false,
    }
    const result = await createRequestInspectionReader(transport)(target, selection)
    expect(result.inspection.status).toBe('captured')
    expect(result.inspection.callId).toBe('job:write:1')
  })

  it('retains a safe server explanation for a missing, stale or purged capture', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getPostRequestInspection, () =>
        create(Service.method.getPostRequestInspection.output, {
          inspection: create(RequestInspectionSchema, {
            version: 1,
            status: InspectionStatus.UNAVAILABLE,
            stage: 'post-writing',
            unavailableReason: 'capture_missing_stale_or_purged',
          }),
        }),
      )
    })
    const result = await createRequestInspectionReader(transport)(postTarget, selection)
    expect(result.inspection.unavailableReason).toBe('capture_missing_stale_or_purged')
    expect(result.inspection.fragments).toEqual([])
  })
})
