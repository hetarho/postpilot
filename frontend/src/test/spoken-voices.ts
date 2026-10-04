import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import {
  AuthService,
  GetMeResponseSchema,
  ProtoPlan,
  SpeechProfileService,
  SpokenVoiceService,
  SpokenVoiceGenerationService,
  SpokenDraftSchema,
  SpokenCandidateSchema,
  SpokenVoiceSchema,
  SpokenOperationSchema,
  type ProtoSpokenDraft,
  type ProtoSpokenVoice,
  type ProtoSpokenOperation,
} from '@/shared/api'
export const SPOKEN_TEST_DRAFT = 'd'.repeat(32),
  SPOKEN_TEST_OPERATION = 'e'.repeat(32),
  SPOKEN_TEST_VOICE = 'f'.repeat(32),
  SPOKEN_TEST_TICKET = 'a'.repeat(32)
export function spokenVoiceFixture(
  options: {
    preset?: 'candidates' | 'confirmed'
    operationState?: string
    unavailable?: boolean
    plan?: ProtoPlan
  } = {},
) {
  const startKeys: string[] = []
  const calls: string[] = [],
    drafts = new Map<string, ProtoSpokenDraft>(),
    voices = new Map<string, ProtoSpokenVoice>(),
    operations = new Map<string, ProtoSpokenOperation>()
  let revision = 1n,
    ticketRequested = false,
    pending = !!options.operationState,
    failStart = false
  const profile = {
    id: 'curated',
    revision: 1n,
    providerId: 'speech',
    designModelId: 'design',
    speechModelId: 'tts',
    designLabel: 'Korean design',
    speechLabel: 'Korean synthesis',
    grade: 'balanced',
    descriptionMax: 1000,
    previewMax: 1000,
    speechMax: 1000,
    outputFormat: 'mp3_44100_128',
  }
  const candidates = () =>
    [0, 1, 2].map((i) =>
      create(SpokenCandidateSchema, {
        id: String(i + 1).repeat(32),
        assetId: String(i + 4).repeat(32),
        durationMs: 1500n,
        auditionedAt: '',
      }),
    )
  if (options.preset) {
    const draft = create(SpokenDraftSchema, {
      id: SPOKEN_TEST_DRAFT,
      revision: revision++,
      name: '나의 목소리',
      description: '차분하고 따뜻하게 한국어를 읽는 성인 목소리입니다.',
      previewText: '가'.repeat(150),
      profile,
      phase: options.preset === 'confirmed' ? 'confirmed' : 'candidates',
      candidates: candidates(),
      selectedCandidateId: '',
      confirmedVoiceId: options.preset === 'confirmed' ? SPOKEN_TEST_VOICE : '',
    })
    drafts.set(draft.id, draft)
  }
  if (options.preset === 'confirmed')
    voices.set(
      SPOKEN_TEST_VOICE,
      create(SpokenVoiceSchema, {
        id: SPOKEN_TEST_VOICE,
        revision: 1n,
        name: '나의 목소리',
        description: '차분하고 따뜻하게 한국어를 읽는 성인 목소리입니다.',
        previewText: '가'.repeat(150),
        profile,
        sampleAssetId: '4'.repeat(32),
        sampleDurationMs: 1500n,
      }),
    )
  if (options.operationState)
    operations.set(
      SPOKEN_TEST_OPERATION,
      create(SpokenOperationSchema, {
        id: SPOKEN_TEST_OPERATION,
        kind: 'voice_confirm',
        state: options.operationState,
        draftId: SPOKEN_TEST_DRAFT,
        jobId: 'job',
      }),
    )
  function draft(id: string) {
    const d = drafts.get(id)
    if (!d) throw new ConnectError('not found', Code.NotFound)
    return d
  }
  function check(id: string, expected: bigint) {
    const d = draft(id)
    if (d.revision !== expected) throw new ConnectError('revision changed', Code.Aborted)
    return d
  }
  function quote() {
    return {
      quoteId: 'approved-quote',
      maximumCredits: 2,
      approvalRequired: true,
      expiresAt: new Date(Date.now() + 600000).toISOString(),
    }
  }
  function progress(o: ProtoSpokenOperation) {
    if (!pending && o.state === 'queued') {
      const d = draft(o.draftId)
      d.revision = revision++
      if (o.kind === 'voice_design') {
        d.candidates = candidates()
        d.phase = 'candidates'
      } else {
        const c = d.candidates.find((c) => c.id === o.candidateId)
        if (!c?.auditionedAt || d.selectedCandidateId !== c.id)
          throw new ConnectError('audition required', Code.FailedPrecondition)
        d.confirmedVoiceId = SPOKEN_TEST_VOICE
        d.phase = 'confirmed'
        voices.set(
          SPOKEN_TEST_VOICE,
          create(SpokenVoiceSchema, {
            id: SPOKEN_TEST_VOICE,
            revision: 1n,
            name: d.name,
            description: d.description,
            previewText: d.previewText,
            profile: d.profile,
            sampleAssetId: c.assetId,
            sampleDurationMs: c.durationMs,
          }),
        )
        o.resultId = SPOKEN_TEST_VOICE
      }
      o.state = 'published'
    }
    return o
  }
  const transport = createRouterTransport(({ service }) => {
    service(AuthService, {
      getMe: () =>
        create(GetMeResponseSchema, {
          user: { id: 'alice', emailVerified: true, hasPassword: true },
          plan: options.plan ?? ProtoPlan.MASTER,
        }),
    })
    service(SpeechProfileService, {
      listSpeechProfiles: () => ({
        profiles: [
          {
            id: 'curated',
            revision: 1n,
            label: '한국어 목소리',
            designModel: { providerId: 'speech', modelId: 'design' },
            speechModel: { providerId: 'speech', modelId: 'tts' },
            designLabel: 'Korean design',
            speechLabel: 'Korean synthesis',
            grade: 'balanced',
            available: !options.unavailable,
            entitled: true,
            requiredPlan: 'free',
            unavailableReason: options.unavailable ? 'SPEECH_BINDING_INCOMPATIBLE' : '',
            descriptionMax: 1000,
            previewMin: 100,
            previewMax: 1000,
            speechMax: 1000,
            voiceReady: true,
          },
        ],
      }),
    })
    service(SpokenVoiceService, {
      listSpokenDrafts: () => ({ drafts: [...drafts.values()] }),
      listSpokenVoices: () => ({ voices: [...voices.values()].filter((v) => !v.removedAt) }),
      getSpokenDraft: (r) => ({ draft: draft(r.id) }),
      createSpokenDraft: (r) => {
        calls.push('create')
        const d = create(SpokenDraftSchema, {
          name: r.input?.name,
          description: r.input?.description,
          previewText: r.input?.previewText,
          qualificationSessionId: r.input?.qualificationSessionId,
          id: SPOKEN_TEST_DRAFT,
          revision: revision++,
          profile,
          phase: 'editing',
        })
        drafts.set(d.id, d)
        return { draft: d }
      },
      updateSpokenDraft: (r) => {
        calls.push('save')
        const d = check(r.id, r.expectedRevision)
        if (
          r.input?.description !== d.description ||
          r.input.previewText !== d.previewText ||
          r.input.profileRevision !== d.profile?.revision
        ) {
          d.candidates = []
          d.selectedCandidateId = ''
        }
        d.name = r.input?.name ?? ''
        d.description = r.input?.description ?? ''
        d.previewText = r.input?.previewText ?? ''
        d.revision = revision++
        return { draft: d }
      },
      selectSpokenCandidate: (r) => {
        calls.push('select')
        const d = check(r.draftId, r.expectedRevision)
        d.selectedCandidateId = r.candidateId
        d.revision = revision++
        return { draft: d }
      },
      acknowledgeSpokenCandidate: (r) => {
        calls.push('acknowledge')
        if (!ticketRequested || r.playbackId !== SPOKEN_TEST_TICKET)
          throw new ConnectError('unplayed', Code.FailedPrecondition)
        const d = check(r.draftId, r.expectedRevision)
        const c = d.candidates.find((c) => c.id === r.candidateId)
        if (!c) throw new ConnectError('stale candidate', Code.NotFound)
        c.auditionedAt = new Date().toISOString()
        d.revision = revision++
        return { draft: d }
      },
      getSpokenSampleAccess: () => {
        calls.push('sample')
        ticketRequested = true
        return {
          playbackId: SPOKEN_TEST_TICKET,
          url: '/spoken/audio/' + SPOKEN_TEST_TICKET,
          expiresAt: new Date(Date.now() + 60000).toISOString(),
        }
      },
      renameSpokenVoice: (r) => {
        calls.push('rename')
        const v = voices.get(r.id)
        if (!v || v.revision !== r.expectedRevision) throw new ConnectError('stale', Code.Aborted)
        v.name = r.name
        v.revision++
        return { voice: v }
      },
      removeSpokenVoice: (r) => {
        calls.push('remove')
        const v = voices.get(r.id)
        if (!v) throw new ConnectError('missing', Code.NotFound)
        v.removedAt = new Date().toISOString()
        v.revision++
        return { voice: v }
      },
    })
    service(SpokenVoiceGenerationService, {
      quoteVoiceCandidates: (r) => {
        calls.push('quote-design')
        check(r.draftId, r.expectedRevision)
        return quote()
      },
      quoteVoiceConfirmation: (r) => {
        calls.push('quote-confirm')
        const d = check(r.draftId, r.expectedRevision)
        if (!d.candidates.find((c) => c.id === r.candidateId)?.auditionedAt)
          throw new ConnectError('listen first', Code.FailedPrecondition)
        return { ...quote(), maximumCredits: 0 }
      },
      startVoiceCandidates: (r) => {
        calls.push('start-design')
        startKeys.push(r.idempotencyKey)
        if (failStart) throw new ConnectError('offline', Code.Unavailable)
        const o = create(SpokenOperationSchema, {
          id: SPOKEN_TEST_OPERATION,
          kind: 'voice_design',
          state: 'queued',
          draftId: r.input?.draftId,
          jobId: 'job',
        })
        operations.set(o.id, o)
        return { operation: o }
      },
      startVoiceConfirmation: (r) => {
        calls.push('start-confirm')
        if (!r.approval || r.approval.approvedMaxCredits !== 0)
          throw new ConnectError('explicit confirmation required', Code.FailedPrecondition)
        const o = create(SpokenOperationSchema, {
          id: 'b'.repeat(32),
          kind: 'voice_confirm',
          state: 'queued',
          draftId: r.input?.draftId,
          candidateId: r.input?.candidateId,
          jobId: 'job',
        })
        operations.set(o.id, o)
        return { operation: o }
      },
      getSpokenOperation: (r) => {
        const o = operations.get(r.id)
        if (!o) throw new ConnectError('missing', Code.NotFound)
        return { operation: progress(o) }
      },
      cancelSpokenOperation: (r) => {
        calls.push('cancel')
        const o = operations.get(r.id)!
        o.state = 'cancelled'
        return { operation: o }
      },
      retrySpokenPublication: (r) => {
        calls.push('recover')
        const o = operations.get(r.id)!
        o.state = 'published'
        return { operation: o }
      },
    })
  })
  return {
    transport,
    calls,
    startKeys,
    drafts,
    voices,
    operations,
    setPending: (v: boolean) => {
      pending = v
    },
    setFailStart: (v: boolean) => {
      failStart = v
    },
  }
}
