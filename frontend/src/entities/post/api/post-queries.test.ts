import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  contentLanguageToProto,
  PostSchema,
  ProtoQualityMetric,
  ProtoReplacementSurface,
  ReplacementCandidateSchema,
  VoiceRefSchema,
  ProtoBlogField,
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

  // GEN-53: a dropped offer changes nothing, so a surface this build cannot name is dropped.
  it('maps replacement candidates and drops one with an unknown surface', () => {
    const draft = toPostDraft(
      create(PostSchema, {
        slug: 'post',
        voice,
        targetLanguage: contentLanguageToProto('ko'),
        replacementCandidates: [
          create(ReplacementCandidateSchema, {
            surface: ProtoReplacementSurface.TAG,
            index: 1,
            source: '산책',
            phrases: ['산책로', '여행'],
          }),
          create(ReplacementCandidateSchema, {
            surface: 9_999 as ProtoReplacementSurface,
            source: '제주',
            phrases: ['제주도'],
          }),
          create(ReplacementCandidateSchema, {
            surface: ProtoReplacementSurface.BODY,
            index: 0,
            source: '기다렸다',
            phrases: ['기다린다'],
          }),
        ],
      }),
    )
    // Each keeps its index in the server's list, which is what a take sends (POST-79).
    expect(draft.replacementCandidates).toEqual([
      { surface: 'tag', index: 1, source: '산책', phrases: ['산책로', '여행'], listIndex: 0 },
      { surface: 'body', index: 0, source: '기다렸다', phrases: ['기다린다'], listIndex: 2 },
    ])
  })

  it('reads both as empty for a post that is not published', () => {
    const draft = toPostDraft(
      create(PostSchema, { slug: 'post', voice, targetLanguage: contentLanguageToProto('ko') }),
    )
    expect(draft.publishedUrl).toBe('')
    expect(draft.publishedAt).toBe('')
  })
})
