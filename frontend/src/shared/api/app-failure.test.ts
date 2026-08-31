import { Code, ConnectError } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'

import { AppErrorDetailSchema, FailureSchema } from './gen/postpilot/v1/error_pb'
import { appFailureFromConnect, appFailureFromProto, normalizeAppFailure } from './app-failure'

describe('application failure boundary', () => {
  it('decodes one known Connect detail without consulting rawMessage', () => {
    const error = new ConnectError('private backend prose', Code.InvalidArgument, undefined, [
      {
        desc: AppErrorDetailSchema,
        value: {
          reason: 'VOICE_SAMPLE_TOO_SHORT',
          params: { actual: '20', min: '200' },
        },
      },
    ])

    expect(appFailureFromConnect(error)).toEqual({
      reason: 'VOICE_SAMPLE_TOO_SHORT',
      params: { actual: '20', min: '200' },
    })
  })

  const malformedDetails: Array<{
    reason: string
    params: Record<string, string>
  }> = [
    { reason: 'NOT_REGISTERED', params: {} },
    { reason: 'VOICE_SAMPLE_TOO_SHORT', params: { actual: '20' } },
    {
      reason: 'VOICE_SAMPLE_TOO_SHORT',
      params: { actual: '20', min: '200', leaked: 'secret' },
    },
  ]

  it.each(malformedDetails)('maps unknown or malformed detail to the generic reason', (detail) => {
    expect(normalizeAppFailure(detail)).toEqual({
      reason: 'UNKNOWN_FAILURE',
      params: {},
    })
  })

  it('maps missing or duplicate Connect details to the generic reason', () => {
    expect(appFailureFromConnect(new ConnectError('raw only', Code.NotFound))).toEqual({
      reason: 'UNKNOWN_FAILURE',
      params: {},
    })

    const detail = {
      desc: AppErrorDetailSchema,
      value: { reason: 'POST_NOT_FOUND', params: {} },
    } as const
    expect(
      appFailureFromConnect(
        new ConnectError('ambiguous', Code.NotFound, undefined, [detail, detail]),
      ),
    ).toEqual({ reason: 'UNKNOWN_FAILURE', params: {} })
  })

  it('keeps optional technical detail as inert text on a durable failure', () => {
    const failure = create(FailureSchema, {
      reason: 'MODEL_UNAVAILABLE',
      params: {},
      technicalDetail: '<img src=x onerror=alert(1)>',
    })

    expect(appFailureFromProto(failure)).toEqual({
      reason: 'MODEL_UNAVAILABLE',
      params: {},
      technicalDetail: '<img src=x onerror=alert(1)>',
    })
  })
})
