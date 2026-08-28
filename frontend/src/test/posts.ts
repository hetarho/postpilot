// Shared fake PostService for tests.
//
// It models the three server rules the frontend actually depends on: an empty slug mints
// one (`YYYYMMDD-title`, serial suffix on collision), someone else's slug is 403 and a
// missing one is 404 (spec/policy/posts.md). Everything else is kept as thin as possible.
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  GetPostResponseSchema,
  ListPostsResponseSchema,
  PostSchema,
  PostService,
  PostSummarySchema,
  SavePostDraftResponseSchema,
} from '@/shared/api'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakePostRow {
  slug: string
  title?: string
  memo?: string
  status?: string
  updatedAt?: string
}

export interface FakePostsOptions {
  /** The acting user's posts, in the order ListPosts should return them. */
  posts?: FakePostRow[]
  /** Slugs that exist but belong to someone else — 403, the way the server answers. */
  foreign?: string[]
  /** Make ListPosts fail. */
  listFails?: boolean
  /** Fail this many SavePostDraft calls before the first success. */
  failSaves?: number
  /** Answer SavePostDraft 200 with no post — a confirmation the client must not trust. */
  saveReturnsNoPost?: boolean
  /** The date the fake mints slugs from. */
  today?: string
  /** Records every procedure the transport was asked for. */
  calls?: string[]
}

const DEFAULT_UPDATED_AT = '2026-08-28T12:00:00Z'

export function registerPostService(router: ConnectRouter, options: FakePostsOptions = {}) {
  const { rpc } = router
  const { foreign = [], listFails, today = '20260828', calls } = options
  let failuresLeft = options.failSaves ?? 0

  const rows = new Map<string, Required<FakePostRow>>(
    (options.posts ?? []).map((row) => [
      row.slug,
      {
        slug: row.slug,
        title: row.title ?? '',
        memo: row.memo ?? '',
        status: row.status ?? 'draft',
        updatedAt: row.updatedAt ?? DEFAULT_UPDATED_AT,
      },
    ]),
  )

  function mintSlug(title: string): string {
    const base = title.trim().toLowerCase().replace(/\s+/g, '-') || 'untitled'
    let slug = `${today}-${base}`
    for (let serial = 2; rows.has(slug); serial += 1) slug = `${today}-${base}-${serial}`
    return slug
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
    const row = rows.get(req.slug)
    if (!row) throw new ConnectError('not found', Code.NotFound)
    return create(GetPostResponseSchema, { post: create(PostSchema, row) })
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
    const row = {
      slug,
      title: req.title,
      memo: req.memo,
      status: rows.get(slug)?.status ?? 'draft',
      updatedAt: DEFAULT_UPDATED_AT,
    }
    rows.set(slug, row)
    return create(SavePostDraftResponseSchema, { post: create(PostSchema, row) })
  })
}

/** A transport serving only PostService — for tests of the post hooks themselves. */
export function createFakePostsTransport(options: FakePostsOptions = {}) {
  return createRouterTransport((router) => registerPostService(router, options))
}
