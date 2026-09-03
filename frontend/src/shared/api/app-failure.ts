import { ConnectError } from '@connectrpc/connect'

import type { AppErrorDetail, Failure as ProtoFailure } from './gen/postpilot/v1/error_pb'
import { AppErrorDetailSchema } from './gen/postpilot/v1/error_pb'

type FailureParamSpec = Readonly<{
  required?: readonly string[]
  optional?: readonly string[]
}>

// This is the browser's allowlist for the public failure contract. A backend reason is
// intentionally not displayable until it is registered here and in both locale catalogs.
export const appFailureSpecs = {
  UNKNOWN_FAILURE: {},
  AUTH_REQUIRED: {},
  INVALID_CREDENTIALS: {},
  POST_NOT_FOUND: {},
  POST_FORBIDDEN: {},
  POST_BUSY: { optional: ['active_job_id'] },
  POST_PUBLISHING: {},
  POST_CONTENT_STALE: {},
  POST_CONTENT_INVALID: {},
  POST_NOT_FINALIZED: {},
  POST_MACHINE_BASELINE_REQUIRED: {},
  POST_TARGET_LANGUAGE_REQUIRED: {},
  POST_TARGET_LANGUAGE_UNSUPPORTED: {},
  POST_FILENAME_TAKEN: { optional: ['filename'] },
  UPLOAD_INVALID: {},
  UPLOAD_NOT_FOUND: {},
  UPLOAD_OBJECT_MISSING: {},
  VOICE_REQUIRED: {},
  VOICE_NOT_FOUND: {},
  VOICE_DELETED: {},
  VOICE_NAME_REQUIRED: {},
  VOICE_NAME_TOO_LONG: { required: ['actual', 'max'] },
  VOICE_DESCRIPTION_TOO_LONG: { required: ['actual', 'max'] },
  VOICE_NAME_TAKEN: {},
  VOICE_DEFAULT_DELETE_FORBIDDEN: {},
  VOICE_BUSY: {},
  VOICE_BASELINE_MISMATCH: {},
  VOICE_SOURCE_LANGUAGE_REQUIRED: {},
  VOICE_SOURCE_LANGUAGE_UNSUPPORTED: {},
  VOICE_CONTENT_LANGUAGE_MISMATCH: {
    optional: ['content_language', 'source_language'],
  },
  VOICE_SAMPLE_TOO_SHORT: { required: ['actual', 'min'] },
  VOICE_SAMPLE_NOT_FOUND: {},
  VOICE_SAMPLE_MUTATION_FAILED: {},
  VOICE_PROFILE_FIELD_REQUIRED: {},
  VOICE_FEEDBACK_INVALID: {},
  VOICE_ANALYZE_MODEL_REQUIRED: {},
  VOICE_LEARNING_NOT_FOUND: {},
  VOICE_RULE_NOT_FOUND: {},
  VOICE_CONFIRMATION_NOT_FOUND: {},
  VOICE_COMPARISON_NOT_FOUND: {},
  VOICE_VALIDATION_NOT_FOUND: {},
  VOICE_INSUFFICIENT_SOURCES: { required: ['min'] },
  VOICE_INVALID_LIFECYCLE: {},
  PURPOSE_NOT_FOUND: {},
  PURPOSE_NAME_REQUIRED: {},
  PURPOSE_INSTRUCTIONS_REQUIRED: {},
  PURPOSE_NAME_TAKEN: {},
  PURPOSE_FIELD_TOO_LONG: { required: ['actual', 'max'], optional: ['field'] },
  GUIDELINE_NOT_FOUND: {},
  GUIDELINE_TEXT_REQUIRED: {},
  GUIDELINE_TEXT_TOO_LONG: { required: ['actual', 'max'] },
  GUIDELINE_TEXT_TAKEN: {},
  GUIDELINE_SCOPE_INVALID: {},
  GUIDELINE_PURPOSE_NOT_FOUND: {},
  GUIDELINE_LIMIT_REACHED: { required: ['max'] },
  MODEL_STAGE_REQUIRED: {},
  MODEL_STAGE_INVALID: {},
  MODEL_NOT_REGISTERED: {},
  MODEL_DISABLED: {},
  MODEL_UNSUITABLE: {},
  MODEL_CANDIDATES_DUPLICATE: {},
  MODEL_RECOMMENDATION_NOT_FOUND: {},
  // A recommendation set names nine refs, so its refusal names every one that blocks it —
  // grouped by cause, because "retired" and "unusable here" are different problems.
  MODEL_SET_UNAVAILABLE: {
    required: ['models'],
    optional: ['unregistered', 'disabled', 'unsuitable'],
  },
  MODEL_NOT_FOUND: {},
  MODEL_ID_REQUIRED: {},
  MODEL_REASONING_INVALID: {},
  GENERATION_WRITE_MODEL_REQUIRED: {},
  GENERATION_OBSERVE_MODEL_REQUIRED: {},
  GENERATION_TARGET_LENGTH_INVALID: {},
  GENERATION_ALREADY_RUNNING: { optional: ['active_job_id'] },
  GENERATION_VOICE_MISMATCH: {},
  REVISION_INSTRUCTION_REQUIRED: {},
  REVISION_INSTRUCTION_TOO_LONG: { required: ['max'] },
  REVISION_CONTENT_REQUIRED: {},
  CONTENT_LANGUAGE_REQUIRED: {},
  EXPERIMENT_NOT_FOUND: {},
  EXPERIMENT_FORBIDDEN: {},
  EXPERIMENT_STAGE_INVALID: {},
  EXPERIMENT_MODELS_REQUIRED: {},
  EXPERIMENT_CANDIDATES_DUPLICATE: {},
  EXPERIMENT_TARGET_LENGTH_INVALID: {},
  EXPERIMENT_STATE_INVALID: {},
  EXPERIMENT_CANDIDATE_NOT_FOUND: {},
  EXPERIMENT_CONFIRMATION_REQUIRED: {},
  EXPERIMENT_SNAPSHOT_UNAVAILABLE: {},
  EXPERIMENT_RETRY_MODEL_UNAVAILABLE: {},
  EXPERIMENT_VOICE_REQUIRED: {},
  EXPERIMENT_VOICE_UNAVAILABLE: {},
  EXPERIMENT_ALREADY_RUNNING: { optional: ['active_job_id'] },
  JOB_NOT_FOUND: {},
  JOB_FORBIDDEN: {},
  JOB_INTERRUPTED: {},
  JOB_PANICKED: {},
  JOB_HANDLER_MISSING: {},
  PROVIDER_DISABLED: {},
  MODEL_UNAVAILABLE: {},
  MODEL_RATE_LIMITED: {},
  MODEL_UNSUPPORTED: {},
  MODEL_OUTPUT_INVALID: {},
  MODEL_OUTPUT_TRUNCATED: {},
  PUBLISH_NOT_FOUND: {},
  PUBLISH_FORBIDDEN: {},
  PUBLISH_REQUEST_INVALID: {},
  PUBLISH_PAIRING_LIMIT: {},
  PUBLISH_PAIRING_INVALID: {},
  PUBLISH_AGENT_REVOKED: {},
  PUBLISH_AGENT_NOT_READY: {},
  PUBLISH_CATEGORY_NOT_FOUND: {},
  PUBLISH_POST_NOT_FINALIZED: {},
  PUBLISH_STALE_REVISION: {},
  PUBLISH_ALREADY_EXISTS: {},
  PUBLISH_LEASE_INVALID: {},
  PUBLISH_TRANSITION_INVALID: {},
  PUBLISH_COMMIT_FENCE: {},
  PUBLISH_URL_INVALID: {},
  PUBLISH_AGENT_UNAVAILABLE: {},
  PUBLISH_NEEDS_ATTENTION: {},
  PUBLISH_OUTCOME_UNKNOWN: {},
  NETWORK_UNAVAILABLE: {},
  // Plan enforcement (plan 17). The two budget axes carry micro-USD integers and the count
  // axis carries a plain count; both are rendered through the catalogs' formatters, so the
  // server never has to guess the reader's currency or timezone.
  INSUFFICIENT_CREDITS: { required: ['required', 'balance', 'renews_at'] },
  PLAN_REQUIRED: {},
  MASTER_ONLY: {},
  LAST_MASTER: {},
  USER_NOT_FOUND: {},
  USER_ID_REQUIRED: {},
} as const satisfies Readonly<Record<string, FailureParamSpec>>

