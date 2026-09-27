import { afterEach, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { clipObservationsFixture } from '@/test/clip-observations'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeJobsOptions } from '@/test/jobs'
import type { FakeProvidersOptions } from '@/test/providers'

afterEach(() => initializeI18n('ko'))

const models: FakeProvidersOptions = {
  models: [
    {
      providerId: 'p',
      modelId: 'o',
      label: 'Video observer',
      vision: true,
      videoInput: true,
      inlineStaticVideo: true,
      stages: [Stage.OBSERVE],
    },
    { providerId: 'p', modelId: 'w', label: 'Writer', stages: [Stage.WRITE] },
  ],
  selections: [
    { stage: Stage.OBSERVE, providerId: 'p', modelId: 'o' },
    { stage: Stage.WRITE, providerId: 'p', modelId: 'w' },
  ],
}

function storylined(extra: Partial<FakeClipProject> = {}): FakeClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: 'template',
    ratio: 'vertical',
    targetDurationMs: 19800,
    disclosure: 'ad',
    observations: clipObservationsFixture(),
    storyline: {
      paragraphs: [
        { text: '음식을 가까이 보여줘요.', observationIds: ['a/0'] },
        { text: '테이블로 마무리해요.', observationIds: [] },
      ],
      editedByHand: false,
      addedSourceIds: [],
      takenOutObservationIds: ['a/1'],
    },
    ...extra,
  }
}

async function mount(
  project: FakeClipProject,
  clips: FakeClipsOptions = {},
  jobs: FakeJobsOptions = {},
  wait: 'space' | 'tab' | 'none' = 'space',
) {
  const view = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: models,
    jobs,
    clips: {
      templates: [{ id: 'template', name: '여행', compositionBody: '<clip version="1"/>' }],
      projects: [project],
      eligibility: [{ providerId: 'p', modelId: 'o', status: 'eligible' }],
      ...clips,
    },
  })
  if (wait === 'space') await screen.findByRole('button', { name: '스토리라인' })
  if (wait === 'tab') await screen.findByRole('tab', { name: '수정' })
  return view
}

const heading = () => screen.getByRole('button', { name: '스토리라인' })
const paragraph = (n: number) => within(screen.getByRole('listitem', { name: `${n}번째 문단` }))

// CLIP-36, CLIP-177: a storyline without a plan is already ②, where its space is open — the
// storyline is what the owner works on until something is built from it.
it('opens ② on a storyline with no plan, its space open with paragraphs and scenes', async () => {
  await mount(storylined())
  expect(screen.getByRole('tab', { name: '수정' })).toHaveAttribute('aria-selected', 'true')
  expect(heading()).toHaveAttribute('aria-expanded', 'true')
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
  expect(paragraph(1).getByText('장면 1')).toBeVisible()
  // No footage in the session: the scene shows what was observed in it instead of a frame.
  expect(paragraph(1).getByText('접시에 담긴 음식을 가까이 촬영')).toBeVisible()
  const out = within(screen.getByRole('region', { name: '빠진 장면' }))
  expect(out.getByText('장면 2')).toBeVisible()
})

// CLIP-179: with a plan the space starts closed — the plan is what ② is for then.
it('closes the space when ② shows a plan', async () => {
  await mount(
    storylined({ editPlanRevision: 1, renderedPlanRevision: 0, editing: clipTimelineFixture() }),
  )
  expect(heading()).toHaveAttribute('aria-expanded', 'false')
  await userEvent.click(heading())
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
})

