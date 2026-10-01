import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ExperimentOrigin, ObservationSchema, Stage } from '@/shared/api'
import type { FakeWriteExperimentStart, FakeExperimentsOptions } from '@/test/experiments'
import { chooseOption } from '@/test/listbox'
import { renderAppAt } from '@/test/app'
import { FAKE_PUBLISHED_URL } from '@/test/posts'

const writeModels = [
  { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
  { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
]

const writePair = {
  stage: Stage.WRITE,
  candidateA: { providerId: 'openrouter', modelId: 'writer-a' },
  candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
}

it('starts a no-photo write comparison from the model tab with the persisted target length', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  const { router } = renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [{ slug: 'post-1', title: '첫 글', targetLength: 1_600 }],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { starts, experimentId: 'write-experiment-1' },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  expect(starts).toHaveLength(0)
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '첫 글')

  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  await waitFor(() =>
    expect(starts).toEqual([
      {
        postSlug: 'post-1',
        // Started in the lab, so its verdict will be a pick that applies nothing.
        origin: ExperimentOrigin.LAB,
        observeModel: undefined,
        modelA: { providerId: 'openrouter', modelId: 'writer-a' },
        modelB: { providerId: 'openrouter', modelId: 'writer-b' },
        targetLength: 1_600,
      },
    ]),
  )
  await waitFor(() =>
    expect(router.state.location.pathname).toBe('/ai-models/experiments/write-experiment-1'),
  )
  expect(router.state.location.search.from).toBe('compare')
  expect(await screen.findByRole('link', { name: '← 모델 비교로 돌아가기' })).toHaveAttribute(
    'href',
    '/ai-models/compare?stage=write',
  )
})

it('starts one write comparison with all five persisted lab candidates in order', async () => {
  const user = userEvent.setup()
  const candidateStarts: NonNullable<FakeExperimentsOptions['candidateStarts']> = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: { posts: [{ slug: 'post-1', title: '첫 글' }] },
    providers: {
      models: [
        ...writeModels,
        ...['c', 'd', 'e'].map((id) => ({
          providerId: 'openrouter',
          modelId: `writer-${id}`,
          label: `Writer ${id.toUpperCase()}`,
        })),
      ],
      comparisonPairs: [
        {
          ...writePair,
          extraCandidates: ['c', 'd', 'e'].map((id) => ({
            providerId: 'openrouter',
            modelId: `writer-${id}`,
          })),
        },
      ],
    },
    experiments: { candidateStarts },
  })
  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '첫 글')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)
  await waitFor(() => expect(candidateStarts).toHaveLength(1))
  expect(candidateStarts[0]).toEqual({
    kind: 'write',
    refs: ['a', 'b', 'c', 'd', 'e'].map((id) => ({
      providerId: 'openrouter',
      modelId: `writer-${id}`,
    })),
  })
})

it('blocks a changed or saving C row until the server confirms the visible list', async () => {
  const user = userEvent.setup()
  let release!: () => void
  const saveExtrasGate = new Promise<void>((resolve) => {
    release = resolve
  })
  const candidateStarts: NonNullable<FakeExperimentsOptions['candidateStarts']> = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: { posts: [{ slug: 'post-1', title: '첫 글' }] },
    providers: {
      models: [
        ...writeModels,
        { providerId: 'openrouter', modelId: 'writer-c', label: 'Writer C' },
      ],
      comparisonPairs: [writePair],
      saveExtrasGate,
    },
    experiments: { candidateStarts },
  })
  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '첫 글')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(screen.getByRole('button', { name: '후보 추가' }))
  expect(start).toHaveAttribute('aria-disabled')
  await chooseOption(user, screen.getByRole('combobox', { name: /후보 C/ }), 'Writer C')
  expect(start).toHaveAttribute('aria-disabled')
  release()
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)
  await waitFor(() =>
    expect(candidateStarts[0]?.refs.map((ref) => ref.modelId)).toEqual([
      'writer-a',
      'writer-b',
      'writer-c',
    ]),
  )
})

