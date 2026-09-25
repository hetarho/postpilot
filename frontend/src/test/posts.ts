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
  AttachmentKind,
  DeleteVideoResponseSchema,
  type Image,
  ImageSchema,
  type Video,
  VideoSchema,
  ListPostsResponseSchema,
  PostSchema,
  PostService,
  PostSummarySchema,
  type ProtoGenerationJob,
  type ProtoTemplateAnswer,
  type ProtoTemplateRef,
  TemplateAnswerSchema,
  TemplateRefSchema,
  type Observation,
  type PostContent,
  PostContentSchema,
  ReplacementCandidateSchema,
  type ProtoReplacementCandidate,
  SavePostDraftResponseSchema,
  SavePostContentResponseSchema,
  SavePostGenerationOptionsResponseSchema,
  SavePostPublishedUrlResponseSchema,
  type ProtoVoiceRef,
  ProtoBlogField,
  VoiceRefSchema,
  contentLanguageFromProto,
  contentLanguageToProto,
  type ContentLanguage,
} from '@/shared/api'
import { blogFieldFromProto, blogFieldToProto, isBlogFieldId } from '@/entities/blog-field'
import {
  qualityMetricFromProto,
  qualityMetricToProto,
  type QualityMetricId,
} from '@/entities/quality'
import { parseNaverBlogUrl, replacementSurfaceToProto } from '@/entities/post'
import { type FakeGenerationJobRow, toFakeProto } from './jobs'
import { connectAppError } from './app-error'
import { POST_CONTENT_FIXTURE } from './fixtures/postContent'

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
  /** What this template seeds onto a post it is assigned to (TEMPLATE-48). Absent is 의견 없음:
   *  the assignment then leaves the post's own option alone. */
  targetLength?: number
  tagCount?: number
}

/** One SavePostDraft as the server saw its assignments: present on a create or a change,
 *  absent on an ordinary edit (spec/legacy/policy/posts.md, spec/legacy/policy/templates.md). An empty
 *  `templateId` is a real value — it clears the assignment. */
export interface FakeDraftSave {
  slug: string
  voiceId: string | undefined
  templateId: string | undefined
  targetLanguage: ContentLanguage | undefined
  /** The 분야 this save carried: undefined when absent, '' for a present UNSPECIFIED — the clear —
   *  and otherwise the id (POST-82). */
  field: string | undefined
  /** The data-field answers this save carried, exactly as they arrived. Every entry is an
   *  upsert of that label, so a test can prove one save carried the whole set on screen and
   *  nothing else (POST-62). */
  templateAnswers: Array<{ label: string; text: string; enabled: boolean }>
}

/** One SavePostGenerationOptions as it arrived: every member undefined when absent, which only an
 *  old tab's partial save leaves out (POST-89). `field` is recorded like `FakeDraftSave.field`. */
export interface FakeOptionsSave {
  slug: string
  targetLength: number | undefined
  tagCount: number | undefined
  useMemory: boolean | undefined
  qualityRules: QualityMetricId[] | undefined
  field: string | undefined
}

/** One clip on a fake post. Only the fields a test actually varies; the rest are filled with
 *  the same defaults the server would produce. */
