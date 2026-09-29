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
  addJobId?: string
  deleteJobId?: string
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
      materialCount: row.materialCount,
      analyzedAt: row.made ? NOW : '',
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

  // The version list records its call so a test can prove the tab fetches only what it renders —
  // the profile screen used to issue every per-tab list on every mount.
  rpc(VoiceService.method.listVoiceProfileVersions, (request) => {
    options.calls?.push('ListVoiceProfileVersions')
    owned(request.voiceId)
    return create(ListVoiceProfileVersionsResponseSchema, { versions: options.versions ?? [] })
  })
}