it('shows an A/B collision or locked extra and refuses to start', async () => {
  const user = userEvent.setup()
  const view = renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: { posts: [{ slug: 'post-1', title: '첫 글' }] },
    providers: { models: writeModels, comparisonPairs: [writePair] },
  })
  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '첫 글')
  await user.click(screen.getByRole('button', { name: '후보 추가' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /후보 C/ }), 'Writer A')
  expect(screen.getByText(/A\/B 후보와 같은 모델/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '비교 시작' })).toHaveAttribute('aria-disabled')
  view.unmount()

  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: { posts: [{ slug: 'post-1', title: '첫 글' }] },
    providers: {
      models: [
        ...writeModels,
        {
          providerId: 'openrouter',
          modelId: 'locked',
          label: 'Locked',
          access: {
            [Stage.WRITE]: {
              grade: 'premium',
              requiredPlan: 'pro',
              entitled: false,
              unavailableReason: 'MODEL_PLAN_REQUIRED',
            },
          },
        },
      ],
      comparisonPairs: [
        { ...writePair, extraCandidates: [{ providerId: 'openrouter', modelId: 'locked' }] },
      ],
    },
  })
  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '첫 글')
  expect(screen.getByRole('button', { name: '비교 시작' })).toHaveAttribute('aria-disabled')
  expect(screen.getByRole('combobox', { name: /후보 C/ })).toHaveTextContent('Locked')
})

it('requires and sends the explicit active observe model for a post with photos', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'photo-post',
          title: '사진 글',
          images: [{ id: 'image-1', filename: 'photo.jpg' }],
        },
      ],
    },
    providers: {
      models: [
        ...writeModels,
        { providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true },
      ],
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'vision' }],
      comparisonPairs: [writePair],
    },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '사진 글')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  await waitFor(() =>
    expect(starts[0]?.observeModel).toEqual({
      providerId: 'openrouter',
      modelId: 'vision',
    }),
  )
})

// The model-lab half: the write comparison's second entry point (MODEL-31) goes through the
// same re-observation picker with the same reuse contract as the editor's.
it('routes a model-lab comparison through the re-observation picker', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'photo-post',
          title: '사진 글',
          images: [
            { id: 'image-1', filename: 'photo.jpg' },
            { id: 'image-2', filename: 'other.jpg' },
          ],
          observations: [
            create(ObservationSchema, {
              file: 'photo.jpg',
              scene: '이미 본 장면',
              model: 'openrouter/vision',
            }),
            create(ObservationSchema, {
              file: 'other.jpg',
              scene: '또 다른 장면',
              model: 'openrouter/vision',
            }),
          ],
        },
      ],
    },
    providers: {
      models: [
        ...writeModels,
        { providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true },
      ],
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'vision' }],
      comparisonPairs: [writePair],
    },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '사진 글')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  const picker = await screen.findByRole('dialog', { name: '다시 관찰할 사진 선택' })
  expect(starts).toHaveLength(0)
  await user.click(within(picker).getByRole('checkbox', { name: 'photo.jpg 다시 관찰' }))
  await user.click(within(picker).getByRole('button', { name: '이대로 시작' }))

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0].reobserveFiles).toEqual(['photo.jpg'])
})

it('keeps photo-backed writing blocked without an active observe model and reports start errors', async () => {
  const user = userEvent.setup()
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'photo-post',
          title: '사진 글',
          images: [{ id: 'image-1', filename: 'photo.jpg' }],
        },
        { slug: 'text-post', title: '텍스트 글' },
      ],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { startError: 'provider unavailable' },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '사진 글')
  expect(await screen.findByText('관찰 모델을 선택하세요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '비교 시작' })).toHaveAttribute('aria-disabled', 'true')

  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '텍스트 글')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)
  expect(await screen.findByText('네트워크에 연결할 수 없어요.')).toBeInTheDocument()
})

