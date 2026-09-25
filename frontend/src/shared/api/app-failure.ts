import { Code, ConnectError } from '@connectrpc/connect'

import type { AppErrorDetail, Failure as ProtoFailure } from './gen/postpilot/v1/error_pb'
import { AppErrorDetailSchema, FailureReason } from './gen/postpilot/v1/error_pb'

type FailureParamSpec = Readonly<{
  required?: readonly string[]
  optional?: readonly string[]
}>

/** Every refusal the API can name, from the proto enum both sides compile against (T278).
 *
 *  The enum is the contract; this file and the two locale catalogues are typed by it, so a
 *  reason nobody gave params or copy to is a `tsc` error rather than the generic unknown
 *  message reaching a user (ARCH-3). */
export type AppFailureReason = keyof typeof FailureReason

// What params each reason is allowed to carry. Keyed by the enum, so adding a reason to the
// proto makes this object incomplete until it is given a spec.
export const appFailureSpecs = {
  UNKNOWN_FAILURE: {},
  AUTH_REQUIRED: {},
  INVALID_CREDENTIALS: {},
  TOO_MANY_ATTEMPTS: { required: ['retry_at'] },
  GOOGLE_SIGNIN_DISABLED: {},
  GOOGLE_EMAIL_UNVERIFIED: {},
  GOOGLE_ACCOUNT_MISMATCH: {},
  GOOGLE_SIGNIN_FAILED: {},
  INVALID_EMAIL: {},
  PASSWORD_TOO_SHORT: { required: ['min'] },
  PASSWORD_TOO_LONG: { required: ['max'] },
  VERIFICATION_LINK_INVALID: {},
  RESET_LINK_INVALID: {},
  EMAIL_ALREADY_VERIFIED: {},
  PASSWORD_NOT_SET: {},
  CURRENT_PASSWORD_WRONG: {},
  EMAIL_VERIFICATION_REQUIRED: {},
  CUSTOMER_KEY_MISMATCH: {},
  SUBSCRIPTION_NEEDS_METHOD: {},
  BILLING_UNAVAILABLE: {},
  TIER_NOT_SUBSCRIBABLE: {},
  BILLING_SELECTION_INVALID: {},
  SUBSCRIPTION_EXISTS: {},
  SUBSCRIPTION_REQUIRED: {},
  NO_CHANGE: {},
  NO_SCHEDULED_CHANGE: {},
  CHANGE_UNSUPPORTED: {},
  PAYMENT_METHOD_REQUIRED: {},
  CHARGE_FAILED: {},
  PURCHASE_TOO_SMALL: {},
  PURCHASE_NOT_FOUND: {},
  REFUND_WINDOW_CLOSED: {},
  PURCHASE_SPENT: {},
  REFUND_FAILED: {},
  POST_NOT_FOUND: {},
  POST_FORBIDDEN: {},
  POST_BUSY: { optional: ['active_job_id'] },
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
  CLIP_INVALID_INPUT: { optional: ['cut_id', 'check'] },
  CLIP_COPY_TOO_LONG: {},
  CLIP_INSUFFICIENT_FOOTAGE: {},
  CLIP_LAYOUT_SAFE_AREA: {},
  CLIP_LAYOUT_SIZE: {},
  CLIP_LAYOUT_OVERLAP: {},
  CLIP_LAYOUT_MOTION: {},
  CLIP_LAYOUT_ANCHOR_STEP: {},
  CLIP_LAYOUT_FREQUENCY: {},
  CLIP_LAYOUT_DISCLOSURE: {},
  CLIP_LAYOUT_KIND: {},
  CLIP_LAYOUT_CONTRAST: {},
  CLIP_DISCLOSURE_REQUIRED: {},
  CLIP_TARGET_DURATION_REQUIRED: {},
  CLIP_FACTS_REQUIRED: { required: ['labels'] },
  CLIP_PREVIEW_BUSY: {},
  CLIP_PREVIEW_TOO_LARGE: {},
  CLIP_PREVIEW_UNAVAILABLE: {},
  CLIP_PREVIEW_TIMEOUT: {},
  CLIP_SOURCE_UNAVAILABLE: {},
  CLIP_SOURCE_EXPIRED: {},
  CLIP_SOURCE_MISSING: {},
  CLIP_BUSY: {},
  CLIP_PLAN_CONFLICT: {},
  CLIP_COMPOSITION_INVALID: {
    required: ['element_id', 'line', 'reason'],
    optional: ['label', 'max', 'actual'],
  },
  CLIP_COMPOSITION_UNAVAILABLE: {},
  CLIP_INVALID_MEDIA: {},
  CLIP_INPUT_TOO_LARGE: {},
  CLIP_ANALYSIS_TOO_LARGE: {},
  CLIP_WORKSPACE_LIMIT: {},
  CLIP_MODEL_INPUT_UNSUPPORTED: {},
  CLIP_PROCESSING_FAILED: {},
  CLIP_MEDIA_UNAVAILABLE: {},
  CLIP_MEDIA_RETRY_EXHAUSTED: {},
  CLIP_MEDIA_TIMEOUT: {},
  CLIP_QUOTE_REQUIRED: {},
  CLIP_CANCELLATION_POLICY_REQUIRED: {},
  CLIP_FINALIZED: {},
  CLIP_FINALIZATION_CONFLICT: {},
  CLIP_FINALIZATION_INVALID: {},
  CLIP_QUOTE_EXPIRED: {},
  CLIP_QUOTE_CHANGED: {},
  CLIP_CREDIT_CEILING_EXCEEDED: { required: ['required', 'approved'] },
  CLIP_MODEL_PRICING_UNAVAILABLE: {},
  CLIP_MODEL_VIDEO_INPUT_ABSENT: { required: ['model'] },
  CLIP_MODEL_INLINE_ENDPOINT_UNAVAILABLE: { required: ['model'] },
  CLIP_MODEL_REQUIRED_PARAMETERS_UNSUPPORTED: { required: ['model'] },
  CLIP_MODEL_PRICE_CEILING_UNAVAILABLE: { required: ['model'] },
  CLIP_NOT_FOUND: {},
  CLIP_TEMPLATE_NAME_TAKEN: {},
  TEMPLATE_NOT_FOUND: {},
  TEMPLATE_NAME_REQUIRED: {},
  TEMPLATE_BODY_REQUIRED: {},
  TEMPLATE_NAME_TAKEN: {},
  TEMPLATE_LIMIT_REACHED: {},
  TEMPLATE_FIELD_TOO_LONG: { required: ['actual', 'max'], optional: ['field'] },
  // `max` is optional: the target length has a floor and no ceiling, because the post option
  // this number seeds has none either.
  TEMPLATE_NUMBER_OUT_OF_RANGE: { required: ['actual', 'min'], optional: ['field', 'max'] },
  // `area` names the part that failed once a template has a title area (TMPL-50); a refusal
  // without it is a body's, as every one was before.
  TEMPLATE_PARSE_FAILED: { required: ['line', 'reason'], optional: ['area'] },
  PURPOSE_NOT_FOUND: {},
  GUIDELINE_NOT_FOUND: {},
  GUIDELINE_TEXT_REQUIRED: {},
  GUIDELINE_TEXT_TOO_LONG: { required: ['actual', 'max'] },
  GUIDELINE_TEXT_TAKEN: {},
  GUIDELINE_SCOPE_INVALID: {},
  GUIDELINE_TEMPLATE_NOT_FOUND: {},
  GUIDELINE_FIELD_NOT_FOUND: {},
  GUIDELINE_LIMIT_REACHED: { required: ['max'] },
  GUIDELINE_CANDIDATE_NOT_FOUND: {},
  // 기억 (MEM r1). The cap and the two ceilings name their numbers, because the copy the
  // user reads has to say them and the client owns no copy of the bound.
  MEMORY_NOT_FOUND: {},
  MEMORY_TEXT_REQUIRED: {},
  MEMORY_TEXT_TOO_LONG: { required: ['actual', 'max'] },
  MEMORY_TEXT_TAKEN: {},
  MEMORY_KIND_INVALID: {},
  MEMORY_TAG_REQUIRED: {},
  MEMORY_TAGS_TOO_MANY: { required: ['actual', 'max'] },
  MEMORY_LIMIT_REACHED: { required: ['max'] },
  MEMORY_EXTRACTION_NOT_READY: {},
  MEMORY_ANALYZE_MODEL_REQUIRED: {},
  MODEL_STAGE_REQUIRED: {},
  MODEL_STAGE_INVALID: {},
  MODEL_NOT_REGISTERED: {},
  MODEL_PURPOSE_INVALID: {},
  MODEL_PURPOSE_INELIGIBLE: {},
  MODEL_PURPOSE_NOT_REGISTERED: {},
  // The operator's estimator-combo assignment (QUOTA-39): a combo that is not one of the
  // four, or a request missing the combo or one of its two models.
  COMBO_UNKNOWN: {},
  COMBO_INCOMPLETE: {},
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
  // The chosen observe model cannot watch a clip. The ref is allowlisted because the fix is to
  // pick another model and the message names the one that cannot (VIDEO-11).
  MODEL_VIDEO_UNSUPPORTED: { optional: ['model'] },
  POST_VIDEO_LIMIT: {},
  POST_PHOTO_LIMIT: {},
  // A data-field answer the client should have bounded itself: the write screen counts both
  // halves down, so these only appear when something bypassed it (TEMPLATE-43).
  POST_TEMPLATE_ANSWER_TOO_LONG: { required: ['max'], optional: ['field', 'actual'] },
  POST_TEMPLATE_ANSWER_INVALID: {},
  // 발행됨 and 분야 (POST r11, QUAL r3). None carries a param: each copy is fixed text.
  POST_PUBLISHED_LOCKED: {},
  POST_PUBLISHED_URL_INVALID: {},
  POST_FIELD_NOT_FOUND: {},
  POST_QUALITY_RULE_INVALID: {},
  // The list's paging and narrowing (POST r15): a request the browser never builds.
  POST_LIST_REQUEST_INVALID: {},
  VOUCHER_INVALID: {},
  VOUCHER_NOT_FOUND: {},
  VOUCHER_REDEEMED: {},
  VOUCHER_EXPIRED: {},
  VOUCHER_REVOKED: {},
  UPLOAD_VIDEO_UNSUPPORTED: {},
  UPLOAD_VIDEO_INVALID: {},
  GENERATION_TARGET_LENGTH_INVALID: {},
  POST_TAG_COUNT_INVALID: {},
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
  EXPERIMENT_POST_FINALIZED: {},
  EXPERIMENT_BADGES_INVALID: {},
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
} as const satisfies Readonly<Record<AppFailureReason, FailureParamSpec>>

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

/** Whether the same request is worth sending again. A refusal the server will repeat for the
 *  same body (invalid, conflict, not found, refused) is not; a connection or capacity failure
 *  is. Callers with a queue ask this rather than reading Connect codes themselves (ARCH-17). */
export function retriableTransportFailure(error: unknown): boolean {
  const code = ConnectError.from(error).code
  return (
    code === Code.Unavailable ||
    code === Code.DeadlineExceeded ||
    code === Code.Unknown ||
    code === Code.Internal ||
    code === Code.ResourceExhausted ||
    code === Code.Aborted
  )
}
