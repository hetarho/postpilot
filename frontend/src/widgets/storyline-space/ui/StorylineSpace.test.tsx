import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef, useState } from 'react'
import type { PostDraft, PostStorylineParagraph } from '@/entities/post'
import type { StorylineActionsHandle } from '@/features/generate-post'
import { Stage } from '@/shared/api'
import type { FakeGenerationStart, FakeJobsOptions } from '@/test/jobs'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { StorylineSpace } from './StorylineSpace'

afterEach(cleanup)

const PARAGRAPHS: PostStorylineParagraph[] = [
  { text: '가게 앞을 보여줍니다.', files: ['a.jpg', 'clip.mp4'] },
  { text: '커피를 이야기합니다.', files: ['b.jpg'] },
]

type SpacePost = Parameters<typeof StorylineSpace>[0]['post']

function post(
  overrides: Partial<NonNullable<PostDraft['storyline']>> = {},
  fields: Partial<SpacePost> = {},
): SpacePost {
  return {
    slug: 'post',
    status: 'draft',
    observations: [],
    pendingExperimentId: '',
    voice: { id: 'voice', name: 'Voice', deleted: false, sourceLanguage: 'ko' },
    content: undefined,
    contentRevision: 0n,
    machineBaselineRevision: 0n,
    storyline: {
      paragraphs: PARAGRAPHS,
      editedByHand: false,
      addedFiles: [],
      takenOutFiles: ['c.jpg'],
      ...overrides,
    },
    images: ['a.jpg', 'b.jpg', 'c.jpg', 'new.jpg'].map((filename, index) => ({
      id: `img-${index}`,
      filename,
      width: 1024,
      height: 768,
      bytes: 1,
      viewUrl: `blob:${filename}`,
    })),
    videos: [
      {
        id: 'vid',
        filename: 'clip.mp4',
        width: 1280,
        height: 720,
        bytes: 1,
        durationMs: 3000,
        contentType: 'video/mp4',
        viewUrl: '',
      },
    ],
    ...fields,
  } as unknown as SpacePost
}

const writer = { providerId: 'openrouter', modelId: 'writer' }
const observer = { providerId: 'openrouter', modelId: 'observer' }

interface HarnessProps {
  hasContent?: boolean
  readOnly?: boolean
  onChange?: (paragraphs: PostStorylineParagraph[]) => void
  source?: SpacePost
  onStarted?: (jobId: string) => void
  beforeStart?: () => Promise<void>
  flushContent?: () => Promise<unknown>
  actionsRef?: React.Ref<StorylineActionsHandle>
}

/** The space over state it owns, the way the editor's autosave holds it. */
function Harness({
  hasContent = false,
  readOnly = false,
  onChange,
  source = post(),
  onStarted = () => {},
  beforeStart = async () => {},
  flushContent = async () => {},
  actionsRef,
}: HarnessProps) {
  const [paragraphs, setParagraphs] = useState(PARAGRAPHS)
  return (
    <StorylineSpace
      ref={actionsRef}
      post={source}
      paragraphs={paragraphs}
      readOnly={readOnly}
      hasContent={hasContent}
      onChange={(next) => {
        setParagraphs(next)
        onChange?.(next)
      }}
      actions={{ onStarted, beforeStart, flushContent, onOpenBrief: () => {} }}
    />
  )
}

function renderSpace(props: HarnessProps = {}, jobs: FakeJobsOptions = {}) {
  const calls: string[] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    providers: {
      // The post has a clip, so the observer has to be able to watch one.
      models: [
        {
          ...observer,
          vision: true,
          videoInput: true,
          signedVideoUrl: true,
          inlineStaticVideo: true,
        },
        writer,
      ],
      selections: [
        { stage: Stage.OBSERVE, ...observer },
        { stage: Stage.WRITE, ...writer },
      ],
    },
    jobs,
  })
  const client = createTestQueryClient()
  const view = render(<Harness {...props} />, { wrapper: withProviders(transport, client) })
  return {
    calls,
    rerender: (next: HarnessProps) => view.rerender(<Harness {...props} {...next} />),
  }
}

/** The actions wait for the model selections to answer. */
async function actionsReady() {
  await waitFor(() => expect(screen.getByRole('button', { name: '다시 만들기' })).toBeEnabled())
}

const paragraph = (n: number) => within(screen.getByRole('listitem', { name: `${n}번째 문단` }))