it('blocks posts with active work or an unresolved write experiment', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'busy-post',
          title: '작업 중인 글',
          activeJob: { id: 'job-1', status: 'running' },
        },
        {
          slug: 'pending-post',
          title: '결과 대기 글',
          pendingExperimentId: 'experiment-pending',
        },
      ],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '작업 중인 글')
  expect(await screen.findByText('이미 생성 중이에요.')).toBeInTheDocument()

  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '결과 대기 글')
  expect(await screen.findByText('먼저 대기 중인 A/B 결과를 확인해 주세요.')).toBeInTheDocument()
  expect(starts).toHaveLength(0)
})

// MODEL-31: the lab's write tab takes an owned post in any status. A lab comparison writes
// nothing to its post (MODEL-66), so the published lock that guards writes does not hold it.
it('starts a lab write comparison on a published post', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'published-post',
          title: '발행된 글',
          status: 'published',
          publishedUrl: FAKE_PUBLISHED_URL,
        },
      ],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '발행된 글')
  const startButton = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(startButton).not.toHaveAttribute('aria-disabled'))
  expect(screen.queryByText(/발행된 글은 바꿀 수 없어요/)).not.toBeInTheDocument()
  await user.click(startButton)
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({ postSlug: 'published-post', origin: ExperimentOrigin.LAB })
})

// MODEL-31, review F76: a video-only post observes too, so the lab sends the active observe
// model for it exactly as the editor's entry does — without it the start is refused.
it('sends the observe model for a post with videos and no photos', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'video-post',
          title: '영상 글',
          videos: [{ id: 'video-1', filename: 'clip.mp4' }],
        },
      ],
    },
    providers: {
      models: [
        ...writeModels,
        {
          providerId: 'openrouter',
          modelId: 'vision',
          label: 'Vision',
          vision: true,
          videoInput: true,
          signedVideoUrl: true,
        },
      ],
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'vision' }],
      comparisonPairs: [writePair],
    },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /비교할 글/ }), '영상 글')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)
  await waitFor(() =>
    expect(starts[0]?.observeModel).toEqual({ providerId: 'openrouter', modelId: 'vision' }),
  )
})

const photoPost = {
  slug: 'photo-post',
  title: '관찰할 사진',
  images: [{ id: 'photo', filename: 'photo.jpg' }],
}

const observePair = { ...writePair, stage: Stage.OBSERVE }

// MODEL-30, MODEL-67: the lab compares observe and write, and 말투 반영 as a write comparison drawn
// from a voice. Analyze keeps its active selection on 모델 변경, so it has no tab here, and only
// the 말투 반영 tab asks for a voice.
it('offers observe, write and 말투 반영, and the voice picker on 말투 반영 alone', async () => {
  const user = userEvent.setup()
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: { posts: [photoPost] },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    voice: { voices: [{ id: 'voice-default', name: '기본 말투', isDefault: true }] },
  })
  const tabs = within(await screen.findByRole('tablist', { name: 'AI 단계' }))
  expect(tabs.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
    '관찰',
    '글 작성',
    '말투 반영',
  ])
  expect(screen.queryByRole('combobox', { name: /말투/ })).not.toBeInTheDocument()
  await user.click(tabs.getByRole('tab', { name: '글 작성' }))
  expect(await screen.findByRole('combobox', { name: /비교할 글/ })).toBeInTheDocument()
  expect(screen.queryByRole('combobox', { name: /말투/ })).not.toBeInTheDocument()
})

