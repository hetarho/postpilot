import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  contentLanguageToProto,
  PostSchema,
  ProtoQualityMetric,
  VoiceRefSchema,
} from '@/shared/api'
import { toPostDraft } from './post-queries'

const voice = create(VoiceRefSchema, {
  id: 'voice-a',
  name: '일상 말투',
  sourceLanguage: contentLanguageToProto('ko'),
})

describe('toPostDraft', () => {
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

  // POST-81, ARCH-3: a metric a newer server adds names no row this build can show.
  it('maps the saved ticks and drops a metric this build does not know', () => {
    const draft = toPostDraft(
      create(PostSchema, {
        slug: 'post',
        voice,
        targetLanguage: contentLanguageToProto('ko'),
        qualityRules: [
          ProtoQualityMetric.TITLE_SATURATION,
          9_999 as ProtoQualityMetric,
          ProtoQualityMetric.COMPOSITION,
          ProtoQualityMetric.UNSPECIFIED,
        ],
      }),
    )
    expect(draft.qualityRules).toEqual(['title_saturation', 'composition'])
  })

  it('reads both as empty for a post that is not published', () => {
    const draft = toPostDraft(
      create(PostSchema, { slug: 'post', voice, targetLanguage: contentLanguageToProto('ko') }),
    )
    expect(draft.publishedUrl).toBe('')
    expect(draft.publishedAt).toBe('')
  })
})
