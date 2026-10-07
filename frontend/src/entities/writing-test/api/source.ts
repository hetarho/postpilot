import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { createConnectQueryKey, useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { PostService, ProtoQualityMetric, requireContentLanguage } from '@/shared/api'
import type { TestQualityRule } from '../model/types'
import type { WritingTestSource, WritingTestSourceSummary } from '../model/source'
import { requirePostRevision, requireText, WritingTestValidationError } from './mappers'

function qualityRule(value: ProtoQualityMetric): TestQualityRule {
  switch (value) {
    case ProtoQualityMetric.TITLE_SATURATION:
      return 'title-saturation'
    case ProtoQualityMetric.CROSS_POST_PHRASES:
      return 'cross-post-phrases'
    case ProtoQualityMetric.IN_POST_REPETITION:
      return 'in-post-repetition'
    case ProtoQualityMetric.COMPOSITION:
      return 'composition'
    default:
      throw new WritingTestValidationError('source quality rule')
  }
}
export function createWritingTestSourceClient(transport: Transport) {
  const client = createClient(PostService, transport)
  return async (slug: string, signal?: AbortSignal): Promise<WritingTestSource> => {
    requireText(slug, 'source')
    const wire = (await client.getPost({ slug }, { signal })).post
    if (
      !wire ||
      wire.slug !== slug ||
      !['draft', 'review', 'finalized', 'published'].includes(wire.status)
    )
      throw new WritingTestValidationError('source identity')
    if (wire.tagCount === undefined || !Number.isInteger(wire.tagCount) || wire.tagCount < 0)
      throw new WritingTestValidationError('source tag count')
    const attachments = [
      ...wire.images.map((image) => ({
        id: requireText(image.id, 'source attachment'),
        name: image.filename,
        kind: 'photo' as const,
      })),
      ...wire.videos.map((video) => ({
        id: requireText(video.id, 'source attachment'),
        name: video.filename,
        kind: 'video' as const,
      })),
    ]
    if (new Set(attachments.map((attachment) => attachment.id)).size !== attachments.length)
      throw new WritingTestValidationError('source attachment identities')
    return {
      name: wire.title,
      status: wire.status as WritingTestSource['status'],
      attachments,
      context: {
        sourcePostSlug: wire.slug,
        expectedInputRevision: requirePostRevision(wire.inputRevision),
        expectedContentRevision: requirePostRevision(wire.contentRevision),
        material: {
          text: wire.memo,
          fictional: false,
          attachmentIds: attachments.map((attachment) => attachment.id),
          templateAnswers: wire.templateAnswers.map((answer) => ({
            label: answer.label,
            text: answer.text,
            enabled: answer.enabled,
          })),
        },
        voiceId: wire.voice?.id ?? '',
        templateId: wire.template?.id ?? '',
        guidelineSlotId: '',
        targetLanguage: requireContentLanguage(wire.targetLanguage),
        targetLength: wire.targetLength ?? 0,
        tagCount: wire.tagCount,
        useMemory: wire.useMemory,
        qualityRules: wire.qualityRules.map(qualityRule),
      },
    }
  }
}
export function useWritingTestSource(ownerId: string, slug: string) {
  const transport = useTransport()
  const load = useMemo(() => createWritingTestSourceClient(transport), [transport])
  return useQuery({
    queryKey: [
      ...createConnectQueryKey({
        schema: PostService.method.getPost,
        input: { slug },
        transport,
        cardinality: 'finite',
      }),
      'writing-test-source',
      ownerId,
    ],
    queryFn: ({ signal }) => load(slug, signal),
    enabled: !!ownerId && !!slug,
    retry: false,
    refetchOnWindowFocus: false,
  })
}
/** Summary-only owned material picker; details load once after an explicit selection. */
export function useWritingTestSources(ownerId: string) {
  const transport = useTransport()
  const client = useMemo(() => createClient(PostService, transport), [transport])
  const query = useQuery({
    queryKey: [
      ...createConnectQueryKey({
        schema: PostService.method.listPosts,
        input: {},
        transport,
        cardinality: 'finite',
      }),
      'writing-test-sources',
      ownerId,
    ],
    queryFn: async ({ signal }): Promise<WritingTestSourceSummary[]> => {
      const wire = await client.listPosts({}, { signal })
      const sources = wire.posts.map((post) => {
        requireText(post.slug, 'source identity')
        if (!['draft', 'review', 'finalized', 'published'].includes(post.status))
          throw new WritingTestValidationError('source status')
        return {
          slug: post.slug,
          name: post.title,
          updatedAt: post.updatedAt,
          status: post.status as WritingTestSourceSummary['status'],
        }
      })
      if (new Set(sources.map((source) => source.slug)).size !== sources.length)
        throw new WritingTestValidationError('source identities')
      return sources
    },
    enabled: !!ownerId,
    retry: false,
    refetchOnWindowFocus: false,
  })
  return { ...query, sources: query.data ?? [] }
}