// A link naming analyze is read as a typo would be: the stage is dropped and the page opens on
// its default, the observe comparison, rather than on a tab that no longer exists.
it('opens an address naming analyze on the observe comparison', async () => {
  renderAppAt('/ai-models/compare?stage=analyze', {
    user: { id: 'owner-1' },
    posts: { posts: [photoPost] },
    providers: { models: writeModels, comparisonPairs: [observePair] },
  })
  expect(await screen.findByRole('tab', { name: '관찰' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.queryByRole('tab', { name: '문체 분석' })).not.toBeInTheDocument()
  expect(await screen.findByRole('combobox', { name: /사진이 있는 글/ })).toBeInTheDocument()
})

// MODEL-65: the gate reads the pair the screen shows. A change that wrote nothing leaves the
// stored pair behind the fields, so nothing may start against the pair no longer on screen.
it('starts nothing while the fields show a pair the store does not hold', async () => {
  const user = userEvent.setup()
  const observeStarts: NonNullable<FakeExperimentsOptions['observeStarts']> = []
  renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: { posts: [photoPost] },
    providers: {
      models: writeModels.map((model) => ({ ...model, vision: true })),
      comparisonPairs: [observePair],
    },
    experiments: { observeStarts },
  })
  await chooseOption(
    user,
    await screen.findByRole('combobox', { name: /사진이 있는 글/ }),
    '관찰할 사진',
  )
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  const candidateB = await screen.findByRole('combobox', { name: /후보 B/ })
  await chooseOption(user, candidateB, 'Writer A')
  expect(candidateB).toHaveAccessibleDescription('서로 다른 모델을 선택해 주세요.')
  await waitFor(() => expect(start).toHaveAttribute('aria-disabled', 'true'))
  await user.click(start)
  expect(observeStarts).toEqual([])
  // Putting the stored pair back on screen reopens the gate.
  await chooseOption(user, candidateB, 'Writer B')
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
})

it('opens observation comparison without work and starts only with the chosen saved pair and post', async () => {
  const user = userEvent.setup()
  const observeStarts: NonNullable<FakeExperimentsOptions['observeStarts']> = []
  const candidateStarts: NonNullable<FakeExperimentsOptions['candidateStarts']> = []
  const { router } = renderAppAt('/ai-models/compare', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'photo-post',
          title: '관찰할 사진',
          images: [{ id: 'photo', filename: 'photo.jpg' }],
        },
      ],
    },
    providers: {
      models: [
        ...writeModels,
        { providerId: 'openrouter', modelId: 'writer-c', label: 'Writer C' },
      ].map((model) => ({ ...model, vision: true })),
      comparisonPairs: [
        {
          ...writePair,
          stage: Stage.OBSERVE,
          extraCandidates: [{ providerId: 'openrouter', modelId: 'writer-c' }],
        },
      ],
    },
    experiments: { observeStarts, candidateStarts, experimentId: 'observe-1' },
  })
  await chooseOption(
    user,
    await screen.findByRole('combobox', { name: /사진이 있는 글/ }),
    '관찰할 사진',
  )
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  expect(observeStarts).toEqual([])
  await user.click(start)
  await waitFor(() =>
    expect(observeStarts).toEqual([
      {
        postSlug: 'photo-post',
        modelA: writePair.candidateA,
        modelB: writePair.candidateB,
      },
    ]),
  )
  expect(candidateStarts[0]).toEqual({
    kind: 'observe',
    refs: [
      writePair.candidateA,
      writePair.candidateB,
      { providerId: 'openrouter', modelId: 'writer-c' },
    ],
  })
  await waitFor(() =>
    expect(router.state.location.pathname).toBe('/ai-models/experiments/observe-1'),
  )
  expect(router.state.location.search.from).toBe('compare')
  expect(await screen.findByRole('link', { name: '← 모델 비교로 돌아가기' })).toHaveAttribute(
    'href',
    '/ai-models/compare?stage=observe',
  )
  expect(router.state.location.search.stage).toBe('observe')
})

