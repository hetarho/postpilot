import { Code, createRouterTransport } from '@connectrpc/connect'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import {
  AddVoiceSampleResponseSchema,
  CreateVoiceResponseSchema,
  DeleteVoiceResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  ListVoicesResponseSchema,
  RenameVoiceResponseSchema,
  RestoreVoiceResponseSchema,
  SetDefaultVoiceResponseSchema,
  GetVoiceProfileVersionSampleResponseSchema,
  RestoreVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceSampleSchema,
  VoiceSchema,
  VoiceService,
  ListVoiceProfileVersionsResponseSchema,
  PostContentSchema,
  VoiceValueSource,
  StructuredVoiceProfileSchema,
  UpdateVoiceOverrideResponseSchema,
  type ProtoVoiceProfile,
  type ProtoVoiceSample,
  VoicePromptPart,
  VoiceSampleKind,
  VoiceReadinessSchema,
  VoicePromptSchema,
  ListVoicePromptsResponseSchema,
  GetVoiceSampleResponseSchema,
  CreateVoicePhotoUploadResponseSchema,
  AnswerVoicePromptResponseSchema,
  AnalyzeVoiceResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeVoiceSampleRow {
  id: string
  label: string
  chars?: number
  createdAt?: string
  kind?: 'post' | 'answer'
  promptKey?: string
  /** The full text; omitted, a 200-character body. */
  body?: string
  hasPhoto?: boolean
}

/** The shared prompt set as the fake serves it: a subset with every part and a photo prompt,
 *  which is all a screen test needs to tell groups and photos apart. */
export const FAKE_VOICE_PROMPTS = [
  {
    key: 'opening_greeting',
    part: 'opening',
    photo: false,
    text: '블로그 글을 시작할 때 쓰는 첫인사를 평소처럼 2~5문장으로 써 보세요.',
  },
  {
    key: 'photo_food',
    part: 'description',
    photo: true,
    text: '음식이나 음료 사진 한 장을 골라, 블로그에 쓰듯 2~5문장으로 써 보세요.',
  },
  {
    key: 'situation_value',
    part: 'description',
    photo: false,
    text: '가격이나 양, 가성비에 대한 생각을 2~5문장으로 써 보세요.',
  },
  {
    key: 'closing_greeting',
    part: 'closing',
    photo: false,
    text: '글을 마무리할 때 쓰는 끝인사를 평소처럼 2~5문장으로 써 보세요.',
  },
] as const

export interface FakeVoiceRow {
  id: string
  name: string
  isDefault?: boolean
  deleted?: boolean
  /** Omitted is made; a voice this fake creates starts not made, as on the server (VOICE-10). */
  made?: boolean
  /** How many 학습 글 the directory row says the voice holds. */
  materialCount?: number
}

/** The voice a fixture account holds unless a test lists its own directory: no account is given
 *  one (VOICE-4), so this stands for the one its owner made. The profile options below describe
 *  THIS voice's profile; every other voice starts empty, like the server. */
export const DEFAULT_FAKE_VOICE: FakeVoiceRow = {
  id: 'voice-default',
  name: '기본 말투',
  isDefault: true,
}

export interface FakeVoiceOptions {
  updatedAt?: string
  activeJobId?: string
  /** Returned from the second profile read, simulating a completed analysis. */
  /** The analysis the profile publishes on the read AFTER the first one — the shape a resumed
   *  analysis has when its job is already done. It lands in the structured profile's lexical
   *  description, which is where an analysis lives now (VOICE-25). */
  analysisAfterAnalysis?: string
  samples?: FakeVoiceSampleRow[]
  addError?: string
  deleteFails?: boolean
  addGate?: Promise<void>
  calls?: string[]
  versions?: Array<{ version: bigint; origin: string; hasSample?: boolean }>
  /** One version's generation snapshot, keyed by version number as a string. A version absent
   *  from this map produced no post, which is what the RPC reports with an unset sample. */
  versionSamples?: Record<string, MessageInitShape<typeof PostContentSchema>>
  /** The typed profile the account has learned. Omitted, the profile reads as empty. */
  structured?: MessageInitShape<typeof StructuredVoiceProfileSchema>
  overrideFails?: boolean
  /** The account's voice directory. Omitted, it holds only `DEFAULT_FAKE_VOICE`. */
  voices?: FakeVoiceRow[]
  /** Voice ids whose deletion the server refuses because work could still publish to them. */
  busyVoices?: string[]
  /** Make ListVoices fail. */
  listFails?: boolean
  /** Voice creates: a voice is created by name alone (VOICE-10). */
  creates?: Array<{ name: string }>
  /** 말투 만들기 / 다시 분석 starts, with the model each named. */
  analyses?: Array<{ voiceId: string; model: string }>
  /** Answers the fake received, with the upload each named. */
  answers?: Array<{ promptKey: string; body: string; uploadId: string }>
  /** The job an AnalyzeVoice answers with. */
  analyzeJobId?: string
}

const PART_TO_PROTO = {
  opening: VoicePromptPart.OPENING,
  description: VoicePromptPart.DESCRIPTION,
  closing: VoicePromptPart.CLOSING,
} as const

/** The server's readiness, counted the simple way the fixtures need: sentences end at `.`, `!`,
 *  `?` or a line break, a post covers every part and an answer its prompt's (VOICE-32). */
function fakeReadiness(rows: readonly MaterialRow[]) {
  let sentences = 0
  const covered = new Set<string>()
  for (const row of rows) {
    sentences += row.body.split(/[.!?\n]+/).filter((part) => part.trim() !== '').length
    if (row.sample.kind === VoiceSampleKind.ANSWER) {
      const prompt = FAKE_VOICE_PROMPTS.find((candidate) => candidate.key === row.sample.promptKey)
      if (prompt) covered.add(prompt.part)
    } else {
      for (const part of ['opening', 'description', 'closing']) covered.add(part)
    }
  }
  const missing = (['opening', 'description', 'closing'] as const).filter(
    (part) => !covered.has(part),
  )
  let percent = Math.floor((Math.min(sentences, 60) * 100) / 60)
  if (missing.length > 0 && percent > 99) percent = 99
  return create(VoiceReadinessSchema, {
    percent,
    sentences,
    needed: 60,
    missingParts: missing.map((part) => PART_TO_PROTO[part]),
  })
}

interface MaterialRow {
  sample: ProtoVoiceSample
  body: string
}

const NOW = '2026-08-29T12:00:00Z'
const NAME_MAX_CHARS = 50

interface VoiceRow {
  id: string
  name: string
  isDefault: boolean
  deletedAt: string
  made: boolean
  materialCount: number
}

const toSampleKind = (kind: FakeVoiceSampleRow['kind']) =>
  kind === 'answer' ? VoiceSampleKind.ANSWER : VoiceSampleKind.POST

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
        made: row.made ?? true,
        materialCount: row.materialCount ?? 0,
      },
    ]),
  )
  let voiceSequence = voices.size
  const defaultId =
    [...voices.values()].find((row) => row.isDefault && !row.deletedAt)?.id ?? DEFAULT_FAKE_VOICE.id

  // Every voice's 학습 글, newest first. The options' samples are the default voice's.
  const materials = new Map<string, MaterialRow[]>()
  const materialsOf = (voiceId: string) => materials.get(voiceId) ?? []
  const toProtoVoice = (row: VoiceRow) =>
    create(VoiceSchema, {
      id: row.id,
      name: row.name,
      isDefault: row.isDefault,
      deleted: row.deletedAt !== '',
      createdAt: NOW,
      updatedAt: NOW,
      deletedAt: row.deletedAt,
      made: row.made,
      materialCount: materials.has(row.id) ? materialsOf(row.id).length : row.materialCount,
      analyzedAt: row.made ? NOW : '',
      readinessPercent: row.made ? 0 : fakeReadiness(materialsOf(row.id)).percent,
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
  if (options.samples) {
    materials.set(
      defaultId,
      options.samples.map((row) => ({
        sample: create(VoiceSampleSchema, {
          id: row.id,
          label: row.label,
          chars: row.chars ?? 200,
          createdAt: row.createdAt ?? NOW,
          kind: toSampleKind(row.kind),
          promptKey: row.promptKey ?? '',
          hasPhoto: row.hasPhoto ?? false,
        }),
        body: row.body ?? '가'.repeat(row.chars ?? 200),
      })),
    )
  }
  profiles.set(
    defaultId,
    create(VoiceProfileSchema, {
      updatedAt: options.updatedAt ?? '',
      activeJobId: options.activeJobId ?? '',
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
  const withVoice = (voiceId: string, profile: ProtoVoiceProfile) => {
    const row = owned(voiceId)
    return create(VoiceProfileSchema, {
      ...profile,
      voice: toProtoVoice(row),
      made: row.made,
      samples: materialsOf(voiceId).map((material) => material.sample),
      readiness: fakeReadiness(materialsOf(voiceId)),
    })
  }

  rpc(VoiceService.method.listVoices, () => {
    options.calls?.push('ListVoices')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVoicesResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.createVoice, (request) => {
    options.calls?.push('CreateVoice')
    const name = validName(request.name)
    if (nameTaken(name, '')) throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
    options.creates?.push({ name })
    voiceSequence += 1
    const row: VoiceRow = {
      id: `voice-${voiceSequence}`,
      name,
      isDefault: false,
      deletedAt: '',
      made: false,
      materialCount: 0,
    }
    voices.set(row.id, row)
    return create(CreateVoiceResponseSchema, { voice: toProtoVoice(row) })
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
    // An empty id clears the 기본 (VOICE-12); a voice not yet made cannot be it.
    if (!request.voiceId) {
      for (const other of voices.values()) other.isDefault = false
      return create(SetDefaultVoiceResponseSchema, { voices: directory().map(toProtoVoice) })
    }
    const row = active(request.voiceId)
    if (!row.made) throw connectAppError('VOICE_NOT_MADE', Code.FailedPrecondition)
    for (const other of voices.values()) other.isDefault = false
    row.isDefault = true
    return create(SetDefaultVoiceResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.deleteVoice, (request) => {
    options.calls?.push('DeleteVoice')
    const row = owned(request.voiceId)
    // The 기본 and the last voice delete like any other (VOICE-13).
    if (!row.deletedAt) {
      if (options.busyVoices?.includes(row.id)) {
        throw connectAppError('VOICE_BUSY', Code.FailedPrecondition)
      }
      row.deletedAt = NOW
      row.isDefault = false
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
      if (profileReads > 0 && options.analysisAfterAnalysis) {
        profile = create(VoiceProfileSchema, {
          ...profile,
          structured: create(StructuredVoiceProfileSchema, {
            meta: { version: 1n },
            empty: false,
            lexical: {
              description: {
                value: options.analysisAfterAnalysis,
                source: VoiceValueSource.ANALYZED,
              },
            },
          }),
          activeJobId: '',
          updatedAt: NOW,
        })
        setProfile(defaultId, profile)
      }
      profileReads += 1
    }
    return create(GetVoiceProfileResponseSchema, { profile: withVoice(request.voiceId, profile) })
  })

  rpc(VoiceService.method.restoreVoiceProfile, (request) => {
    options.calls?.push('RestoreVoiceProfile')
    owned(request.voiceId)
    // Adopting a version publishes a NEW head and destroys nothing, so the fake answers with
    // the profile it already holds rather than pretending to rewrite history.
    return create(RestoreVoiceProfileResponseSchema, {
      profile: withVoice(request.voiceId, profileOf(request.voiceId)),
    })
  })

  rpc(VoiceService.method.getVoiceProfileVersionSample, (request) => {
    options.calls?.push('GetVoiceProfileVersionSample')
    const sample = options.versionSamples?.[request.version.toString()]
    return create(GetVoiceProfileVersionSampleResponseSchema, {
      sample: sample ? create(PostContentSchema, sample) : undefined,
      createdAt: sample ? NOW : '',
    })
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

  const addMaterial = (voiceId: string, material: MaterialRow) =>
    materials.set(voiceId, [material, ...materialsOf(voiceId)])

  rpc(VoiceService.method.addVoiceSample, async (request) => {
    options.calls?.push('AddVoiceSample')
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
    sequence += 1
    const sample = create(VoiceSampleSchema, {
      id: `sample-${sequence}`,
      kind: VoiceSampleKind.POST,
      label: request.label.trim() || body.slice(0, 20),
      chars,
      createdAt: NOW,
    })
    addMaterial(request.voiceId, { sample, body })
    return create(AddVoiceSampleResponseSchema, { sample })
  })

  rpc(VoiceService.method.deleteVoiceSample, (request) => {
    options.calls?.push('DeleteVoiceSample')
    active(request.voiceId)
    if (options.deleteFails) throw connectAppError('UNKNOWN_FAILURE', Code.Internal)
    const rows = materialsOf(request.voiceId)
    const kept = rows.filter((row) => row.sample.id !== request.sampleId)
    if (kept.length === rows.length) {
      throw connectAppError('VOICE_SAMPLE_NOT_FOUND', Code.NotFound)
    }
    materials.set(request.voiceId, kept)
    return create(DeleteVoiceSampleResponseSchema, {})
  })

  rpc(VoiceService.method.getVoiceSample, (request) => {
    options.calls?.push('GetVoiceSample')
    owned(request.voiceId)
    const row = materialsOf(request.voiceId).find(
      (material) => material.sample.id === request.sampleId,
    )
    if (!row) throw connectAppError('VOICE_SAMPLE_NOT_FOUND', Code.NotFound)
    return create(GetVoiceSampleResponseSchema, {
      sample: row.sample,
      body: row.body,
      photoUrl: row.sample.hasPhoto ? `https://storage.test/voices/${request.sampleId}.jpg` : '',
      photoWidth: row.sample.hasPhoto ? 1024 : 0,
      photoHeight: row.sample.hasPhoto ? 768 : 0,
    })
  })

  rpc(VoiceService.method.listVoicePrompts, () => {
    options.calls?.push('ListVoicePrompts')
    return create(ListVoicePromptsResponseSchema, {
      prompts: FAKE_VOICE_PROMPTS.map((prompt) =>
        create(VoicePromptSchema, { ...prompt, part: PART_TO_PROTO[prompt.part] }),
      ),
    })
  })

  let uploadSequence = 0
  const pendingUploads = new Map<string, string>()
  rpc(VoiceService.method.createVoicePhotoUpload, (request) => {
    options.calls?.push('CreateVoicePhotoUpload')
    active(request.voiceId)
    const prompt = FAKE_VOICE_PROMPTS.find((candidate) => candidate.key === request.promptKey)
    if (!prompt?.photo) throw connectAppError('VOICE_PROMPT_NOT_FOUND', Code.NotFound)
    uploadSequence += 1
    const uploadId = `voice-upload-${uploadSequence}`
    pendingUploads.set(uploadId, request.promptKey)
    return create(CreateVoicePhotoUploadResponseSchema, {
      uploadId,
      putUrl: `https://storage.test/put/${uploadId}`,
      contentType: 'image/jpeg',
      expiresAt: NOW,
    })
  })

  rpc(VoiceService.method.answerVoicePrompt, (request) => {
    options.calls?.push('AnswerVoicePrompt')
    active(request.voiceId)
    const prompt = FAKE_VOICE_PROMPTS.find((candidate) => candidate.key === request.promptKey)
    if (!prompt) throw connectAppError('VOICE_PROMPT_NOT_FOUND', Code.NotFound)
    const body = request.body.trim()
    if (!body) throw connectAppError('VOICE_ANSWER_REQUIRED', Code.InvalidArgument)
    if (materialsOf(request.voiceId).some((row) => row.sample.promptKey === prompt.key)) {
      throw connectAppError('VOICE_PROMPT_ANSWERED', Code.AlreadyExists)
    }
    if (prompt.photo && pendingUploads.get(request.uploadId) !== prompt.key) {
      throw connectAppError('VOICE_PHOTO_REQUIRED', Code.FailedPrecondition)
    }
    options.answers?.push({ promptKey: prompt.key, body, uploadId: request.uploadId })
    pendingUploads.delete(request.uploadId)
    sequence += 1
    const sample = create(VoiceSampleSchema, {
      id: `sample-${sequence}`,
      kind: VoiceSampleKind.ANSWER,
      promptKey: prompt.key,
      hasPhoto: prompt.photo,
      chars: Array.from(body).length,
      createdAt: NOW,
    })
    addMaterial(request.voiceId, { sample, body })
    return create(AnswerVoicePromptResponseSchema, { sample })
  })

  rpc(VoiceService.method.analyzeVoice, (request) => {
    options.calls?.push('AnalyzeVoice')
    active(request.voiceId)
    if (!request.model?.providerId || !request.model.modelId) {
      throw connectAppError('VOICE_ANALYZE_MODEL_REQUIRED', Code.FailedPrecondition)
    }
    if (fakeReadiness(materialsOf(request.voiceId)).percent < 100) {
      throw connectAppError('VOICE_NOT_READY', Code.FailedPrecondition)
    }
    options.analyses?.push({
      voiceId: request.voiceId,
      model: `${request.model.providerId}/${request.model.modelId}`,
    })
    const jobId = options.analyzeJobId ?? 'voice-job'
    setProfile(
      request.voiceId,
      create(VoiceProfileSchema, { ...profileOf(request.voiceId), activeJobId: jobId }),
    )
    return create(AnalyzeVoiceResponseSchema, { jobId })
  })

  // The version list records its call so a test can prove the tab fetches only what it renders —
  // the profile screen used to issue every per-tab list on every mount.
  rpc(VoiceService.method.listVoiceProfileVersions, (request) => {
    options.calls?.push('ListVoiceProfileVersions')
    owned(request.voiceId)
    return create(ListVoiceProfileVersionsResponseSchema, { versions: options.versions ?? [] })
  })
}