// CLIP-178, CLIP-39: an edit saves itself — the text, a scene moved, a scene taken out and put
// back — through the project update, with no save button.
it('saves the owner’s edits by themselves', async () => {
  const storylineEdits: NonNullable<FakeClipsOptions['storylineEdits']> = []
  await mount(storylined(), { storylineEdits })
  const user = userEvent.setup()
  await user.click(paragraph(2).getByRole('button', { name: '2번째 문단 고치기' }))
  const field = paragraph(2).getByRole('textbox', { name: '2번째 문단' })
  await user.clear(field)
  await user.type(field, '테이블을 비추며 끝내요.')
  await waitFor(() => expect(storylineEdits.at(-1)?.[1]?.text).toBe('테이블을 비추며 끝내요.'), {
    timeout: 3000,
  })
  await user.click(paragraph(2).getByRole('button', { name: '완료' }))

  await user.click(paragraph(1).getByRole('button', { name: '장면 1 옮기기' }))
  await user.click(await screen.findByRole('menuitemradio', { name: '2번째 문단' }))
  await waitFor(() =>
    expect(storylineEdits.at(-1)?.map((p) => p.observationIds)).toEqual([[], ['a/0']]),
  )

  await user.click(paragraph(2).getByRole('button', { name: '장면 1 옮기기' }))
  await user.click(await screen.findByRole('menuitemradio', { name: '빼기' }))
  await waitFor(() => expect(storylineEdits.at(-1)?.map((p) => p.observationIds)).toEqual([[], []]))
  const out = within(screen.getByRole('region', { name: '빠진 장면' }))
  expect(out.getByText('장면 1')).toBeVisible()

  await user.click(out.getByRole('button', { name: '장면 2 넣기' }))
  await user.click(await screen.findByRole('menuitem', { name: '1번째 문단' }))
  await waitFor(() =>
    expect(storylineEdits.at(-1)?.map((p) => p.observationIds)).toEqual([['a/1'], []]),
  )
})

// CLIP-178: footage added after the storyline was made is said, with what to do about it.
it('says when footage was added after the storyline was made', async () => {
  await mount(storylined({ storyline: { ...storylined().storyline!, addedSourceIds: ['c'] } }))
  expect(
    screen.getByText('이 스토리라인을 만든 뒤 영상이 추가됐어요. 다시 만들면 새 영상도 들어가요.'),
  ).toBeVisible()
})

// CLIP-160: a finalized clip and a running job leave the storyline to be read, not edited.
it.each([
  [
    'finalized',
    storylined({
      editPlanRevision: 1,
      renderedPlanRevision: 1,
      editing: clipTimelineFixture(),
      finalized: { at: '2026-09-12T00:00:00Z', planRevision: 1, resultId: 'result' },
      result: {
        id: 'result',
        contentType: 'video/mp4',
        bytes: 5,
        durationMs: 19800,
        createdAt: '2026-09-10T00:00:00Z',
        viewUrl: 'https://private.test/view',
      },
    }),
  ],
])('reads the storyline without controls when %s', async (_, project) => {
  await mount(project, {}, {}, 'tab')
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
  await screen.findByRole('button', { name: '스토리라인' })
  if (heading().getAttribute('aria-expanded') !== 'true') await userEvent.click(heading())
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
  expect(paragraph(1).queryByRole('button', { name: '1번째 문단 고치기' })).not.toBeInTheDocument()
  expect(paragraph(1).queryByRole('button', { name: '장면 1 옮기기' })).not.toBeInTheDocument()
  // A finalized clip carries none of the space's actions (CLIP-160).
  expect(screen.queryByRole('button', { name: '다시 만들기' })).not.toBeInTheDocument()
  expect(screen.queryByLabelText('스토리라인 수정 요청')).not.toBeInTheDocument()
})

// A job that ② owns keeps ② on screen, and the storyline is read-only under it.
it('reads the storyline without controls while a job runs', async () => {
  const job = {
    id: 'running',
    kind: 'revise_clip',
    status: 'running',
    stage: 'narrate',
    clipProjectId: 'clip',
  }
  await mount(
    storylined({
      editPlanRevision: 1,
      renderedPlanRevision: 1,
      editing: clipTimelineFixture(),
      latestJob: job,
    }),
    {},
    { jobs: [job] },
    'tab',
  )
  await userEvent.click(await screen.findByRole('button', { name: '스토리라인' }))
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
  expect(paragraph(1).queryByRole('button', { name: '1번째 문단 고치기' })).not.toBeInTheDocument()
  // Every action is held while the job runs (CLIP-181).
  expect(screen.getByRole('button', { name: '다시 만들기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '이 스토리로 다시 만들기' })).toBeDisabled()
  expect(screen.getByLabelText('스토리라인 수정 요청')).toBeDisabled()
})

// CLIP-38: the focused run view and the status line name the storyline call's stage.
it('names the storyline stage while the storyline call runs', async () => {
  const job = {
    id: 'running',
    kind: 'storyline_clip',
    status: 'running',
    stage: 'storyline',
    clipProjectId: 'clip',
  }
  await mount(storylined({ latestJob: job }), {}, { jobs: [job] }, 'none')
  expect(await screen.findAllByText('스토리라인 작성 중')).not.toHaveLength(0)
})