export interface FakeVideoRow {
  id: string
  filename: string
  width?: number
  height?: number
  durationMs?: bigint
  contentType?: string
  viewUrl?: string
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
  /** The post's 분야 as an `entities/blog-field` id; omitted is 없음. */
  field?: string
  /** What this post already answers to its template's data fields. */
  templateAnswers?: Array<{ label: string; text?: string; enabled?: boolean }>
  /** Shorthand for a post whose content carries these tags. A tag lives inside the content
   *  (POST-65), so passing them without `content` synthesizes the minimal content that would
   *  hold them — which is also the only way a real post comes to have one. */
  tags?: string[]
  machineBaselineVoiceId?: string
  images?: FakeImageRow[]
  videos?: FakeVideoRow[]
  activeJob?: FakeGenerationJobRow
  content?: PostContent
  /** What the last write offered to replace, as stored (GEN-53). */
  replacementCandidates?: Array<{
    surface: 'title' | 'tag' | 'body'
    index: number
    source: string
    phrases: string[]
  }>
  observations?: Observation[]
  pendingExperimentId?: string
  contentRevision?: bigint
  machineBaselineRevision?: bigint
  canFinalize?: boolean
  targetLength?: number
  tagCount?: number
  /** The memory opt-in (MEM-18). Omitted means off, which is what every draft saved before
   *  memories existed reads as. */
  useMemory?: boolean
  /** The quality metrics ticked for the next run (POST-81); omitted means none. */
  qualityRules?: QualityMetricId[]
  finalizedRevision?: bigint
  finalizedAt?: string
  /** A published post's Naver Blog address and when it was recorded; `status: 'published'` is
   *  what locks it. */
  publishedUrl?: string
  publishedAt?: string
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
  /** Every SavePostGenerationOptions as it arrived, so a test can prove one 저장 was one request
   *  carrying the whole set (POST-89). */
  optionSaves?: FakeOptionsSave[]
  /** Holds SavePostGenerationOptions in flight, after it is recorded, until a test releases it. */
  optionSaveGate?: Promise<void>
  /** Refuse every option save, so the form's failure path is testable. */
  optionSaveFails?: boolean
  /** The voices a post may be assigned to. Omitted, only `DEFAULT_POST_VOICE` exists. */
  voices?: FakePostVoice[]
  /** The 템플릿 a post may be assigned to. Omitted, the account has none. */
  templates?: FakePostTemplate[]
  /** Records every SavePostDraft's slug and assignment presence. */
  draftSaves?: FakeDraftSave[]
  /** Holds SavePostContent in flight until a test releases it. */
  contentSaveGate?: Promise<void>
  /** Every SavePostContent as it arrived: slug, the revision it expected and the content. */
  contentSaves?: Array<{
    slug: string
    expectedRevision: bigint
    content: PostContent
    /** The replacement candidates the save spent, as indices into the stored list (POST-79). */
    takenCandidates: number[]
  }>
  /** The next SavePostDraft on this slug first publishes the post and is then refused as
   *  locked, the way a publish from another tab lands between two autosaves (POST-86). */
  publishOnDraftSave?: string
  /** Every SavePostPublishedUrl's `url`, as sent. */
  publishedUrlSaves?: string[]
  /** Refuse every SavePostPublishedUrl as POST_BUSY, the way a running content job does. */
  publishedUrlBusy?: boolean
}

/** The address a publish from another tab records in `publishOnDraftSave`. */
export const FAKE_PUBLISHED_URL = 'https://blog.naver.com/alice/1'

/** A post finalized at its only revision, holding the shared fixture content. `canFinalize` is
 *  explicit because a SavePostDraft answer reads it from the row rather than deriving it. */
export function finalizedPostRow(row: FakePostRow & { slug: string }): FakePostRow {
  return {
    status: 'finalized',
    content: POST_CONTENT_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
    finalizedRevision: 1n,
    finalizedAt: '2026-08-20T12:00:00Z',
    ...row,
  }
}

/** A finalized post published at the fixture address (POST-73). */
export function publishedPostRow(row: FakePostRow & { slug: string }): FakePostRow {
  return finalizedPostRow({
    status: 'published',
    publishedUrl: FAKE_PUBLISHED_URL,
    publishedAt: '2026-08-21T09:00:00Z',
    ...row,
  })
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
  field: ProtoBlogField
  templateAnswers: ProtoTemplateAnswer[]
  images: Image[]
  videos: Video[]
  activeJob?: ProtoGenerationJob
  content?: PostContent
  replacementCandidates: ProtoReplacementCandidate[]
  observations: Observation[]
  pendingExperimentId: string
  contentRevision: bigint
  machineBaselineRevision: bigint
  machineBaselineVoiceId: string
  canFinalize: boolean
  targetLength?: number
  tagCount: number
  useMemory: boolean
  qualityRules: ReturnType<typeof qualityMetricToProto>[]
  finalizedRevision: bigint
  finalizedAt: string
  publishedUrl: string
  publishedAt: string
  targetLanguage: ReturnType<typeof contentLanguageToProto>
  contentLanguage: ReturnType<typeof contentLanguageToProto>
}

