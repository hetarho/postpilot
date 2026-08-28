// Query keys and the proto→domain mappers for the post entity.
import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { PostService, type Post, type PostSummary } from '@/shared/api'
import type { PostDraft, PostListItem } from '../model/types'

export function toPostDraft(post: Post): PostDraft {
  return {
    slug: post.slug,
    title: post.title,
    memo: post.memo,
    status: post.status,
    updatedAt: post.updatedAt,
  }
}

export function toPostListItem(summary: PostSummary): PostListItem {
  return {
    slug: summary.slug,
    title: summary.title,
    status: summary.status,
    updatedAt: summary.updatedAt,
  }
}

/** The exact key `useQuery(getPost, { slug })` registers under.
 *
 *  All four properties are load-bearing — see entities/session for the full reason. The
 *  save mutation writes this entry so the editor mounted by the mint navigation renders
 *  from cache; a key that does not match to the byte would silently write a second
 *  entry and the editor would come up empty. */
export function getPostQueryKey(transport: Transport, slug: string) {
  return createConnectQueryKey({
    schema: PostService.method.getPost,
    input: { slug },
    transport,
    cardinality: 'finite',
  })
}

export function listPostsQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: PostService.method.listPosts,
    input: {},
    transport,
    cardinality: 'finite',
  })
}
