// Shared fake PostService for tests.
//
// It models the server rules the frontend actually depends on: an empty slug mints one
// (`YYYYMMDD-title`, serial suffix on collision), someone else's slug is 403 and a missing
// one is 404 (spec/legacy/policy/posts.md); an upload is a CreateUpload → ConfirmUpload pair
// and a confirmed filename is taken (spec/legacy/policy/uploads.md). Everything else is kept as
// thin as possible.
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  ConfirmUploadResponseSchema,
  CreateUploadResponseSchema,
  DeleteImageResponseSchema,
  FinalizePostResponseSchema,
  GetPostResponseSchema,
  type Image,
  ImageSchema,
  ListPostsResponseSchema,
  PostSchema,
  PostService,
  PostSummarySchema,
  type ProtoGenerationJob,
  type ProtoTemplateRef,
  TemplateRefSchema,
  type Observation,
  type PostContent,
  SavePostDraftResponseSchema,
  SavePostContentResponseSchema,
  SavePostGenerationOptionsResponseSchema,
  type ProtoVoiceRef,
  VoiceRefSchema,
  contentLanguageFromProto,
  contentLanguageToProto,
  type ContentLanguage,
} from '@/shared/api'
import { type FakeGenerationJobRow, toFakeProto } from './jobs'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeImageRow {
  id: string
  filename: string
  width?: number
  height?: number
  viewUrl?: string
}

/** A voice as the post fake knows it: just enough to answer a post's `voice` projection and to
 *  accept or refuse an assignment. The voice fake owns the full directory. */
export interface FakePostVoice {
  id: string
  name: string
  deleted?: boolean
  sourceLanguage?: ContentLanguage
}

/** The voice every fixture post is written in unless it says otherwise — the same one the voice
 *  fake starts with, so a screen can go from a post to its voice's profile. */
export const DEFAULT_POST_VOICE: FakePostVoice = {
  id: 'voice-default',
  name: '기본 말투',
  sourceLanguage: 'ko',
}

/** A 템플릿 as the post fake knows it: just enough to answer a post's `template` projection and to
 *  accept or refuse an assignment. The template fake owns the full directory. */
export interface FakePostTemplate {
  id: string
  name: string
}

/** One SavePostDraft as the server saw its assignments: present on a create or a change,
 *  absent on an ordinary edit (spec/legacy/policy/posts.md, spec/legacy/policy/templates.md). An empty
 *  `templateId` is a real value — it clears the assignment. */
export interface FakeDraftSave {
  slug: string
  voiceId: string | undefined
  templateId: string | undefined
  targetLanguage: ContentLanguage | undefined
}

export interface FakePostRow {
  slug: string
  title?: string
  memo?: string
  status?: string
  createdAt?: string
  updatedAt?: string
  voice?: FakePostVoice
  template?: FakePostTemplate
  machineBaselineVoiceId?: string
  images?: FakeImageRow[]
  activeJob?: FakeGenerationJobRow
  content?: PostContent
  observations?: Observation[]
  pendingExperimentId?: string
  contentRevision?: bigint
  machineBaselineRevision?: bigint
  canFinalize?: boolean
  targetLength?: number
  finalizedRevision?: bigint
  finalizedAt?: string
  targetLanguage?: ContentLanguage
  /** `null` deliberately models malformed pre-migration data; learned content defaults to the
   *  migration's Korean backfill when a fixture omits the field. */
  contentLanguage?: ContentLanguage | null
}

export interface FakePostsOptions {
  /** The acting user's posts, in the order ListPosts should return them. */
  posts?: FakePostRow[]
  /** Optional successive snapshots returned by GetPost for the same slug. */
  getSequence?: FakePostRow[]
  /** Slugs that exist but belong to someone else — 403, the way the server answers. */
  foreign?: string[]
  /** Make ListPosts fail. */
  listFails?: boolean
  /** Fail this many SavePostDraft calls before the first success. */
  failSaves?: number
  /** Answer SavePostDraft 200 with no post — a confirmation the client must not trust. */
  saveReturnsNoPost?: boolean
  /** Make DeleteImage fail. */
  deleteFails?: boolean
  /** The date the fake mints slugs from. */
  today?: string
  /** Records every procedure the transport was asked for. */
  calls?: string[]
  /** Records target-length option saves, including an explicit clear as undefined. */
  generationOptionSaves?: Array<number | undefined>
  /** The voices a post may be assigned to. Omitted, only `DEFAULT_POST_VOICE` exists. */
  voices?: FakePostVoice[]
  /** The 템플릿 a post may be assigned to. Omitted, the account has none. */
  templates?: FakePostTemplate[]
  /** Records every SavePostDraft's slug and assignment presence. */
  draftSaves?: FakeDraftSave[]
  /** Holds SavePostContent in flight until a test releases it. */
  contentSaveGate?: Promise<void>
}