export type AppFailureReason = keyof typeof appFailureSpecs

export interface AppFailure {
  readonly reason: AppFailureReason
  readonly params: Readonly<Record<string, string>>
  readonly technicalDetail?: string
}

const unknownFailure: AppFailure = {
  reason: 'UNKNOWN_FAILURE',
  params: {},
}

function isKnownReason(reason: string): reason is AppFailureReason {
  return Object.hasOwn(appFailureSpecs, reason)
}

function validateParams(
  reason: AppFailureReason,
  params: Readonly<Record<string, string>>,
): Readonly<Record<string, string>> | undefined {
  const spec: FailureParamSpec = appFailureSpecs[reason]
  const required = new Set(spec.required ?? [])
  const allowed = new Set([...(spec.required ?? []), ...(spec.optional ?? [])])

  for (const key of required) {
    if (!Object.hasOwn(params, key) || typeof params[key] !== 'string') return undefined
  }
  for (const [key, value] of Object.entries(params)) {
    if (!allowed.has(key) || typeof value !== 'string') return undefined
  }
  return Object.freeze({ ...params })
}

export function normalizeAppFailure(
  value: Pick<AppErrorDetail, 'reason' | 'params'> & { technicalDetail?: string },
): AppFailure {
  if (!isKnownReason(value.reason)) {
    return value.technicalDetail
      ? { ...unknownFailure, technicalDetail: value.technicalDetail }
      : unknownFailure
  }
  const params = validateParams(value.reason, value.params)
  if (!params) {
    return value.technicalDetail
      ? { ...unknownFailure, technicalDetail: value.technicalDetail }
      : unknownFailure
  }
  return {
    reason: value.reason,
    params,
    ...(value.technicalDetail ? { technicalDetail: value.technicalDetail } : {}),
  }
}

export function appFailureFromConnect(error: unknown): AppFailure {
  const details = ConnectError.from(error).findDetails(AppErrorDetailSchema)
  if (details.length !== 1) return unknownFailure
  return normalizeAppFailure(details[0])
}

export function appFailureFromProto(failure?: ProtoFailure): AppFailure {
  if (!failure) return unknownFailure
  return normalizeAppFailure({
    reason: failure.reason,
    params: failure.params,
    technicalDetail: failure.technicalDetail || undefined,
  })
}