// MODEL-41, MODEL-67: 말투 반영 picks a made voice — the 기본 first and chosen — one of its answered
// prompts, and the saved write pair; a photo prompt is listed but not choosable unless both write
// models read images, and 비교 시작 starts the comparison and opens its review.
describe('the 말투 반영 tab', () => {
  const voices = [
    { id: 'voice-review', name: '리뷰' },
    { id: 'voice-default', name: '기본 말투', isDefault: true },
    { id: 'voice-new', name: '새 말투', made: false },
  ]
  const answers = [
    {
      id: 'a1',
      label: '',
      kind: 'answer' as const,
      promptKey: 'opening_greeting',
      body: '안녕하세요!',
    },
    {
      id: 'a2',
      label: '',
      kind: 'answer' as const,
      promptKey: 'photo_food',
      body: '크루아상이에요.',
    },
  ]

  it('starts a comparison on the 기본 and an answered prompt', async () => {
    const user = userEvent.setup()
    const reflectionStarts: NonNullable<FakeExperimentsOptions['reflectionStarts']> = []
    const candidateStarts: NonNullable<FakeExperimentsOptions['candidateStarts']> = []
    const { router } = renderAppAt('/ai-models/compare?stage=voice', {
      user: { id: 'owner-1' },
      providers: {
        models: [
          ...writeModels,
          { providerId: 'openrouter', modelId: 'writer-c', label: 'Writer C' },
        ],
        comparisonPairs: [
          { ...writePair, extraCandidates: [{ providerId: 'openrouter', modelId: 'writer-c' }] },
        ],
      },
      voice: { voices, samples: answers },
      experiments: { reflectionStarts, candidateStarts, experimentId: 'reflection-1' },
    })

    const voice = await screen.findByRole('combobox', { name: /^말투/ })
    await waitFor(() => expect(voice).toHaveTextContent('기본 말투'))
    await user.click(voice)
    const listbox = await screen.findByRole('listbox')
    expect(
      within(listbox)
        .getAllByRole('option')
        .map((option) => option.textContent),
    ).toEqual(['기본 말투', '리뷰'])
    await user.keyboard('{Escape}')

    const prompt = screen.getByRole('combobox', { name: /^비교할 문항/ })
    await waitFor(() => expect(prompt).toHaveTextContent('블로그 글을 시작할 때'))
    await user.click(prompt)
    const photo = within(await screen.findByRole('listbox')).getByRole('option', {
      name: /음식이나 음료 사진/,
    })
    expect(photo).toHaveAttribute('aria-disabled', 'true')
    await user.keyboard('{Escape}')
    expect(
      screen.getByText('사진 문항은 모든 후보가 사진을 읽을 때만 비교할 수 있어요.'),
    ).toBeInTheDocument()

    const start = screen.getByRole('button', { name: '비교 시작' })
    await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
    await user.click(start)
    await waitFor(() =>
      expect(reflectionStarts).toEqual([
        {
          voiceId: 'voice-default',
          promptKey: 'opening_greeting',
          modelA: { providerId: 'openrouter', modelId: 'writer-a' },
          modelB: { providerId: 'openrouter', modelId: 'writer-b' },
        },
      ]),
    )
    expect(candidateStarts[0]).toEqual({
      kind: 'voice',
      refs: [
        writePair.candidateA,
        writePair.candidateB,
        { providerId: 'openrouter', modelId: 'writer-c' },
      ],
    })
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/ai-models/experiments/reflection-1'),
    )
  })

  it('offers a photo prompt once both write models read images', async () => {
    const user = userEvent.setup()
    renderAppAt('/ai-models/compare?stage=voice', {
      user: { id: 'owner-1' },
      providers: {
        models: writeModels.map((model) => ({ ...model, vision: true })),
        comparisonPairs: [writePair],
      },
      voice: { voices, samples: answers },
    })

    const prompt = await screen.findByRole('combobox', { name: /^비교할 문항/ })
    await waitFor(() => expect(prompt).toHaveTextContent('블로그 글을 시작할 때'))
    await user.click(prompt)
    const photo = within(await screen.findByRole('listbox')).getByRole('option', {
      name: /음식이나 음료 사진/,
    })
    expect(photo).not.toHaveAttribute('aria-disabled')
  })

  it('sends a voice with no answered prompt to its 학습 글', async () => {
    renderAppAt('/ai-models/compare?stage=voice', {
      user: { id: 'owner-1' },
      providers: { models: writeModels, comparisonPairs: [writePair] },
      voice: { voices },
    })

    expect(
      await screen.findAllByText('이 말투에는 답한 문항이 없어요. 학습 글에서 문항에 답해 주세요.'),
    ).not.toHaveLength(0)
    expect(await screen.findByRole('link', { name: '문항에 답하러 가기' })).toHaveAttribute(
      'href',
      '/voices/voice-default/materials',
    )
    expect(screen.getByRole('button', { name: '비교 시작' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
  })
})
