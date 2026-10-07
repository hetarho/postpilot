import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { expect, it } from 'vitest'
import { PostService, contentLanguageToProto, ProtoQualityMetric } from '@/shared/api'
import { createWritingTestSourceClient } from './source'

it('loads only a real owned post and preserves source bigint revisions and frozen language', async () => {
  let reads = 0
  const client = createWritingTestSourceClient(
    createRouterTransport(({ rpc }) => {
      rpc(PostService.method.getPost, (request) => {
        reads++
        expect(request.slug).toBe('owned')
        return create(PostService.method.getPost.output, {
          post: {
            slug: 'owned',
            title: 'Source title',
            memo: 'Material',
            status: 'draft',
            inputRevision: 9007199254740993n,
            contentRevision: 5n,
            targetLanguage: contentLanguageToProto('en'),
            tagCount: 4,
            targetLength: 1700,
            voice: { id: 'voice' },
            template: { id: 'template' },
            useMemory: true,
            qualityRules: [ProtoQualityMetric.COMPOSITION],
            images: [{ id: 'photo', filename: 'actual.jpg' }],
            videos: [{ id: 'video', filename: 'actual.mp4' }],
            templateAnswers: [{ label: 'Place', text: 'Actual place', enabled: true }],
          },
        })
      })
    }),
  )
  const source = await client('owned')
  expect(reads).toBe(1)
  expect(source.context).toMatchObject({
    sourcePostSlug: 'owned',
    expectedInputRevision: 9007199254740993n,
    expectedContentRevision: 5n,
    targetLanguage: 'en',
    tagCount: 4,
    voiceId: 'voice',
    templateId: 'template',
    qualityRules: ['composition'],
    useMemory: true,
    material: { fictional: false, text: 'Material', attachmentIds: ['photo', 'video'] },
  })
  expect(source.context.writeModel).toBeUndefined()
  expect(source.attachments).toEqual([
    { id: 'photo', name: 'actual.jpg', kind: 'photo' },
    { id: 'video', name: 'actual.mp4', kind: 'video' },
  ])
})
it('refuses mismatched source identities and missing target provenance rather than defaulting', async () => {
  let targetLanguage = 0
  const client = createWritingTestSourceClient(
    createRouterTransport(({ rpc }) => {
      rpc(PostService.method.getPost, () =>
        create(PostService.method.getPost.output, {
          post: { slug: 'owned', status: 'draft', tagCount: 3, targetLanguage },
        }),
      )
    }),
  )
  await expect(client('foreign')).rejects.toThrow('source identity')
  await expect(client('owned')).rejects.toThrow('unsupported content language')
  targetLanguage = contentLanguageToProto('ko')
  expect((await client('owned')).context.targetLanguage).toBe('ko')
})
