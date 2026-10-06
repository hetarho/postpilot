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
  VoiceProfileSchema,
  VoiceSampleSchema,
  VoiceSchema,
  VoiceService,
  VoiceAnalysisSchema,
  VoiceNoticeKind,
  VoiceNoticeSchema,
  RestorePreviousVoiceAnalysisResponseSchema,
  GetPostFingerprintResponseSchema,
  VoiceCheckSchema,
  ProtoVoiceCheckStatus,
  ListVoiceChecksResponseSchema,
  StartVoiceCheckResponseSchema,
  RetryVoiceCheckResponseSchema,
  type AppFailureReason,
  type ProtoVoiceCheck,
  type ProtoVoiceProfile,
  type ProtoVoiceSample,
  type ProtoVoiceAnalysis,
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
  createFails?: boolean
  createGate?: Promise<void>

  activeJobId?: string
  /** The impression of an analysis the profile publishes on the read AFTER the first one — the
   *  shape a resumed analysis has when its job is already done. */
  analysisAfterAnalysis?: string
  /** The default voice's current analysis. Omitted, a made voice reads with an empty one. */
  analysis?: MessageInitShape<typeof VoiceAnalysisSchema>
  /** The default voice's previous analysis, which 이전 분석으로 되돌리기 returns to. */
  previousAnalysis?: MessageInitShape<typeof VoiceAnalysisSchema>
  /** The default voice's notice (VOICE-21). */
  notice?: { kind: 'added' | 'changed'; count?: number }
  samples?: FakeVoiceSampleRow[]
  addError?: string
  deleteFails?: boolean
  addGate?: Promise<void>
  calls?: string[]
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
  /** ②'s comparison per post slug (POST-102); a slug not listed answers not applicable. */
  postFingerprints?: Record<string, MessageInitShape<typeof GetPostFingerprintResponseSchema>>
  /** The slug of every GetPostFingerprint the fake received. */
  postFingerprintReads?: string[]
  /** The default voice's 검증 results, newest first (VOICE-43). */
  checks?: MessageInitShape<typeof VoiceCheckSchema>[]
  /** The default voice's queued or running 검증 job, as ListVoiceChecks names it. */
  activeCheckJobId?: string
  /** 검증 starts and retries the fake received, with the model each named. */
  checkStarts?: Array<{ voiceId: string; promptKey: string; model: string }>
  checkRetries?: Array<{ checkId: string; model: string }>
  /** Refuse a 검증 start with this reason — a shared entitlement refusal included. */
  checkStartRefusal?: { reason: AppFailureReason; code: Code; params?: Record<string, string> }
  /** The job a 검증 start or retry answers with. */
  checkJobId?: string
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
      activeJobId: options.activeJobId ?? '',
    }),
  )
  // Each voice's current and previous analysis (VOICE-30). A made voice with no analysis given
  // reads with an empty one, so it is shown as made.
  const current = new Map<string, ReturnType<typeof create<typeof VoiceAnalysisSchema>>>()
  const previous = new Map<string, ReturnType<typeof create<typeof VoiceAnalysisSchema>>>()
  if (options.analysis) current.set(defaultId, create(VoiceAnalysisSchema, options.analysis))
  if (options.previousAnalysis)
    previous.set(defaultId, create(VoiceAnalysisSchema, options.previousAnalysis))
  // Like the server, an example whose 학습 글 was deleted is not returned (VOICE-21).
  const withoutDeletedExamples = (voiceId: string, analysis: ProtoVoiceAnalysis) => {
    const present = new Set(materialsOf(voiceId).map((row) => row.sample.id))
    const copy = create(VoiceAnalysisSchema, analysis)
    const counted = copy.counted
    if (counted) {
      for (const item of [
        counted.endings,
        counted.marks,
        counted.emoji,
        counted.shape,
        counted.openings,
        counted.adverbs,
        counted.person,
        counted.headings,
      ]) {
        if (item?.example && !present.has(item.example.materialId)) item.example = undefined
      }
    }
    if (copy.ai)
      copy.ai.examples = copy.ai.examples.filter((example) => present.has(example.materialId))
    return copy
  }
  const analysisOf = (row: VoiceRow) => {
    const found = current.get(row.id)
    if (found) return withoutDeletedExamples(row.id, found)
    return row.made ? create(VoiceAnalysisSchema, { counted: {}, ai: {} }) : undefined
  }
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
    const notice =
      voiceId === defaultId && options.notice
        ? create(VoiceNoticeSchema, {
            kind: options.notice.kind === 'added' ? VoiceNoticeKind.ADDED : VoiceNoticeKind.CHANGED,
            count: options.notice.count ?? 0,
          })
        : undefined
    return create(VoiceProfileSchema, {
      ...profile,
      voice: toProtoVoice(row),
      made: row.made,
      samples: materialsOf(voiceId).map((material) => material.sample),
      readiness: fakeReadiness(materialsOf(voiceId)),
      analysis: analysisOf(row),
      hasPrevious: previous.has(voiceId),
      notice,
    })
  }

  rpc(VoiceService.method.listVoices, () => {
    options.calls?.push('ListVoices')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVoicesResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.createVoice, async (request) => {
    options.calls?.push('CreateVoice')
    if (options.createGate) await options.createGate
    if (options.createFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
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
        profile = create(VoiceProfileSchema, { ...profile, activeJobId: '' })
        setProfile(defaultId, profile)
        current.set(
          defaultId,
          create(VoiceAnalysisSchema, {
            counted: {},
            ai: { impression: options.analysisAfterAnalysis },
          }),
        )
        owned(defaultId).made = true
      }
      profileReads += 1
    }
    return create(GetVoiceProfileResponseSchema, { profile: withVoice(request.voiceId, profile) })
  })

  // 검증 (VOICE-43): the default voice's results, a start or retry adding a queued check.
  const checks = new Map<string, ProtoVoiceCheck[]>()
  checks.set(
    defaultId,
    (options.checks ?? []).map((check) => create(VoiceCheckSchema, check)),
  )
  let checkSequence = 0
  const queuedCheck = (voiceId: string, promptKey: string) => {
    const prompt = FAKE_VOICE_PROMPTS.find((candidate) => candidate.key === promptKey)
    if (!prompt) throw connectAppError('VOICE_PROMPT_NOT_FOUND', Code.NotFound)
    const answer = materialsOf(voiceId).find((row) => row.sample.promptKey === promptKey)
    if (!answer) throw connectAppError('VOICE_CHECK_PROMPT_UNANSWERED', Code.FailedPrecondition)
    checkSequence += 1
    const check = create(VoiceCheckSchema, {
      id: `check-new-${checkSequence}`,
      prompt: {
        key: prompt.key,
        part: PART_TO_PROTO[prompt.part],
        photo: prompt.photo,
        text: prompt.text,
      },
      answer: answer.body,
      status: ProtoVoiceCheckStatus.QUEUED,
      createdAt: NOW,
    })
    checks.set(voiceId, [check, ...(checks.get(voiceId) ?? [])])
    return check
  }
  rpc(VoiceService.method.listVoiceChecks, (request) => {
    options.calls?.push('ListVoiceChecks')
    owned(request.voiceId)
    return create(ListVoiceChecksResponseSchema, {
      checks: checks.get(request.voiceId) ?? [],
      activeJobId: request.voiceId === defaultId ? (options.activeCheckJobId ?? '') : '',
    })
  })
  rpc(VoiceService.method.startVoiceCheck, (request) => {
    options.calls?.push('StartVoiceCheck')
    const model = `${request.model?.providerId ?? ''}/${request.model?.modelId ?? ''}`
    options.checkStarts?.push({ voiceId: request.voiceId, promptKey: request.promptKey, model })
    active(request.voiceId)
    if (options.checkStartRefusal) {
      const { reason, code, params } = options.checkStartRefusal
      throw connectAppError(reason, code, params)
    }
    const check = queuedCheck(request.voiceId, request.promptKey)
    return create(StartVoiceCheckResponseSchema, {
      check,
      jobId: options.checkJobId ?? 'check-job',
    })
  })
  rpc(VoiceService.method.retryVoiceCheck, (request) => {
    options.calls?.push('RetryVoiceCheck')
    const model = `${request.model?.providerId ?? ''}/${request.model?.modelId ?? ''}`
    options.checkRetries?.push({ checkId: request.checkId, model })
    for (const [voiceId, rows] of checks) {
      const found = rows.find((row) => row.id === request.checkId)
      if (found?.prompt) {
        const check = queuedCheck(voiceId, found.prompt.key)
        return create(RetryVoiceCheckResponseSchema, {
          check,
          jobId: options.checkJobId ?? 'check-job',
        })
      }
    }
    throw connectAppError('VOICE_CHECK_NOT_FOUND', Code.NotFound)
  })

  rpc(VoiceService.method.getPostFingerprint, (request) => {
    options.calls?.push('GetPostFingerprint')
    options.postFingerprintReads?.push(request.postSlug)
    return create(
      GetPostFingerprintResponseSchema,
      options.postFingerprints?.[request.postSlug] ?? { applicable: false },
    )
  })

  rpc(VoiceService.method.restorePreviousVoiceAnalysis, (request) => {
    options.calls?.push('RestorePreviousVoiceAnalysis')
    active(request.voiceId)
    const back = previous.get(request.voiceId)
    if (!back) throw connectAppError('VOICE_NO_PREVIOUS_ANALYSIS', Code.FailedPrecondition)
    current.set(request.voiceId, back)
    previous.delete(request.voiceId)
    return create(RestorePreviousVoiceAnalysisResponseSchema, {
      profile: withVoice(request.voiceId, profileOf(request.voiceId)),
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
    // A second answer rewrites the first, on its photo unless another is uploaded (VOICE-60).
    const rows = materialsOf(request.voiceId)
    const previous = rows.find((row) => row.sample.promptKey === prompt.key)
    const keepsPhoto = !request.uploadId && previous?.sample.hasPhoto === true
    if (prompt.photo && !keepsPhoto && pendingUploads.get(request.uploadId) !== prompt.key) {
      throw connectAppError('VOICE_PHOTO_REQUIRED', Code.FailedPrecondition)
    }
    if (previous)
      materials.set(
        request.voiceId,
        rows.filter((row) => row !== previous),
      )
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
}
