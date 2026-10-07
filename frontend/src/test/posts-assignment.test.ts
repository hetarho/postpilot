import { createClient, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { appFailureFromConnect, PostContentSchema, PostService } from '@/shared/api'
import { POST_CONTENT_FIXTURE } from './fixtures/postContent'
import { registerPostService, type FakePostRow } from './posts'

function client(
  post: Partial<FakePostRow> = {},
  deletedTarget = false,
  draftMaterials?: Array<{ slug: string; title: string; memo: string }>,
) {
  return createClient(
    PostService,
    createRouterTransport((router) =>
      registerPostService(router, {
        draftMaterials,
        posts: [
          {
            slug: 'post',
            title: 'Retained title',
            memo: 'Retained notes',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 3n,
            machineBaselineRevision: 2n,
            pendingExperimentId: 'retained-result',
            ...post,
          },
        ],
        voices: [
          { id: 'voice-default', name: 'Default' },
          { id: 'next', name: 'Next', deleted: deletedTarget },
        ],
      }),
    ),
  )
}

const draft = {
  slug: 'post',
  title: 'Retained title',
  memo: 'Retained notes',
  voiceId: 'next',
}

it('records the exact request material independently of assignment-presence assertions', async () => {
  const materials: Array<{ slug: string; title: string; memo: string }> = []
  const api = client({}, false, materials)
  await api.savePostDraft({ ...draft, title: 'Newest title', memo: 'Newest notes' })
  expect(materials).toEqual([{ slug: 'post', title: 'Newest title', memo: 'Newest notes' }])
})

it.each([
  undefined,
  { id: 'frozen-model', kind: 'model_experiment', status: 'queued' },
  { id: 'running-model', kind: 'model_experiment', status: 'running' },
  { id: 'frozen-common', kind: 'writing_test', status: 'queued' },
  { id: 'running-common', kind: 'writing_test', status: 'running' },
])(
  'models future voice assignment independently of retained/frozen tests (%j)',
  async (activeJob) => {
    const api = client({ activeJob })
    const before = (await api.getPost({ slug: 'post' })).post!
    expect(before.activeJob).toBeUndefined()
    const after = (await api.savePostDraft(draft)).post!
    expect(after.voice?.id).toBe('next')
    expect(after.content).toEqual(before.content)
    expect(after.contentRevision).toBe(3n)
    expect(after.machineBaselineRevision).toBe(2n)
    expect(after.status).toBe('review')
    expect(after.pendingExperimentId).toBe('retained-result')
    expect(after.activeJob).toBeUndefined()
    expect((await api.listPosts({})).posts[0]?.activeJob).toBeUndefined()
  },
)

it.each(['done', 'failed', 'cancelled'])(
  'models ordinary %s jobs as terminal for future assignments',
  async (status) => {
    const api = client({ activeJob: { id: 'ordinary', kind: 'generate', status } })
    expect((await api.savePostDraft(draft)).post?.voice?.id).toBe('next')
  },
)

it.each(['queued', 'running'])(
  'still refuses voice reassignment during ordinary %s writing',
  async (status) => {
    const api = client({ activeJob: { id: 'ordinary', kind: 'generate', status } })
    const reason = await api
      .savePostDraft(draft)
      .catch((cause: unknown) => appFailureFromConnect(cause).reason)
    expect(reason).toBe('POST_BUSY')
    expect((await api.getPost({ slug: 'post' })).post?.voice?.id).toBe('voice-default')
  },
)

it('preserves published and deleted-target refusals alongside independent tests', async () => {
  const published = client({ status: 'published' })
  expect(
    await published
      .savePostDraft(draft)
      .catch((cause: unknown) => appFailureFromConnect(cause).reason),
  ).toBe('POST_PUBLISHED_LOCKED')
  const deleted = client({}, true)
  expect(
    await deleted
      .savePostDraft(draft)
      .catch((cause: unknown) => appFailureFromConnect(cause).reason),
  ).toBe('VOICE_DELETED')
})

it.each(['draft', 'review', 'finalized', 'published'])(
  'bases %s history export availability on canonical blocks rather than status',
  async (status) => {
    const empty = client({ status, content: create(PostContentSchema, { title: 'No blocks' }) })
    expect((await empty.listPosts({})).posts[0]).toMatchObject({
      contentReady: false,
      exportReady: false,
    })
    const complete = client({ status })
    expect((await complete.listPosts({})).posts[0]).toMatchObject({
      contentReady: true,
      exportReady: true,
    })
  },
)
