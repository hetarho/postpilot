// Query keys and the proto→domain mappers for the post entity.
import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { toPostImage } from '@/entities/image/@x/post'
import { toGenerationJob } from '@/entities/generation-job/@x/post'
import { toPurposeRef } from '@/entities/purpose/@x/post'
import { toVoiceRef } from '@/entities/voice/@x/post'
import { PostService, type Post, type PostSummary } from '@/shared/api'
import type { PostDraft, PostListItem } from '../model/types'

export function toPostDraft(post: Post): PostDraft {
  return {
    slug: post.slug,
    title: post.title,
    memo: post.memo,
    status: post.status as PostDraft['status'],
    createdAt: post.createdAt,
    updatedAt: post.updatedAt,
    voice: toVoiceRef(post.voice),
    purpose: toPurposeRef(post.purpose),
    images: post.images.map(toPostImage),
    activeJob: post.activeJob ? toGenerationJob(post.activeJob) : undefined,
    content: post.content,
    observations: post.observations,
    pendingExperimentId: post.pendingExperimentId,
    contentRevision: post.contentRevision,
    machineBaselineRevision: post.machineBaselineRevision,
    machineBaselineVoiceId: post.machineBaselineVoiceId,
    canFinalize: post.canFinalize,
    targetLength: post.targetLength,
    finalizedRevision: post.finalizedRevision,
    finalizedAt: post.finalizedAt,
  }
}

export function toPostListItem(summary: PostSummary): PostListItem {
  return {
    slug: summary.slug,
    title: summary.title,
    status: summary.status as PostListItem['status'],
    updatedAt: summary.updatedAt,
    voice: toVoiceRef(summary.voice),
    purpose: toPurposeRef(summary.purpose),
    activeJob: summary.activeJob ? toGenerationJob(summary.activeJob) : undefined,
    pendingExperimentId: summary.pendingExperimentId,
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

/** Matches every cached GetPost, whatever its slug — for a change that touches all posts at once,
 *  such as renaming or restoring a voice. connect-query keys match partially when `input` is
 *  omitted, so this is a prefix of every `getPostQueryKey`. */
export function postDetailQueriesKey(transport: Transport) {
  return createConnectQueryKey({
    schema: PostService.method.getPost,
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