const DEFAULT_UPDATED_AT = '2026-08-28T12:00:00Z'

/** The storage host presigned URLs point at — anything but the API's own origin. */
export const FAKE_STORAGE_ORIGIN = 'https://storage.test'

type Row = {
  slug: string
  title: string
  memo: string
  status: string
  createdAt: string
  updatedAt: string
  voice: ProtoVoiceRef
  template?: ProtoTemplateRef
  images: Image[]
  activeJob?: ProtoGenerationJob
  content?: PostContent
  observations: Observation[]
  pendingExperimentId: string
  contentRevision: bigint
  machineBaselineRevision: bigint
  machineBaselineVoiceId: string
  canFinalize: boolean
  targetLength?: number
  finalizedRevision: bigint
  finalizedAt: string
  targetLanguage: ReturnType<typeof contentLanguageToProto>
  contentLanguage: ReturnType<typeof contentLanguageToProto>
}

export function registerPostService(router: ConnectRouter, options: FakePostsOptions = {}) {
  const { rpc } = router
  const { foreign = [], listFails, today = '20260828', calls } = options
  let failuresLeft = options.failSaves ?? 0
  let uploadSequence = 0
  let getSequenceIndex = 0
  const pending = new Map<string, { slug: string; filename: string }>()
  const voices = options.voices ?? [DEFAULT_POST_VOICE]
  const templates = options.templates ?? []

  const toTemplateRef = (template: FakePostTemplate) =>
    create(TemplateRefSchema, { id: template.id, name: template.name })

  /** Like the server: an unknown or foreign id is 404 and is never substituted with 없음. */
  function assignableTemplate(templateId: string): ProtoTemplateRef | undefined {
    if (templateId === '') return undefined
    const template = templates.find((candidate) => candidate.id === templateId)
    if (!template) throw connectAppError('TEMPLATE_NOT_FOUND', Code.NotFound)
    return toTemplateRef(template)
  }

  const toVoiceRef = (voice: FakePostVoice) =>
    create(VoiceRefSchema, {
      id: voice.id,
      name: voice.name,
      deleted: voice.deleted ?? false,
      sourceLanguage: contentLanguageToProto(voice.sourceLanguage ?? 'ko'),
    })

  /** Like the server: an unknown voice is 404, a deleted one is refused, never substituted. */
  function assignable(voiceId: string): ProtoVoiceRef {
    const voice = voices.find((candidate) => candidate.id === voiceId)
    if (!voice) throw connectAppError('VOICE_NOT_FOUND', Code.NotFound)
    if (voice.deleted) throw connectAppError('VOICE_DELETED', Code.FailedPrecondition)
    return toVoiceRef(voice)
  }

  function toRow(row: FakePostRow): Row {
    const voice = row.voice ?? DEFAULT_POST_VOICE
    return {
      slug: row.slug,
      title: row.title ?? '',
      memo: row.memo ?? '',
      status: row.status ?? 'draft',
      createdAt: row.createdAt ?? DEFAULT_UPDATED_AT,
      updatedAt: row.updatedAt ?? DEFAULT_UPDATED_AT,
      voice: toVoiceRef(voice),
      template: row.template ? toTemplateRef(row.template) : undefined,
      machineBaselineVoiceId:
        row.machineBaselineVoiceId ?? ((row.machineBaselineRevision ?? 0n) > 0n ? voice.id : ''),
      images: (row.images ?? []).map((image) =>
        create(ImageSchema, {
          id: image.id,
          filename: image.filename,
          width: image.width ?? 1024,
          height: image.height ?? 768,
          bytes: 200_000n,
          viewUrl:
            image.viewUrl ?? `${FAKE_STORAGE_ORIGIN}/posts/${row.slug}/${image.id}.jpg?sig=1`,
        }),
      ),
      activeJob: row.activeJob ? toFakeProto(row.activeJob) : undefined,
      content: row.content,
      observations: row.observations ?? [],
      pendingExperimentId: row.pendingExperimentId ?? '',
      contentRevision: row.contentRevision ?? 0n,
      machineBaselineRevision: row.machineBaselineRevision ?? 0n,
      canFinalize:
        row.canFinalize ?? Boolean(row.content && (row.machineBaselineRevision ?? 0n) > 0n),
      targetLength: row.targetLength,
      finalizedRevision: row.finalizedRevision ?? 0n,
      finalizedAt: row.finalizedAt ?? '',
      targetLanguage: contentLanguageToProto(row.targetLanguage ?? 'ko'),
      contentLanguage:
        row.contentLanguage === null
          ? 0
          : row.contentLanguage
            ? contentLanguageToProto(row.contentLanguage)
            : row.content
              ? contentLanguageToProto('ko')
              : 0,
    }
  }

  const rows = new Map<string, Row>((options.posts ?? []).map((row) => [row.slug, toRow(row)]))

  function mintSlug(title: string): string {
    const base = title.trim().toLowerCase().replace(/\s+/g, '-') || 'untitled'
    let slug = `${today}-${base}`
    for (let serial = 2; rows.has(slug); serial += 1) slug = `${today}-${base}-${serial}`
    return slug
  }

  function toProto(row: Row) {
    return create(PostSchema, row)
  }

  /** Like the server, GetPost mints a view URL for every photo, fresh each time. */
  function withViewUrls(row: Row) {
    return create(PostSchema, {
      ...row,
      images: row.images.map((image) =>
        create(ImageSchema, {
          ...image,
          viewUrl:
            image.viewUrl || `${FAKE_STORAGE_ORIGIN}/posts/${row.slug}/${image.id}.jpg?sig=get`,
        }),
      ),
    })
  }

  rpc(PostService.method.listPosts, () => {
    calls?.push('ListPosts')
    if (listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListPostsResponseSchema, {
      posts: [...rows.values()].map((row) => create(PostSummarySchema, row)),
    })
  })

  rpc(PostService.method.getPost, (req) => {
    calls?.push('GetPost')
    if (foreign.includes(req.slug)) throw connectAppError('POST_FORBIDDEN', Code.PermissionDenied)
    const sequenced =
      options.getSequence?.[Math.min(getSequenceIndex, options.getSequence.length - 1)]
    if (sequenced?.slug === req.slug) {
      getSequenceIndex += 1
      rows.set(req.slug, toRow(sequenced))
    }
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    return create(GetPostResponseSchema, { post: withViewUrls(row) })
  })

  rpc(PostService.method.savePostDraft, (req) => {
    calls?.push('SavePostDraft')
    options.draftSaves?.push({
      slug: req.slug,
      voiceId: req.voiceId,
      templateId: req.templateId,
      targetLanguage:
        req.targetLanguage === undefined ? undefined : contentLanguageFromProto(req.targetLanguage),
    })
    if (failuresLeft > 0) {
      failuresLeft -= 1
      throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    }
    if (options.saveReturnsNoPost) return create(SavePostDraftResponseSchema, {})
    if (req.slug && foreign.includes(req.slug)) {
      throw connectAppError('POST_FORBIDDEN', Code.PermissionDenied)
    }
    const existing = req.slug ? rows.get(req.slug) : undefined
    const requestedTarget =
      req.targetLanguage === undefined ? undefined : contentLanguageFromProto(req.targetLanguage)
    if (!req.slug && !requestedTarget)
      throw connectAppError('POST_TARGET_LANGUAGE_REQUIRED', Code.InvalidArgument)
    if (req.targetLanguage !== undefined && !requestedTarget)
      throw connectAppError('POST_TARGET_LANGUAGE_UNSUPPORTED', Code.InvalidArgument)
    // The server's assignment rules (spec/legacy/policy/posts.md): a create names its voice, an edit
    // that omits it preserves it, and a different present value reassigns — refused while a job
    // or an undecided A/B result could still write a baseline for the old voice.
    // Validated before anything else is applied, like the server: a bad 템플릿 must leave the
    // title and memo exactly as they were.
    let template = existing?.template
    if (req.templateId !== undefined) template = assignableTemplate(req.templateId)
    let voice = existing?.voice ?? toVoiceRef(DEFAULT_POST_VOICE)
    let reassigned = false
    if (!req.slug) {
      if (!req.voiceId) throw connectAppError('VOICE_REQUIRED', Code.InvalidArgument)
      voice = assignable(req.voiceId)
    } else if (req.voiceId !== undefined && req.voiceId !== voice.id) {
      const next = assignable(req.voiceId)
      const busy =
        (existing?.activeJob &&
          existing.activeJob.status !== 'done' &&
          existing.activeJob.status !== 'failed') ||
        Boolean(existing?.pendingExperimentId)
      if (busy) throw connectAppError('POST_BUSY', Code.FailedPrecondition)
      voice = next
      reassigned = true
    }
    const slug = req.slug || mintSlug(req.title)
    const row: Row = {
      slug,
      title: req.title,
      memo: req.memo,
      status: existing?.status ?? 'draft',
      createdAt: existing?.createdAt ?? DEFAULT_UPDATED_AT,
      updatedAt: DEFAULT_UPDATED_AT,
      voice,
      template,
      images: existing?.images ?? [],
      activeJob: existing?.activeJob,
      content: existing?.content,
      observations: existing?.observations ?? [],
      pendingExperimentId: existing?.pendingExperimentId ?? '',
      contentRevision: existing?.contentRevision ?? 0n,
      machineBaselineRevision: reassigned ? 0n : (existing?.machineBaselineRevision ?? 0n),
      machineBaselineVoiceId: reassigned ? '' : (existing?.machineBaselineVoiceId ?? ''),
      canFinalize: reassigned ? Boolean(existing?.content) : (existing?.canFinalize ?? false),
      targetLength: existing?.targetLength,
      finalizedRevision: existing?.finalizedRevision ?? 0n,
      finalizedAt: existing?.finalizedAt ?? '',
      targetLanguage: contentLanguageToProto(
        requestedTarget ??
          (existing ? contentLanguageFromProto(existing.targetLanguage) : undefined) ??
          'ko',
      ),
      contentLanguage: existing?.contentLanguage ?? 0,
    }
    rows.set(slug, row)
    return create(SavePostDraftResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.createUpload, (req) => {
    calls?.push('CreateUpload')
    const row = rows.get(req.postSlug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    if (row.images.some((image) => image.filename === req.filename)) {
      throw connectAppError('POST_FILENAME_TAKEN', Code.AlreadyExists, {
        filename: req.filename,
      })
    }
    uploadSequence += 1
    const uploadId = `upload-${uploadSequence}`
    pending.set(uploadId, { slug: req.postSlug, filename: req.filename })
    return create(CreateUploadResponseSchema, {
      uploadId,
      putUrl: `${FAKE_STORAGE_ORIGIN}/posts/${req.postSlug}/${uploadId}.jpg?sig=put`,
      contentType: 'image/jpeg',
      expiresAt: '2026-08-28T12:10:00Z',
    })
  })

  rpc(PostService.method.savePostContent, async (req) => {
    calls?.push('SavePostContent')
    await options.contentSaveGate
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    if (row.contentRevision !== req.expectedRevision)
      throw connectAppError('POST_CONTENT_STALE', Code.Aborted)
    if (!req.content) throw connectAppError('POST_CONTENT_INVALID', Code.InvalidArgument)
    if (JSON.stringify(row.content) !== JSON.stringify(req.content)) {
      row.content = req.content
      row.contentRevision += 1n
      row.status = 'review'
      row.finalizedRevision = 0n
      row.finalizedAt = ''
    }
    return create(SavePostContentResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.savePostGenerationOptions, (req) => {
    calls?.push('SavePostGenerationOptions')
    options.generationOptionSaves?.push(req.targetLength)
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    row.targetLength = req.targetLength
    return create(SavePostGenerationOptionsResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.finalizePost, (req) => {
    calls?.push('FinalizePost')
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    if (row.contentRevision !== req.expectedRevision)
      throw connectAppError('POST_CONTENT_STALE', Code.Aborted)
    if (!row.content || row.machineBaselineRevision <= 0n)
      throw connectAppError('POST_MACHINE_BASELINE_REQUIRED', Code.FailedPrecondition)
    row.status = 'finalized'
    row.finalizedRevision = row.contentRevision
    row.finalizedAt = DEFAULT_UPDATED_AT
    // Like the server: the confirmed content's title becomes the post's title, and an untitled
    // generation leaves the working title in place (spec/legacy/policy/posts.md).
    row.title = row.content.title.trim() || row.title
    return create(FinalizePostResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.confirmUpload, (req) => {
    calls?.push('ConfirmUpload')
    const upload = pending.get(req.uploadId)
    if (!upload) throw connectAppError('UPLOAD_NOT_FOUND', Code.NotFound)
    pending.delete(req.uploadId)
    const image = create(ImageSchema, {
      id: req.uploadId,
      filename: upload.filename,
      width: req.width,
      height: req.height,
      bytes: 200_000n,
    })
    rows.get(upload.slug)?.images.push(image)
    return create(ConfirmUploadResponseSchema, { image })
  })

  rpc(PostService.method.deleteImage, (req) => {
    calls?.push('DeleteImage')
    if (options.deleteFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    for (const row of rows.values()) {
      const index = row.images.findIndex((image) => image.id === req.imageId)
      if (index !== -1) {
        row.images.splice(index, 1)
        return create(DeleteImageResponseSchema, {})
      }
    }
    throw connectAppError('UPLOAD_NOT_FOUND', Code.NotFound)
  })
}

/** A transport serving only PostService — for tests of the post hooks themselves. */
export function createFakePostsTransport(options: FakePostsOptions = {}) {
  return createRouterTransport((router) => registerPostService(router, options))
}
