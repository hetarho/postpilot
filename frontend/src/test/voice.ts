import { Code, createRouterTransport } from '@connectrpc/connect'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import {
  AddVoiceSampleResponseSchema,
  CreateVoiceResponseSchema,
  DeleteVoiceResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  GiveSentenceFeedbackResponseSchema,
  LearnFromFinalizedPostResponseSchema,
  ListVoicesResponseSchema,
  RenameVoiceResponseSchema,
  RestoreVoiceResponseSchema,
  RetryVoiceLearningResponseSchema,
  SetDefaultVoiceResponseSchema,
  UpdateVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceLearningEventSchema,
  VoiceLearningService,
  VoiceSampleSchema,
  VoiceSchema,
  VoiceService,
  VoiceValidationService,
  ListVoiceProfileVersionsResponseSchema,
  ListRuleConfirmationsResponseSchema,
  ListVoiceProfileValidationsResponseSchema,
  StructuredVoiceProfileSchema,
  UpdateVoiceOverrideResponseSchema,
  type ProtoVoiceProfile,
  contentLanguageFromProto,
  contentLanguageToProto,
  type ContentLanguage,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeVoiceSampleRow {
  id: string
  label: string
  chars?: number
  createdAt?: string
}

export interface FakeVoiceRow {
  id: string
  name: string
  isDefault?: boolean
  deleted?: boolean
  sourceLanguage?: ContentLanguage
}

/** The one voice every account starts with (the migration and adduser create it). The profile
 *  options below describe THIS voice's profile; every other voice starts empty, like the server. */
export const DEFAULT_FAKE_VOICE: FakeVoiceRow = {
  id: 'voice-default',
  name: '기본 말투',
  isDefault: true,
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
  validations?: Array<{
    id: string
    voiceId: string
    profileVersion: bigint
    status: string
    judgeEnabled?: boolean
    totalCount?: number
    yCount?: number
  }>
  /** The typed profile the account has learned. Omitted, the profile reads as empty. */
  structured?: MessageInitShape<typeof StructuredVoiceProfileSchema>
  overrideFails?: boolean
  /** The account's voice directory. Omitted, it holds only `DEFAULT_FAKE_VOICE`. */
  voices?: FakeVoiceRow[]
  /** Voice ids whose deletion the server refuses because work could still publish to them. */
  busyVoices?: string[]
  /** Make ListVoices fail. */
  listFails?: boolean
  /** Sentence feedback the fake received, for tests that assert *which* sentence was sent. */
  sentenceFeedback?: Array<{ sentenceRef: string; authoredText: string }>
  /** Voice creates, including the concrete language that must never be inferred server-side and
   *  the optional description with the analyze ref that must travel with it. */
  creates?: Array<{
    name: string
    sourceLanguage: ContentLanguage
    description: string
    analyzeModel: string
  }>
  /** The seeding job a described create answers with. */
  createJobId?: string
}

const NOW = '2026-08-29T12:00:00Z'
const NAME_MAX_CHARS = 50
const DESCRIPTION_MAX_CHARS = 500

interface VoiceRow {
  id: string
  name: string
  isDefault: boolean
  deletedAt: string
  sourceLanguage: ContentLanguage
}

export function registerVoiceService(router: ConnectRouter, options: FakeVoiceOptions = {}) {
  const { rpc } = router
  let sequence = options.samples?.length ?? 0
  let profileReads = 0

  const voices = new Map<string, VoiceRow>(
    (options.voices ?? [DEFAULT_FAKE_VOICE]).map((row) => [
      row.id,
      {
        id: row.id,
        name: row.name,
        isDefault: row.isDefault ?? false,
        deletedAt: row.deleted ? NOW : '',
        sourceLanguage: row.sourceLanguage ?? 'ko',
      },
    ]),
  )
  let voiceSequence = voices.size
  const defaultId =
    [...voices.values()].find((row) => row.isDefault && !row.deletedAt)?.id ?? DEFAULT_FAKE_VOICE.id

  const toProtoVoice = (row: VoiceRow) =>
    create(VoiceSchema, {
      id: row.id,
      name: row.name,
      isDefault: row.isDefault,
      deleted: row.deletedAt !== '',
      createdAt: NOW,
      updatedAt: NOW,
      deletedAt: row.deletedAt,
      sourceLanguage: contentLanguageToProto(row.sourceLanguage),
    })
  const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0)
  // The server's order: active before deleted, the default first, then by name.
  const directory = () =>
    [...voices.values()].sort(
      (a, b) =>
        Number(a.deletedAt !== '') - Number(b.deletedAt !== '') ||
        Number(b.isDefault) - Number(a.isDefault) ||
        compare(a.name, b.name) ||
        compare(a.id, b.id),
    )
  const owned = (voiceId: string): VoiceRow => {
    if (!voiceId) throw connectAppError('VOICE_REQUIRED', Code.InvalidArgument)
    const row = voices.get(voiceId)
    if (!row) throw connectAppError('VOICE_NOT_FOUND', Code.NotFound)
    return row
  }
  const active = (voiceId: string): VoiceRow => {
    const row = owned(voiceId)
    if (row.deletedAt) throw connectAppError('VOICE_DELETED', Code.FailedPrecondition)
    return row
  }
  const validName = (name: string): string => {
    const trimmed = name.trim()
    const chars = Array.from(trimmed).length
    if (chars === 0) throw connectAppError('VOICE_NAME_REQUIRED', Code.InvalidArgument)
    if (chars > NAME_MAX_CHARS)
      throw connectAppError('VOICE_NAME_TOO_LONG', Code.InvalidArgument, {
        actual: String(chars),
        max: String(NAME_MAX_CHARS),
      })
    return trimmed
  }
  const nameTaken = (name: string, except: string) =>
    [...voices.values()].some((row) => row.id !== except && !row.deletedAt && row.name === name)

  // Profiles are partitioned per voice: the options describe the default voice's, and any other
  // voice — created here or listed in `voices` — starts empty, as on the server.
  const profiles = new Map<string, ProtoVoiceProfile>()
  profiles.set(
    defaultId,
    create(VoiceProfileSchema, {
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
    }),
  )
  const profileOf = (voiceId: string): ProtoVoiceProfile => {
    owned(voiceId)
    let profile = profiles.get(voiceId)
    if (!profile) {
      profile = create(VoiceProfileSchema, {})
      profiles.set(voiceId, profile)
    }
    return profile
  }
  const setProfile = (voiceId: string, profile: ProtoVoiceProfile) => profiles.set(voiceId, profile)
  const withVoice = (voiceId: string, profile: ProtoVoiceProfile) =>
    create(VoiceProfileSchema, { ...profile, voice: toProtoVoice(owned(voiceId)) })

  rpc(VoiceService.method.listVoices, () => {
    options.calls?.push('ListVoices')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVoicesResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.createVoice, (request) => {
    options.calls?.push('CreateVoice')
    const name = validName(request.name)
    if (nameTaken(name, '')) throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
    const sourceLanguage = contentLanguageFromProto(request.sourceLanguage ?? 0)
    if (!sourceLanguage)
      throw connectAppError('VOICE_SOURCE_LANGUAGE_REQUIRED', Code.InvalidArgument)
    const description = request.description.trim()
    const describedChars = Array.from(description).length
    if (describedChars > DESCRIPTION_MAX_CHARS) {
      throw connectAppError('VOICE_DESCRIPTION_TOO_LONG', Code.InvalidArgument, {
        actual: String(describedChars),
        max: String(DESCRIPTION_MAX_CHARS),
      })
    }
    // Every reason to refuse is checked before the row exists, as on the server.
    const analyzeModel = request.analyzeModel
    if (description && (!analyzeModel?.providerId || !analyzeModel.modelId)) {
      throw connectAppError('VOICE_ANALYZE_MODEL_REQUIRED', Code.FailedPrecondition)
    }
    options.creates?.push({
      name,
      sourceLanguage,
      description,
      analyzeModel: description ? `${analyzeModel!.providerId}/${analyzeModel!.modelId}` : '',
    })
    voiceSequence += 1
    const row: VoiceRow = {
      id: `voice-${voiceSequence}`,
      name,
      isDefault: false,
      deletedAt: '',
      sourceLanguage,
    }
    voices.set(row.id, row)
    // Only a described create enqueues, and the new voice's profile carries that run so the
    // screen it lands on can show it.
    const jobId = description ? (options.createJobId ?? 'seed-job') : ''
    if (jobId) profiles.set(row.id, create(VoiceProfileSchema, { activeJobId: jobId }))
    return create(CreateVoiceResponseSchema, { voice: toProtoVoice(row), jobId })
  })

  rpc(VoiceService.method.renameVoice, (request) => {
    options.calls?.push('RenameVoice')
    const row = owned(request.voiceId)
    const name = validName(request.name)
    if (!row.deletedAt && nameTaken(name, row.id)) {
      throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
    }
    row.name = name
    return create(RenameVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.setDefaultVoice, (request) => {
    options.calls?.push('SetDefaultVoice')
    const row = active(request.voiceId)
    for (const other of voices.values()) other.isDefault = false
    row.isDefault = true
    return create(SetDefaultVoiceResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.deleteVoice, (request) => {
    options.calls?.push('DeleteVoice')
    const row = owned(request.voiceId)
    if (!row.deletedAt) {
      const activeCount = [...voices.values()].filter((other) => !other.deletedAt).length
      if (row.isDefault || activeCount <= 1) {
        throw connectAppError('VOICE_DEFAULT_DELETE_FORBIDDEN', Code.FailedPrecondition)
      }
      if (options.busyVoices?.includes(row.id)) {
        throw connectAppError('VOICE_BUSY', Code.FailedPrecondition)
      }
      row.deletedAt = NOW
    }
    return create(DeleteVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.restoreVoice, (request) => {
    options.calls?.push('RestoreVoice')
    const row = owned(request.voiceId)
    if (row.deletedAt) {
      if (nameTaken(row.name, row.id)) {
        throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
      }
      row.deletedAt = ''
    }
    return create(RestoreVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.getVoiceProfile, (request) => {
    options.calls?.push('GetVoiceProfile')
    let profile = profileOf(request.voiceId)
    if (request.voiceId === defaultId) {
      if (profileReads > 0 && options.styleguideAfterAnalysis) {
        profile = create(VoiceProfileSchema, {
          ...profile,
          styleguide: options.styleguideAfterAnalysis,
          activeJobId: '',
          updatedAt: NOW,
        })
        setProfile(defaultId, profile)
      }
      profileReads += 1
    }
    return create(GetVoiceProfileResponseSchema, { profile: withVoice(request.voiceId, profile) })
  })

  rpc(VoiceService.method.updateVoiceProfile, async (request) => {
    options.calls?.push('UpdateVoiceProfile')
    const profile = profileOf(request.voiceId)
    active(request.voiceId)
    if (options.updateGate) await options.updateGate
    if (options.updateFails)
      throw connectAppError('VOICE_INVALID_LIFECYCLE', Code.FailedPrecondition)
    const next = create(VoiceProfileSchema, {
      ...profile,
      styleguide: request.styleguide ?? profile.styleguide,
      rules: request.rules ?? profile.rules,
      updatedAt: NOW,
    })
    setProfile(request.voiceId, next)
    return create(UpdateVoiceProfileResponseSchema, { profile: withVoice(request.voiceId, next) })
  })

  rpc(VoiceService.method.updateVoiceOverride, (request) => {
    options.calls?.push('UpdateVoiceOverride')
    const profile = profileOf(request.voiceId)
    active(request.voiceId)
    if (options.overrideFails)
      throw connectAppError('VOICE_PROFILE_FIELD_REQUIRED', Code.InvalidArgument)
    // The response is the unchanged profile: what an override publishes is backend behavior with
    // its own coverage, and a fake that half-rebuilds a typed profile would only test itself.
    return create(UpdateVoiceOverrideResponseSchema, {
      profile: withVoice(request.voiceId, profile),
    })
  })

  rpc(VoiceService.method.addVoiceSample, async (request) => {
    options.calls?.push('AddVoiceSample')
    const profile = profileOf(request.voiceId)
    active(request.voiceId)
    if (options.addGate) await options.addGate
    if (options.addError)
      throw connectAppError('VOICE_SAMPLE_TOO_SHORT', Code.InvalidArgument, {
        actual: '199',
        min: '200',
      })
    const body = request.body.trim()
    const chars = Array.from(body).length
    if (chars < 200) {
      throw connectAppError('VOICE_SAMPLE_TOO_SHORT', Code.InvalidArgument, {
        actual: String(chars),
        min: '200',
      })
    }
    if (!request.model?.providerId || !request.model.modelId) {
      throw connectAppError('VOICE_ANALYZE_MODEL_REQUIRED', Code.FailedPrecondition)
    }
    sequence += 1
    const sample = create(VoiceSampleSchema, {
      id: `sample-${sequence}`,
      label: request.label.trim() || body.slice(0, 20),
      chars,
      createdAt: NOW,
    })
    const jobId = options.addJobId ?? 'voice-job'
    setProfile(
      request.voiceId,
      create(VoiceProfileSchema, {
        ...profile,
        samples: [sample, ...profile.samples],
        activeJobId: jobId,
      }),
    )
    return create(AddVoiceSampleResponseSchema, { sample, jobId })
  })

  rpc(VoiceService.method.deleteVoiceSample, (request) => {
    options.calls?.push('DeleteVoiceSample')
    const profile = profileOf(request.voiceId)
    active(request.voiceId)
    if (options.deleteFails) throw connectAppError('VOICE_SAMPLE_MUTATION_FAILED', Code.Internal)
    const samples = profile.samples.filter((sample) => sample.id !== request.sampleId)
    if (samples.length === profile.samples.length) {
      throw connectAppError('VOICE_SAMPLE_NOT_FOUND', Code.NotFound)
    }
    const jobId = samples.length > 0 ? (options.deleteJobId ?? 'voice-job') : ''
    setProfile(
      request.voiceId,
      create(VoiceProfileSchema, { ...profile, samples, activeJobId: jobId }),
    )
    return create(DeleteVoiceSampleResponseSchema, { jobId })
  })

  rpc(VoiceLearningService.method.learnFromFinalizedPost, (request) => {
    options.calls?.push('LearnFromFinalizedPost')
    if (options.learningFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const jobId = options.learningJobId ?? 'learning-job'
    return create(LearnFromFinalizedPostResponseSchema, {
      event: create(VoiceLearningEventSchema, {
        id: `event-${request.postSlug}`,
        postSlug: request.postSlug,
        voiceId: defaultId,
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

  rpc(VoiceLearningService.method.giveSentenceFeedback, (request) => {
    options.calls?.push('GiveSentenceFeedback')
    options.sentenceFeedback?.push({
      sentenceRef: request.sentenceRef,
      authoredText: request.authoredText,
    })
    return create(GiveSentenceFeedbackResponseSchema, { feedbackId: 'feedback-1' })
  })

  // The three per-tab lists. They record their calls so a test can prove a tab fetches only what
  // it renders — the profile screen used to issue all three on every mount.
  rpc(VoiceService.method.listVoiceProfileVersions, (request) => {
    options.calls?.push('ListVoiceProfileVersions')
    owned(request.voiceId)
    return create(ListVoiceProfileVersionsResponseSchema, { versions: options.versions ?? [] })
  })

  rpc(VoiceLearningService.method.listRuleConfirmations, (request) => {
    options.calls?.push('ListRuleConfirmations')
    owned(request.voiceId)
    return create(ListRuleConfirmationsResponseSchema, {})
  })

  rpc(VoiceValidationService.method.listVoiceProfileValidations, (request) => {
    options.calls?.push('ListVoiceProfileValidations')
    owned(request.voiceId)
    return create(ListVoiceProfileValidationsResponseSchema, {
      validations: options.validations ?? [],
    })
  })
}
