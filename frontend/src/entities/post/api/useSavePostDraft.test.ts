import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  GenerationJobSchema,
  GetPostResponseSchema,
  ImageSchema,
  ObservationSchema,
  PostContentSchema,
  PostSchema,
  VoiceRefSchema,
} from '@/shared/api'
import { applyingSavedDraft } from './useSavePostDraft'

describe('applying a draft save response', () => {
  it('updates its text without rolling server-owned generation fields back', () => {
    const cached = create(GetPostResponseSchema, {
      post: create(PostSchema, {
        slug: 'post',
        title: 'old title',
        memo: 'old memo',
        status: 'review',
        updatedAt: '2026-08-29T01:00:02Z',
        images: [create(ImageSchema, { id: 'image', filename: 'IMG_1.jpg' })],
        observations: [
          create(ObservationSchema, { file: 'IMG_1.jpg', scene: 'completed observation' }),
        ],
        content: create(PostContentSchema, { title: 'completed draft' }),
        activeJob: create(GenerationJobSchema, { id: 'job-new', status: 'running' }),
      }),
    })
    // This whole-post response was read before generation advanced, then arrived late.
    const staleSave = create(PostSchema, {
      slug: 'post',
      title: 'latest title',
      memo: 'latest memo',
      status: 'draft',
      updatedAt: '2026-08-29T01:00:01Z',
    })

    const applied = applyingSavedDraft(staleSave, cached)

    expect(applied.title).toBe('latest title')
    expect(applied.memo).toBe('latest memo')
    expect(applied.status).toBe('review')
    expect(applied.updatedAt).toBe('2026-08-29T01:00:02Z')
    expect(applied.images[0]?.filename).toBe('IMG_1.jpg')
    expect(applied.observations[0]?.scene).toBe('completed observation')
    expect(applied.content?.title).toBe('completed draft')
    expect(applied.activeJob?.id).toBe('job-new')
  })

  it('takes the voice and the cleared baseline only when the save reassigned the post', () => {
    const cached = create(GetPostResponseSchema, {
      post: create(PostSchema, {
        slug: 'post',
        voice: create(VoiceRefSchema, { id: 'voice-a', name: '일기' }),
        machineBaselineRevision: 3n,
        machineBaselineVoiceId: 'voice-a',
        canFinalize: true,
      }),
    })
    const renamedOnly = create(PostSchema, {
      slug: 'post',
      voice: create(VoiceRefSchema, { id: 'voice-a', name: '일기장' }),
      machineBaselineRevision: 0n,
      canFinalize: false,
    })
    // Same voice: the name is refreshed, but the baseline stays the cache's — this response may
    // predate a generation that just established it.
    const same = applyingSavedDraft(renamedOnly, cached)
    expect(same.voice?.name).toBe('일기장')
    expect(same.machineBaselineRevision).toBe(3n)
    expect(same.canFinalize).toBe(true)

    const reassigned = create(PostSchema, {
      slug: 'post',
      voice: create(VoiceRefSchema, { id: 'voice-b', name: '리뷰' }),
      machineBaselineRevision: 0n,
      machineBaselineVoiceId: '',
      canFinalize: false,
    })
    const moved = applyingSavedDraft(reassigned, cached)
    expect(moved.voice?.id).toBe('voice-b')
    expect(moved.machineBaselineRevision).toBe(0n)
    expect(moved.machineBaselineVoiceId).toBe('')
    expect(moved.canFinalize).toBe(false)
  })

  it('uses the full response when a newly minted post has no cache entry yet', () => {
    const created = create(PostSchema, {
      slug: 'new-post',
      title: 'first title',
      memo: 'first memo',
    })

    expect(applyingSavedDraft(created, undefined)).toBe(created)
  })
})