/** A fixture's 분야. An id the catalogue does not hold is a mistake in the test, not 없음. */
function fixtureField(id: string | undefined): ProtoBlogField {
  if (!id) return ProtoBlogField.UNSPECIFIED
  if (!isBlogFieldId(id)) throw new Error(`fake post: unknown 분야 ${id}`)
  return blogFieldToProto(id)
}

/** Like the server (T334): a published post takes no write but its address and the delete. */
function refuseIfPublished(row: Row): void {
  if (row.status === 'published')
    throw connectAppError('POST_PUBLISHED_LOCKED', Code.FailedPrecondition)
}

export function registerPostService(router: ConnectRouter, options: FakePostsOptions = {}) {
  const { rpc } = router
  const { foreign = [], listFails, today = '20260828', calls } = options
  let failuresLeft = options.failSaves ?? 0
  let publishOnDraftSave = options.publishOnDraftSave
  // The fake clock a publish is stamped with: each one later than the last (POST-75).
  let publishSequence = 0
  let uploadSequence = 0
  let getSequenceIndex = 0
  // `video` is the RESERVATION's kind: the confirm answers with the half the upload asked for,
  // never with the half the request implies.
  const pending = new Map<string, { slug: string; filename: string; video: boolean }>()
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

  /** The server's rule: each entry sets that label, an absent label is preserved, and nothing
   *  is ever deleted. */
  function upsertAnswers(
    saved: ProtoTemplateAnswer[],
    incoming: ProtoTemplateAnswer[],
  ): ProtoTemplateAnswer[] {
    const merged = [...saved]
    for (const answer of incoming) {
      const at = merged.findIndex((candidate) => candidate.label === answer.label)
      if (at === -1) merged.push(answer)
      else merged[at] = answer
    }
    return merged
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
      field: fixtureField(row.field),
      templateAnswers: (row.templateAnswers ?? []).map((answer) =>
        create(TemplateAnswerSchema, {
          label: answer.label,
          text: answer.text ?? '',
          enabled: answer.enabled ?? true,
        }),
      ),
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
      videos: (row.videos ?? []).map((video) =>
        create(VideoSchema, {
          id: video.id,
          filename: video.filename,
          width: video.width ?? 1920,
          height: video.height ?? 1080,
          bytes: 12_000_000n,
          durationMs: video.durationMs ?? 8_000n,
          contentType: video.contentType || 'video/mp4',
          viewUrl:
            video.viewUrl ?? `${FAKE_STORAGE_ORIGIN}/posts/${row.slug}/${video.id}.mp4?sig=1`,
        }),
      ),
      activeJob: row.activeJob ? toFakeProto(row.activeJob) : undefined,
      content:
        row.content ??
        (row.tags
          ? create(PostContentSchema, { title: row.title ?? '', tags: row.tags })
          : undefined),
      replacementCandidates: (row.replacementCandidates ?? []).map((candidate) =>
        create(ReplacementCandidateSchema, {
          ...candidate,
          surface: replacementSurfaceToProto(candidate.surface),
        }),
      ),
      observations: row.observations ?? [],
      pendingExperimentId: row.pendingExperimentId ?? '',
      contentRevision: row.contentRevision ?? 0n,
      machineBaselineRevision: row.machineBaselineRevision ?? 0n,
      canFinalize:
        row.canFinalize ?? Boolean(row.content && (row.machineBaselineRevision ?? 0n) > 0n),
      targetLength: row.targetLength,
      // The real server always fills it (POST-63): a row never saved with one is the default.
      tagCount: row.tagCount ?? 4,
      useMemory: row.useMemory ?? false,
      qualityRules: (row.qualityRules ?? []).map(qualityMetricToProto),
      finalizedRevision: row.finalizedRevision ?? 0n,
      finalizedAt: row.finalizedAt ?? '',
      publishedUrl: row.publishedUrl ?? '',
      publishedAt: row.publishedAt ?? '',
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
      videos: row.videos.map((video) =>
        create(VideoSchema, {
          ...video,
          viewUrl:
            video.viewUrl || `${FAKE_STORAGE_ORIGIN}/posts/${row.slug}/${video.id}.mp4?sig=get`,
        }),
      ),
    })
  }

  rpc(PostService.method.listPosts, () => {
    calls?.push('ListPosts')
    if (listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListPostsResponseSchema, {
      // The summary's tags are the content's, read out of the content the way the server
      // reads them rather than out of a column of their own (POST-65).
      posts: [...rows.values()].map((row) =>
        create(PostSummarySchema, { ...row, tags: row.content?.tags ?? [] }),
      ),
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
      templateAnswers: req.templateAnswers.map((answer) => ({
        label: answer.label,
        text: answer.text,
        enabled: answer.enabled,
      })),
      targetLanguage:
        req.targetLanguage === undefined ? undefined : contentLanguageFromProto(req.targetLanguage),
      field:
        req.field === undefined ? undefined : (blogFieldFromProto(req.field) ?? `?${req.field}`),
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
    if (existing && publishOnDraftSave === existing.slug) {
      publishOnDraftSave = undefined
      existing.status = 'published'
      existing.publishedUrl = FAKE_PUBLISHED_URL
      existing.publishedAt = '2026-08-28T12:30:00Z'
    }
    // Before any validation or write, like the server: a locked post changes nothing.
    if (existing) refuseIfPublished(existing)
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
    // The 분야 the same way (POST-82): absent keeps, UNSPECIFIED clears, and a number the server
    // does not know is 404 before anything is written.
    let field = existing?.field ?? ProtoBlogField.UNSPECIFIED
    if (req.field !== undefined) {
      if (blogFieldFromProto(req.field) === undefined)
        throw connectAppError('POST_FIELD_NOT_FOUND', Code.NotFound)
      field = req.field
    }
    // What the assignment seeds, resolved before anything is written: only an assignment that
    // CHANGES the template seeds, and only for the numbers that template has an opinion about
    // (TEMPLATE-48).
    let seededLength = existing?.targetLength
    let seededTags = existing?.tagCount ?? 4
    if (req.templateId !== undefined) {
      template = assignableTemplate(req.templateId)
      if (template && template.id !== existing?.template?.id) {
        const source = templates.find((candidate) => candidate.id === template?.id)
        if (source?.targetLength !== undefined) seededLength = source.targetLength
        if (source?.tagCount !== undefined) seededTags = source.tagCount
      }
    }
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
      field,
      // Upsert per label, never a replacement of the set — the server's rule, so a test cannot
      // pass here on behavior the server would not produce.
      templateAnswers: upsertAnswers(existing?.templateAnswers ?? [], req.templateAnswers),
      images: existing?.images ?? [],
      videos: existing?.videos ?? [],
      activeJob: existing?.activeJob,
      content: existing?.content,
      // Kept by a draft save, as by a manual content save and a revision (T341).
      replacementCandidates: existing?.replacementCandidates ?? [],
      observations: existing?.observations ?? [],
      pendingExperimentId: existing?.pendingExperimentId ?? '',
      contentRevision: existing?.contentRevision ?? 0n,
      machineBaselineRevision: reassigned ? 0n : (existing?.machineBaselineRevision ?? 0n),
      machineBaselineVoiceId: reassigned ? '' : (existing?.machineBaselineVoiceId ?? ''),
      canFinalize: reassigned ? Boolean(existing?.content) : (existing?.canFinalize ?? false),
      targetLength: seededLength,
      tagCount: seededTags,
      useMemory: existing?.useMemory ?? false,
      // A draft save never touches the ticks; only the options save does.
      qualityRules: existing?.qualityRules ?? [],
      finalizedRevision: existing?.finalizedRevision ?? 0n,
      finalizedAt: existing?.finalizedAt ?? '',
      publishedUrl: existing?.publishedUrl ?? '',
      publishedAt: existing?.publishedAt ?? '',
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
    refuseIfPublished(row)
    // ONE filename namespace across both kinds, like the server's (VIDEO-5).
    const taken =
      row.images.some((image) => image.filename === req.filename) ||
      row.videos.some((video) => video.filename === req.filename)
    if (taken) {
      throw connectAppError('POST_FILENAME_TAKEN', Code.AlreadyExists, {
        filename: req.filename,
      })
    }
    uploadSequence += 1
    const uploadId = `upload-${uploadSequence}`
    const isVideo = req.kind === AttachmentKind.VIDEO
    pending.set(uploadId, { slug: req.postSlug, filename: req.filename, video: isVideo })
    return create(CreateUploadResponseSchema, {
      uploadId,
      putUrl: `${FAKE_STORAGE_ORIGIN}/posts/${req.postSlug}/${uploadId}.${isVideo ? 'mp4' : 'jpg'}?sig=put`,
      contentType: isVideo ? 'video/mp4' : 'image/jpeg',
      expiresAt: '2026-08-28T12:10:00Z',
    })
  })

  rpc(PostService.method.savePostContent, async (req) => {
    calls?.push('SavePostContent')
    if (req.content)
      options.contentSaves?.push({
        slug: req.slug,
        expectedRevision: req.expectedRevision,
        content: req.content,
        takenCandidates: [...req.takenCandidates],
      })
    await options.contentSaveGate
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    if (row.contentRevision !== req.expectedRevision)
      throw connectAppError('POST_CONTENT_STALE', Code.Aborted)
    if (!req.content) throw connectAppError('POST_CONTENT_INVALID', Code.InvalidArgument)
    // Like the server: a repeated index, or one outside the stored list, is malformed.
    const taken = new Set(req.takenCandidates)
    if (
      taken.size !== req.takenCandidates.length ||
      req.takenCandidates.some((index) => index < 0 || index >= row.replacementCandidates.length)
    )
      throw connectAppError('POST_CONTENT_INVALID', Code.InvalidArgument)
    // Only a save that changes something is refused: an identical one stays a no-op (R7).
    if (JSON.stringify(row.content) !== JSON.stringify(req.content)) {
      refuseIfPublished(row)
      // A take spends its candidate in the same write (POST-79); the rest keep their order.
      row.replacementCandidates = row.replacementCandidates.filter((_, index) => !taken.has(index))
      row.content = req.content
      row.contentRevision += 1n
      row.status = 'review'
      row.finalizedRevision = 0n
      row.finalizedAt = ''
    }
    return create(SavePostContentResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.savePostGenerationOptions, async (req) => {
    calls?.push('SavePostGenerationOptions')
    const ticks = req.qualityRules?.metrics.map(qualityMetricFromProto)
    options.optionSaves?.push({
      slug: req.slug,
      targetLength: req.targetLength,
      tagCount: req.tagCount,
      useMemory: req.useMemory,
      qualityRules: ticks?.filter((id): id is QualityMetricId => id !== undefined),
      field:
        req.field === undefined ? undefined : (blogFieldFromProto(req.field) ?? `?${req.field}`),
    })
    await options.optionSaveGate
    if (options.optionSaveFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    refuseIfPublished(row)
    // A whole set, like the server: a request missing a member is refused before anything is
    // written (POST-89). Only the length's absence is a value, natural length.
    if (
      req.tagCount === undefined ||
      req.useMemory === undefined ||
      req.qualityRules === undefined ||
      req.field === undefined
    )
      throw connectAppError('POST_CONTENT_INVALID', Code.InvalidArgument)
    // Validated before anything is written: an unknown metric or 분야 changes nothing.
    if (ticks?.some((id) => id === undefined))
      throw connectAppError('POST_QUALITY_RULE_INVALID', Code.InvalidArgument)
    if (blogFieldFromProto(req.field) === undefined)
      throw connectAppError('POST_FIELD_NOT_FOUND', Code.NotFound)
    row.targetLength = req.targetLength
    row.tagCount = req.tagCount
    row.useMemory = req.useMemory
    // Deduplicated; present with none clears the ticks.
    row.qualityRules = [...new Set(req.qualityRules.metrics)]
    row.field = req.field
    return create(SavePostGenerationOptionsResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.finalizePost, (req) => {
    calls?.push('FinalizePost')
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    refuseIfPublished(row)
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
    // Which half comes back is the UPLOAD's kind, never the request's.
    if (upload.video) {
      const video = create(VideoSchema, {
        id: req.uploadId,
        filename: upload.filename,
        width: req.width,
        height: req.height,
        bytes: 12_000_000n,
        durationMs: req.durationMs,
        contentType: 'video/mp4',
      })
      rows.get(upload.slug)?.videos.push(video)
      return create(ConfirmUploadResponseSchema, { video })
    }
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

  // Like the server (T329): an empty address clears a published post back to finalized; only a
  // finalized or published post takes one; the rule is POST-77's, the same parser the pre-check
  // runs; the same address is a no-op, and another replaces it and restamps the publish.
  rpc(PostService.method.savePostPublishedUrl, (req) => {
    calls?.push('SavePostPublishedUrl')
    options.publishedUrlSaves?.push(req.url)
    if (foreign.includes(req.slug)) throw connectAppError('POST_FORBIDDEN', Code.PermissionDenied)
    const row = rows.get(req.slug)
    if (!row) throw connectAppError('POST_NOT_FOUND', Code.NotFound)
    if (options.publishedUrlBusy) throw connectAppError('POST_BUSY', Code.FailedPrecondition)
    if (req.url.trim() === '') {
      if (row.status === 'published') {
        row.status = 'finalized'
        row.publishedUrl = ''
        row.publishedAt = ''
      }
      return create(SavePostPublishedUrlResponseSchema, { post: toProto(row) })
    }
    if (row.status !== 'finalized' && row.status !== 'published')
      throw connectAppError('POST_NOT_FINALIZED', Code.FailedPrecondition)
    const stored = parseNaverBlogUrl(req.url)
    if (stored === undefined)
      throw connectAppError('POST_PUBLISHED_URL_INVALID', Code.InvalidArgument)
    if (row.status === 'published' && row.publishedUrl === stored)
      return create(SavePostPublishedUrlResponseSchema, { post: toProto(row) })
    publishSequence += 1
    row.status = 'published'
    row.publishedUrl = stored
    row.publishedAt = `2026-08-28T13:${String(publishSequence).padStart(2, '0')}:00Z`
    return create(SavePostPublishedUrlResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.deleteVideo, (req) => {
    calls?.push('DeleteVideo')
    if (options.deleteFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    for (const row of rows.values()) {
      const index = row.videos.findIndex((video) => video.id === req.videoId)
      if (index !== -1) {
        refuseIfPublished(row)
        row.videos.splice(index, 1)
        return create(DeleteVideoResponseSchema, {})
      }
    }
    throw connectAppError('POST_NOT_FOUND', Code.NotFound)
  })

  rpc(PostService.method.deleteImage, (req) => {
    calls?.push('DeleteImage')
    if (options.deleteFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    for (const row of rows.values()) {
      const index = row.images.findIndex((image) => image.id === req.imageId)
      if (index !== -1) {
        refuseIfPublished(row)
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
