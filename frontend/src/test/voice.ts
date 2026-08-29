import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import {
  AddVoiceSampleResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  GiveSentenceFeedbackResponseSchema,
  LearnFromFinalizedPostResponseSchema,
  RetryVoiceLearningResponseSchema,
  UpdateVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceLearningEventSchema,
  VoiceLearningService,
  VoiceSampleSchema,
  VoiceService,
  VoiceValidationService,
  ListVoiceProfileVersionsResponseSchema,
  ListRuleConfirmationsResponseSchema,
  ListVoiceProfileValidationsResponseSchema,
  StructuredVoiceProfileSchema,
  UpdateVoiceOverrideResponseSchema,
} from '@/shared/api'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeVoiceSampleRow {
  id: string
  label: string
  chars?: number
  createdAt?: string
}

export interface FakeVoiceOptions {
  styleguide?: string
  rules?: string
  updatedAt?: string
  activeJobId?: string
  /** Returned from the second profile read, simulating a completed analysis. */
  styleguideAfterAnalysis?: string
  samples?: FakeVoiceSampleRow[]
  addJobId?: string
  deleteJobId?: string
  addError?: string
  updateFails?: boolean
  deleteFails?: boolean
  updateGate?: Promise<void>
  addGate?: Promise<void>
  calls?: string[]
  learningFails?: boolean
  learningJobId?: string
  versions?: Array<{ version: bigint; origin: string }>
  /** The typed profile the account has learned. Omitted, the profile reads as empty. */
  structured?: MessageInitShape<typeof StructuredVoiceProfileSchema>
  overrideFails?: boolean
}

const NOW = '2026-08-29T12:00:00Z'