describe('StorylineSpace', () => {
  it('opens while the post has no content, and closes once content first shows', () => {
    const { rerender } = renderSpace()
    expect(screen.getByRole('button', { name: '스토리라인' })).toHaveAttribute(
      'aria-expanded',
      'true',
    )
    expect(paragraph(1).getByText('가게 앞을 보여줍니다.')).toBeInTheDocument()

    rerender({ hasContent: true })
    expect(screen.getByRole('button', { name: '스토리라인' })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
  })

  it('starts closed over a post that already has content, and opens on request', async () => {
    const user = userEvent.setup()
    renderSpace({ hasContent: true })
    const heading = screen.getByRole('button', { name: '스토리라인' })
    expect(heading).toHaveAttribute('aria-expanded', 'false')
    await user.click(heading)
    expect(paragraph(2).getByText('커피를 이야기합니다.')).toBeInTheDocument()
  })

  it('edits a paragraph’s text in place, handing the whole list back', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    renderSpace({ onChange })
    await user.click(paragraph(2).getByRole('button', { name: '2번째 문단 고치기' }))
    const field = paragraph(2).getByRole('textbox', { name: '2번째 문단' })
    await user.clear(field)
    await user.type(field, '라떼')
    expect(onChange).toHaveBeenLastCalledWith([PARAGRAPHS[0], { text: '라떼', files: ['b.jpg'] }])
    await user.click(paragraph(2).getByRole('button', { name: '완료' }))
    expect(paragraph(2).getByText('라떼')).toBeInTheDocument()
  })

  it('moves a file to another paragraph and takes one out', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    renderSpace({ onChange })
    await user.click(paragraph(1).getByRole('button', { name: 'a.jpg 옮기기' }))
    await user.click(screen.getByRole('menuitemradio', { name: '2번째 문단' }))
    expect(onChange).toHaveBeenLastCalledWith([
      { text: '가게 앞을 보여줍니다.', files: ['clip.mp4'] },
      { text: '커피를 이야기합니다.', files: ['b.jpg', 'a.jpg'] },
    ])

    await user.click(paragraph(2).getByRole('button', { name: 'b.jpg 옮기기' }))
    await user.click(screen.getByRole('menuitemradio', { name: '빼기' }))
    expect(onChange).toHaveBeenLastCalledWith([
      { text: '가게 앞을 보여줍니다.', files: ['clip.mp4'] },
      { text: '커피를 이야기합니다.', files: ['a.jpg'] },
    ])
    // Taken out, it waits under 빠진 사진 with the one taken out before.
    const takenOut = within(screen.getByRole('region', { name: '빠진 사진' }))
    expect(takenOut.getByRole('button', { name: 'b.jpg 넣기' })).toBeInTheDocument()
    expect(takenOut.getByRole('button', { name: 'c.jpg 넣기' })).toBeInTheDocument()
  })

  it('puts a taken-out file back and never offers one added after the storyline', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    renderSpace({ onChange, source: post({ addedFiles: ['new.jpg'] }) })
    expect(
      screen.getByText(
        '이 스토리라인을 만든 뒤 사진이 추가됐어요. 다시 만들면 새 사진도 들어가요.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'new.jpg 넣기' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'c.jpg 넣기' }))
    await user.click(screen.getByRole('menuitem', { name: '1번째 문단' }))
    expect(onChange).toHaveBeenLastCalledWith([
      { text: '가게 앞을 보여줍니다.', files: ['a.jpg', 'clip.mp4', 'c.jpg'] },
      PARAGRAPHS[1],
    ])
    expect(screen.queryByRole('region', { name: '빠진 사진' })).not.toBeInTheDocument()
  })

  it('shows a clip with no view URL by its name', () => {
    renderSpace()
    expect(paragraph(1).getByText('clip.mp4')).toBeInTheDocument()
  })

  it('offers no field and no move control while read-only', () => {
    renderSpace({ readOnly: true })
    expect(screen.queryByRole('button', { name: /고치기$/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /옮기기$/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /넣기$/ })).not.toBeInTheDocument()
    expect(screen.getByRole('region', { name: '빠진 사진' })).toBeInTheDocument()
  })

  // POST-97: 다시 만들기 saves the draft first and starts a storyline job with the chosen models.
  it('makes the storyline again after saving the draft', async () => {
    const user = userEvent.setup()
    const storylineStarts: FakeGenerationStart[] = []
    const beforeStart = vi.fn(async () => {})
    const onStarted = vi.fn()
    renderSpace({ beforeStart, onStarted }, { storylineStarts })
    await actionsReady()
    await user.click(screen.getByRole('button', { name: '다시 만들기' }))
    await waitFor(() => expect(storylineStarts).toHaveLength(1))
    expect(storylineStarts[0]).toMatchObject({ postSlug: 'post', writeModel: writer })
    expect(beforeStart).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(onStarted).toHaveBeenCalledWith('job-started'))
  })

  it('asks before remaking a storyline edited by hand', async () => {
    const user = userEvent.setup()
    const storylineStarts: FakeGenerationStart[] = []
    renderSpace({ source: post({ editedByHand: true }) }, { storylineStarts })
    await actionsReady()
    await user.click(screen.getByRole('button', { name: '다시 만들기' }))
    const dialog = await screen.findByRole('dialog', { name: '스토리라인을 다시 만들까요?' })
    expect(dialog).toHaveTextContent('직접 고친 스토리라인이 새로 만든 것으로 바뀌어요.')
    expect(storylineStarts).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: '다시 만들기' }))
    await waitFor(() => expect(storylineStarts).toHaveLength(1))
  })

  // POST-98: the write action names what it does, flushes both queues and writes along the storyline.
  it('writes from the storyline, and asks first only over a post edited by hand', async () => {
    const user = userEvent.setup()
    const starts: FakeGenerationStart[] = []
    const flushContent = vi.fn(async () => {})
    renderSpace({ flushContent }, { starts })
    await actionsReady()
    await user.click(screen.getByRole('button', { name: '이 스토리로 글 쓰기' }))
    await waitFor(() => expect(starts).toHaveLength(1))
    expect(starts[0]).toMatchObject({ fromStoryline: true, writeModel: writer })
    expect(starts[0]!.reobserveFiles).toBeUndefined()
    expect(flushContent).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    cleanup()

    const edited = post(
      {},
      {
        content: { title: 't', summary: '', tags: [], blocks: [] } as never,
        contentRevision: 3n,
        machineBaselineRevision: 2n,
      },
    )
    const rewrites: FakeGenerationStart[] = []
    renderSpace({ source: edited, hasContent: true }, { starts: rewrites })
    await actionsReady()
    await user.click(screen.getByRole('button', { name: '이 스토리로 다시 쓰기' }))
    const dialog = await screen.findByRole('dialog', { name: '이 스토리로 다시 쓸까요?' })
    expect(dialog).toHaveTextContent('직접 고친 글이 사라지고 이 스토리로 새로 쓰여요.')
    await user.click(within(dialog).getByRole('button', { name: '다시 쓰기' }))
    await waitFor(() => expect(rewrites).toHaveLength(1))
    expect(rewrites[0]!.fromStoryline).toBe(true)
  })

  // GEN-69: the request clears once its job starts, keeps its text on a refusal, and a failed one
  // retries with the text it carried.
  it('sends the storyline request, clears on a start and retries with the same text', async () => {
    const user = userEvent.setup()
    const storylineRequests: Array<{ postSlug: string; request: string }> = []
    const actionsRef = createRef<StorylineActionsHandle>()
    renderSpace({ actionsRef }, { storylineRequests })
    await actionsReady()
    const field = screen.getByRole('textbox', { name: '스토리라인 수정 요청' })
    expect(screen.getByRole('button', { name: '스토리라인 수정 요청 보내기' })).toBeDisabled()
    await user.type(field, '  커피를 앞으로 ')
    expect(screen.getByText('10/500')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '스토리라인 수정 요청 보내기' }))
    await waitFor(() =>
      expect(storylineRequests).toEqual([{ postSlug: 'post', request: '커피를 앞으로' }]),
    )
    await waitFor(() => expect(field).toHaveValue(''))

    actionsRef.current?.retryRequest()
    await waitFor(() => expect(storylineRequests).toHaveLength(2))
    expect(storylineRequests[1]!.request).toBe('커피를 앞으로')
  })

  it('keeps the request on a refusal and says why', async () => {
    const user = userEvent.setup()
    renderSpace({}, { storylineRequestRefusal: 'POST_STORYLINE_MISSING' })
    await actionsReady()
    const field = screen.getByRole('textbox', { name: '스토리라인 수정 요청' })
    await user.type(field, '짧게')
    await user.click(screen.getByRole('button', { name: '스토리라인 수정 요청 보내기' }))
    expect(
      await screen.findByText('아직 스토리라인이 없어요. 먼저 스토리라인을 만들어 주세요.'),
    ).toBeInTheDocument()
    expect(field).toHaveValue('짧게')
  })

  it('holds every action while a job targets the post, and says why above the row', async () => {
    renderSpace({})
    cleanup()
    const running = { id: 'job-1', kind: 'storyline', status: 'running', stage: 'storyline' }
    const calls: string[] = []
    const transport = createFakeAuthTransport({
      user: { id: 'alice' },
      calls,
      providers: { models: [writer], selections: [{ stage: Stage.WRITE, ...writer }] },
    })
    render(
      <StorylineSpace
        post={post()}
        paragraphs={PARAGRAPHS}
        onChange={() => {}}
        readOnly
        hasContent={false}
        actions={{
          activeJob: {
            ...running,
            progressDone: 0,
            progressTotal: 1,
            failure: undefined,
            postSlug: 'post',
            observeModel: undefined,
            writeModel: undefined,
            createdAt: '',
            updatedAt: '',
            targetLanguage: undefined,
          },
          onStarted: () => {},
          beforeStart: async () => {},
          flushContent: async () => {},
          onOpenBrief: () => {},
        }}
      />,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )
    await waitFor(() => expect(calls).toContain('GetSelections'))
    expect(screen.getByRole('button', { name: '다시 만들기' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '이 스토리로 글 쓰기' })).toBeDisabled()
    expect(screen.getByRole('textbox', { name: '스토리라인 수정 요청' })).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('이미 생성 중이에요.')
  })
})
