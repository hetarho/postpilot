import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  contentLanguageToProto,
  PostSchema,
  PostSummarySchema,
  GenerationJobSchema,
  ProtoQualityMetric,
  VoiceRefSchema,
  ProtoBlogField,
  OriginReviewSchema,
  OriginFieldKind,
  OriginReviewState,
  SemanticOriginCategory,
} from '@/shared/api'
import { toPostDraft, toPostListItem } from './post-queries'

describe('toPostListItem', () => {
  it('projects canonical readiness, published address, revisions and the ordinary failure snapshot', () => {
    const row = toPostListItem(
      create(PostSummarySchema, {
        slug: 'post',
        status: 'review',
        targetLanguage: contentLanguageToProto('ko'),
        contentReady: true,
        exportReady: true,
        publishedUrl: 'https://blog.naver.com/alice/1',
        inputRevision: 8n,
        contentRevision: 3n,
        latestOrdinaryFailure: create(GenerationJobSchema, {
          id: 'ordinary-failure',
          kind: 'generate_post',
          status: 'failed',
          stage: 'write',
        }),
      }),
    )
    expect(row.contentReady).toBe(true)
    expect(row.exportReady).toBe(true)
    expect(row.publishedUrl).toBe('https://blog.naver.com/alice/1')
    expect(row.inputRevision).toBe(8n)
    expect(row.contentRevision).toBe(3n)
    expect(row.latestOrdinaryFailure).toMatchObject({
      id: 'ordinary-failure',
      status: 'failed',
      stage: 'write',
    })
  })

  it('does not infer canonical content or export readiness from a finalized status', () => {
    const row = toPostListItem(
      create(PostSummarySchema, {
        slug: 'empty-finalized',
        status: 'finalized',
        targetLanguage: contentLanguageToProto('ko'),
      }),
    )
    expect(row.contentReady).toBe(false)
    expect(row.exportReady).toBe(false)
    expect(row.latestOrdinaryFailure).toBeUndefined()
  })
})

const voice = create(VoiceRefSchema, {
  id: 'voice-a',
  name: '일상 말투',
})

describe('toPostDraft', () => {
  it('maps current authoritative origin identity and discards a stale review without changing prose', () => {
    const wire = create(PostSchema, {
      slug: 'current-origin',
      targetLanguage: contentLanguageToProto('ko'),
      contentRevision: 7n,
      contentHash: 'server-current-hash',
      content: { title: '가😀끝', summary: '', tags: [], blocks: [] },
      contentOrigins: create(OriginReviewSchema, {
        version: 1,
        result: { contentRevision: 7n, contentHash: 'server-current-hash' },
        spans: [
          {
            field: { kind: OriginFieldKind.TITLE },
            start: 1,
            end: 2,
            quote: '😀',
            category: SemanticOriginCategory.AI_ADDED,
            reviewState: OriginReviewState.CONFIRMED,
          },
        ],
      }),
    })
    const current = toPostDraft(wire)
    expect(current.contentRevision).toBe(7n)
    expect(current.contentHash).toBe('server-current-hash')
    expect(current.contentOrigins?.result).toEqual({
      contentRevision: 7n,
      contentHash: 'server-current-hash',
    })
    expect(current.contentOrigins?.spans[0]).toMatchObject({
      quote: '😀',
      start: 1,
      end: 2,
      category: 'ai_added',
      reviewState: 'confirmed',
    })
    wire.contentOrigins!.result!.contentRevision = 6n
    const stale = toPostDraft(wire)
    expect(stale.contentOrigins).toBeUndefined()
    expect(stale.contentRevision).toBe(7n)
    expect(stale.contentHash).toBe('server-current-hash')
    expect(stale.content?.title).toBe('가😀끝')
    wire.contentOrigins = undefined
    expect(toPostDraft(wire).contentOrigins).toBeUndefined()
  })

  it('carries where and when the post was published', () => {
    const draft = toPostDraft(
      create(PostSchema, {
        slug: 'post',
        status: 'published',
        voice,
        targetLanguage: contentLanguageToProto('ko'),
        publishedUrl: 'https://blog.naver.com/alice/1',
        publishedAt: '2026-08-21T09:00:00Z',
      }),
    )
    expect(draft.status).toBe('published')
    expect(draft.publishedUrl).toBe('https://blog.naver.com/alice/1')
    expect(draft.publishedAt).toBe('2026-08-21T09:00:00Z')
  })

  it('maps the saved ticks', () => {
    const draft = toPostDraft(
      create(PostSchema, {
        slug: 'post',
        voice,
        targetLanguage: contentLanguageToProto('ko'),
        qualityRules: [ProtoQualityMetric.TITLE_SATURATION, ProtoQualityMetric.COMPOSITION],
      }),
    )
    expect(draft.qualityRules).toEqual(['title_saturation', 'composition'])
  })

  // POST-81, ARCH-3: every 저장 resends the whole tick set (POST-89), so a metric a newer server
  // adds fails the read rather than being dropped and then erased by the next save.
  it.each([
    ['a number this build does not know', 9_999 as ProtoQualityMetric],
    ['UNSPECIFIED', ProtoQualityMetric.UNSPECIFIED],
  ])('refuses a quality tick this build does not know: %s', (_name, metric) => {
    expect(() =>
      toPostDraft(
        create(PostSchema, {
          slug: 'post',
          voice,
          targetLanguage: contentLanguageToProto('ko'),
          qualityRules: [ProtoQualityMetric.TITLE_SATURATION, metric],
        }),
      ),
    ).toThrow('unsupported quality metric enum')
  })

  // ARCH-3, POST-89: the brief's form is seeded from the post's 분야 and every 저장 sends it, so a
  // 분야 read as 없음 would be erased on the server by the next save of any run option.
  it('refuses a 분야 this build does not know', () => {
    expect(() =>
      toPostDraft(
        create(PostSchema, {
          slug: 'post',
          voice,
          targetLanguage: contentLanguageToProto('ko'),
          field: 9_999 as ProtoBlogField,
        }),
      ),
    ).toThrow('unsupported blog field enum')
    expect(
      toPostDraft(
        create(PostSchema, {
          slug: 'post',
          voice,
          targetLanguage: contentLanguageToProto('ko'),
          field: ProtoBlogField.UNSPECIFIED,
        }),
      ).field,
    ).toBe('')
  })

  it('reads both as empty for a post that is not published', () => {
    const draft = toPostDraft(
      create(PostSchema, { slug: 'post', voice, targetLanguage: contentLanguageToProto('ko') }),
    )
    expect(draft.publishedUrl).toBe('')
    expect(draft.publishedAt).toBe('')
  })
})
