// Shared fake PostService for tests.
//
// It models the server rules the frontend actually depends on: an empty slug mints one
// (`YYYYMMDD-title`, serial suffix on collision), someone else's slug is 403 and a missing
// one is 404 (spec/policy/posts.md); an upload is a CreateUpload → ConfirmUpload pair
// and a confirmed filename is taken (spec/policy/uploads.md). Everything else is kept as
// thin as possible.
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  ConfirmUploadResponseSchema,
  CreateUploadResponseSchema,
  DeleteImageResponseSchema,
  GetPostResponseSchema,
  type Image,
  ImageSchema,
  ListPostsResponseSchema,
  PostSchema,
  PostService,
  PostSummarySchema,
  type ProtoGenerationJob,
  type Observation,
  type PostContent,
  SavePostDraftResponseSchema,
} from '@/shared/api'
import { type FakeGenerationJobRow, toFakeProto } from './jobs'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeImageRow {
  id: string
  filename: string
  width?: number
  height?: number
  viewUrl?: string
}

export interface FakePostRow {
  slug: string
  title?: string
  memo?: string
  status?: string
  updatedAt?: string
  images?: FakeImageRow[]
  activeJob?: FakeGenerationJobRow
  content?: PostContent
  observations?: Observation[]
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
}

const DEFAULT_UPDATED_AT = '2026-08-28T12:00:00Z'

/** The storage host presigned URLs point at — anything but the API's own origin. */
export const FAKE_STORAGE_ORIGIN = 'https://storage.test'

type Row = {
  slug: string
  title: string
  memo: string
  status: string
  updatedAt: string
  images: Image[]
  activeJob?: ProtoGenerationJob
  content?: PostContent
  observations: Observation[]
}

export function registerPostService(router: ConnectRouter, options: FakePostsOptions = {}) {
  const { rpc } = router
  const { foreign = [], listFails, today = '20260828', calls } = options
  let failuresLeft = options.failSaves ?? 0
  let uploadSequence = 0
  let getSequenceIndex = 0
  const pending = new Map<string, { slug: string; filename: string }>()

  function toRow(row: FakePostRow): Row {
    return {
      slug: row.slug,
      title: row.title ?? '',
      memo: row.memo ?? '',
      status: row.status ?? 'draft',
      updatedAt: row.updatedAt ?? DEFAULT_UPDATED_AT,
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
    if (listFails) throw new ConnectError('unavailable', Code.Unavailable)
    return create(ListPostsResponseSchema, {
      posts: [...rows.values()].map((row) => create(PostSummarySchema, row)),
    })
  })

  rpc(PostService.method.getPost, (req) => {
    calls?.push('GetPost')
    if (foreign.includes(req.slug)) throw new ConnectError('not yours', Code.PermissionDenied)
    const sequenced =
      options.getSequence?.[Math.min(getSequenceIndex, options.getSequence.length - 1)]
    if (sequenced?.slug === req.slug) {
      getSequenceIndex += 1
      rows.set(req.slug, toRow(sequenced))
    }
    const row = rows.get(req.slug)
    if (!row) throw new ConnectError('not found', Code.NotFound)
    return create(GetPostResponseSchema, { post: withViewUrls(row) })
  })

  rpc(PostService.method.savePostDraft, (req) => {
    calls?.push('SavePostDraft')
    if (failuresLeft > 0) {
      failuresLeft -= 1
      throw new ConnectError('unavailable', Code.Unavailable)
    }
    if (options.saveReturnsNoPost) return create(SavePostDraftResponseSchema, {})
    if (req.slug && foreign.includes(req.slug)) {
      throw new ConnectError('not yours', Code.PermissionDenied)
    }
    const slug = req.slug || mintSlug(req.title)
    const row: Row = {
      slug,
      title: req.title,
      memo: req.memo,
      status: rows.get(slug)?.status ?? 'draft',
      updatedAt: DEFAULT_UPDATED_AT,
      images: rows.get(slug)?.images ?? [],
      activeJob: rows.get(slug)?.activeJob,
      content: rows.get(slug)?.content,
      observations: rows.get(slug)?.observations ?? [],
    }
    rows.set(slug, row)
    return create(SavePostDraftResponseSchema, { post: toProto(row) })
  })

  rpc(PostService.method.createUpload, (req) => {
    calls?.push('CreateUpload')
    const row = rows.get(req.postSlug)
    if (!row) throw new ConnectError('not found', Code.NotFound)
    if (row.images.some((image) => image.filename === req.filename)) {
      throw new ConnectError('taken', Code.AlreadyExists)
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

  rpc(PostService.method.confirmUpload, (req) => {
    calls?.push('ConfirmUpload')
    const upload = pending.get(req.uploadId)
    if (!upload) throw new ConnectError('not found', Code.NotFound)
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
    if (options.deleteFails) throw new ConnectError('unavailable', Code.Unavailable)
    for (const row of rows.values()) {
      const index = row.images.findIndex((image) => image.id === req.imageId)
      if (index !== -1) {
        row.images.splice(index, 1)
        return create(DeleteImageResponseSchema, {})
      }
    }
    throw new ConnectError('not found', Code.NotFound)
  })
}

/** A transport serving only PostService — for tests of the post hooks themselves. */
export function createFakePostsTransport(options: FakePostsOptions = {}) {
  return createRouterTransport((router) => registerPostService(router, options))
}