export function registerVoiceService(router: ConnectRouter, options: FakeVoiceOptions = {}) {
  const { rpc } = router
  let sequence = options.samples?.length ?? 0
  let profileReads = 0
  let profile = create(VoiceProfileSchema, {
    styleguide: options.styleguide ?? '',
    rules: options.rules ?? '',
    updatedAt: options.updatedAt ?? '',
    activeJobId: options.activeJobId ?? '',
    samples: (options.samples ?? []).map((sample) =>
      create(VoiceSampleSchema, {
        ...sample,
        chars: sample.chars ?? 200,
        createdAt: sample.createdAt ?? NOW,
      }),
    ),
    structured: options.structured
      ? create(StructuredVoiceProfileSchema, options.structured)
      : undefined,
  })

  rpc(VoiceService.method.getVoiceProfile, () => {
    options.calls?.push('GetVoiceProfile')
    if (profileReads > 0 && options.styleguideAfterAnalysis) {
      profile = create(VoiceProfileSchema, {
        ...profile,
        styleguide: options.styleguideAfterAnalysis,
        activeJobId: '',
        updatedAt: NOW,
      })
    }
    profileReads += 1
    return create(GetVoiceProfileResponseSchema, { profile })
  })

  rpc(VoiceService.method.updateVoiceProfile, async (request) => {
    options.calls?.push('UpdateVoiceProfile')
    if (options.updateGate) await options.updateGate
    if (options.updateFails) throw new ConnectError('update voice profile failed', Code.Internal)
    profile = create(VoiceProfileSchema, {
      ...profile,
      styleguide: request.styleguide ?? profile.styleguide,
      rules: request.rules ?? profile.rules,
      updatedAt: NOW,
    })
    return create(UpdateVoiceProfileResponseSchema, { profile })
  })

  rpc(VoiceService.method.updateVoiceOverride, () => {
    options.calls?.push('UpdateVoiceOverride')
    if (options.overrideFails) throw new ConnectError('직접 설정을 저장하지 못했어요', Code.Internal)
    // The response is the unchanged profile: what an override publishes is backend behavior with
    // its own coverage, and a fake that half-rebuilds a typed profile would only test itself.
    return create(UpdateVoiceOverrideResponseSchema, { profile })
  })

  rpc(VoiceService.method.addVoiceSample, async (request) => {
    options.calls?.push('AddVoiceSample')
    if (options.addGate) await options.addGate
    if (options.addError) throw new ConnectError(options.addError, Code.InvalidArgument)
    const body = request.body.trim()
    const chars = Array.from(body).length
    if (chars < 200) {
      throw new ConnectError(
        `sample has ${chars} characters; at least 200 are required`,
        Code.InvalidArgument,
      )
    }
    if (!request.model?.providerId || !request.model.modelId) {
      throw new ConnectError('an enabled analyze model is required', Code.FailedPrecondition)
    }
    sequence += 1
    const sample = create(VoiceSampleSchema, {
      id: `sample-${sequence}`,
      label: request.label.trim() || body.slice(0, 20),
      chars,
      createdAt: NOW,
    })
    const jobId = options.addJobId ?? 'voice-job'
    profile = create(VoiceProfileSchema, {
      ...profile,
      samples: [sample, ...profile.samples],
      activeJobId: jobId,
    })
    return create(AddVoiceSampleResponseSchema, { sample, jobId })
  })

  rpc(VoiceService.method.deleteVoiceSample, (request) => {
    options.calls?.push('DeleteVoiceSample')
    if (options.deleteFails) throw new ConnectError('delete voice sample failed', Code.Internal)
    const samples = profile.samples.filter((sample) => sample.id !== request.sampleId)
    if (samples.length === profile.samples.length) {
      throw new ConnectError('voice sample not found', Code.NotFound)
    }
    const jobId = samples.length > 0 ? (options.deleteJobId ?? 'voice-job') : ''
    profile = create(VoiceProfileSchema, { ...profile, samples, activeJobId: jobId })
    return create(DeleteVoiceSampleResponseSchema, { jobId })
  })

  rpc(VoiceLearningService.method.learnFromFinalizedPost, (request) => {
    options.calls?.push('LearnFromFinalizedPost')
    if (options.learningFails) throw new ConnectError('learning unavailable', Code.Unavailable)
    const jobId = options.learningJobId ?? 'learning-job'
    return create(LearnFromFinalizedPostResponseSchema, {
      event: create(VoiceLearningEventSchema, {
        id: `event-${request.postSlug}`,
        postSlug: request.postSlug,
        baselineRevision: 1n,
        status: 'queued',
        jobId,
        createdAt: NOW,
      }),
      jobId,
    })
  })

  rpc(VoiceLearningService.method.retryVoiceLearning, (request) => {
    options.calls?.push('RetryVoiceLearning')
    const jobId = options.learningJobId ?? 'learning-job-retry'
    return create(RetryVoiceLearningResponseSchema, {
      event: create(VoiceLearningEventSchema, { id: request.eventId, status: 'queued', jobId }),
      jobId,
    })
  })

  rpc(VoiceLearningService.method.giveSentenceFeedback, () => {
    options.calls?.push('GiveSentenceFeedback')
    return create(GiveSentenceFeedbackResponseSchema, { feedbackId: 'feedback-1' })
  })

  // The three per-tab lists. They record their calls so a test can prove a tab fetches only what
  // it renders — the profile screen used to issue all three on every mount.
  rpc(VoiceService.method.listVoiceProfileVersions, () => {
    options.calls?.push('ListVoiceProfileVersions')
    return create(ListVoiceProfileVersionsResponseSchema, { versions: options.versions ?? [] })
  })

  rpc(VoiceLearningService.method.listRuleConfirmations, () => {
    options.calls?.push('ListRuleConfirmations')
    return create(ListRuleConfirmationsResponseSchema, {})
  })

  rpc(VoiceValidationService.method.listVoiceProfileValidations, () => {
    options.calls?.push('ListVoiceProfileValidations')
    return create(ListVoiceProfileValidationsResponseSchema, {})
  })
}
