import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'
import { transport } from '@/shared/api'
import {
  invalidatePostsDependingOn,
  invalidateWrittenPost,
  type PostDependency,
} from './post-dependencies'
import { getPostQueryKey, listPostsQueryKey, postDetailQueriesKey } from './post-queries'

/** The one statement of "a write to another noun staled the posts" (ARCH-14). Pinned here rather
 *  than in each verb: the point of the entry is that a NEW voice or template verb inherits the
 *  rule instead of remembering it. */
function invalidatedKeys(run: (queryClient: QueryClient) => void) {
  const queryClient = new QueryClient()
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue()
  run(queryClient)
  return invalidate.mock.calls.map(([options]) => options?.queryKey)
}

describe('invalidatePostsDependingOn', () => {
  it.each<[string, PostDependency]>([
    ['a voice write', { voiceId: 'voice-1' }],
    ['a template write', { templateId: 'template-1' }],
    ['a guideline write', { guidelineId: 'guideline-1' }],
  ])('stales the list and every detail after %s', (_name, dependency) => {
    const keys = invalidatedKeys((queryClient) =>
      invalidatePostsDependingOn(queryClient, transport, dependency),
    )

    expect(keys).toContainEqual(listPostsQueryKey(transport))
    expect(keys).toContainEqual(postDetailQueriesKey(transport))
    expect(keys).toHaveLength(2)
  })

  it('does nothing when no dependency is named', () => {
    expect(invalidatedKeys((q) => invalidatePostsDependingOn(q, transport, {}))).toEqual([])
  })
})

describe('invalidateWrittenPost', () => {
  it('stales the written post and the list, and no other post', () => {
    const keys = invalidatedKeys((queryClient) => {
      void invalidateWrittenPost(queryClient, transport, 'post-1')
    })

    expect(keys).toContainEqual(getPostQueryKey(transport, 'post-1'))
    expect(keys).toContainEqual(listPostsQueryKey(transport))
    expect(keys).not.toContainEqual(postDetailQueriesKey(transport))
  })

  it('stales only the list when the experiment named no post', () => {
    const keys = invalidatedKeys((queryClient) => {
      void invalidateWrittenPost(queryClient, transport, '')
    })

    expect(keys).toEqual([listPostsQueryKey(transport)])
  })
})
