import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'
import {
  AppErrorDetailSchema,
  FailureSchema,
  type AppFailureReason,
  type ProtoFailure,
} from '@/shared/api'

export function connectAppError(
  reason: AppFailureReason,
  code: Code,
  params: Record<string, string> = {},
): ConnectError {
  return new ConnectError('private backend prose', code, undefined, [
    { desc: AppErrorDetailSchema, value: { reason, params } },
  ])
}

export function durableFailure(
  reason: AppFailureReason,
  params: Record<string, string> = {},
  technicalDetail = '',
): ProtoFailure {
  return create(FailureSchema, { reason, params, technicalDetail })
}
