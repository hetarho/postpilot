import { describe, expect, it } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoMemoryKind } from '@/shared/api'
import type { FakeMemoriesOptions } from '@/test/memories'
import type { FakeJobsOptions } from '@/test/jobs'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { ExtractMemoriesButton } from './ExtractMemoriesButton'

const CANDIDATES = [
  { text: '매운 음식을 못 먹는다', kind: ProtoMemoryKind.PREFERENCE, tags: ['음식'] },
  { text: '연남동에 자주 간다', kind: ProtoMemoryKind.PLACE, tags: ['연남동'] },
]

/** The job the start hands back, already finished: this feature's subject is what happens after
 *  the extraction lands, and the polling itself is the job entity's own tested behaviour. */
const DONE: FakeJobsOptions = {
  jobs: [{ id: 'extract-job', kind: 'extract_memory', status: 'done' }],
}

function renderButton(
  memories: FakeMemoriesOptions = {},
  jobs: FakeJobsOptions = DONE,
  hasContent = true,
) {
  const transport = createFakeAuthTransport({ user: { id: 'alice' }, memories, jobs })
  return render(
    <ExtractMemoriesButton ownerId="alice" postSlug="draft" hasContent={hasContent} />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
}

describe('기억으로 저장', () => {
  // POST-72: the reason is stated in place rather than left behind a silent disabled control.
  it('is disabled with its reason stated while the post has no content', () => {
    renderButton({}, DONE, false)
    expect(screen.getByRole('button', { name: '기억으로 저장' })).toBeDisabled()
    expect(screen.getByText('글이 만들어진 뒤에 기억을 뽑을 수 있어요.')).toBeInTheDocument()
  })

  // MEM-13/QUOTA-13: the credit gate refuses at the enqueue seam, and nothing is started.
  it('renders the start refusal and opens no sheet', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderButton({ extractionRefused: true, calls })

    await user.click(screen.getByRole('button', { name: '기억으로 저장' }))

    expect(await screen.findByText(/크레딧/)).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(calls.filter((call) => call === 'CreateMemory')).toEqual([])
  })

  it('opens the candidates as a checkbox list and saves only the checked ones', async () => {
    const user = userEvent.setup()
    const creates: FakeMemoriesOptions['creates'] = []
    const extractions: string[] = []
    renderButton({ candidates: CANDIDATES, creates, extractions })

    await user.click(screen.getByRole('button', { name: '기억으로 저장' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(extractions).toEqual(['draft'])

    // Everything starts checked: the work is removing what is wrong, not picking what is right.
    const boxes = sheet.getAllByRole('checkbox')
    expect(boxes.every((box) => (box as HTMLInputElement).checked)).toBe(true)
    await user.click(boxes[1]!)
    await user.click(sheet.getByRole('button', { name: '1개 저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      text: '매운 음식을 못 먹는다',
      kind: ProtoMemoryKind.PREFERENCE,
      tags: ['음식'],
      // The post it was approved from, so the memory outlives it only with another source.
      sourcePostSlug: 'draft',
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  // The bulk shape 지침's 전부 수락 uses: one refusal keeps its row and its reason, the rest save.
  it('keeps a refused row in the sheet with its reason and saves its neighbours', async () => {
    const user = userEvent.setup()
    const creates: FakeMemoriesOptions['creates'] = []
    renderButton({ candidates: CANDIDATES, creates, refuseCreateOf: '연남동에 자주 간다' })

    await user.click(screen.getByRole('button', { name: '기억으로 저장' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.click(sheet.getByRole('button', { name: '2개 저장' }))

    await waitFor(() => expect(creates).toHaveLength(2))
    // Still open, with the reason on the row it belongs to.
    expect(await sheet.findByText('이미 같은 기억이 있어요.')).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    // And a retry would send only the one that failed.
    expect(sheet.getByRole('button', { name: '1개 저장' })).toBeInTheDocument()
  })

  // MEM-15: an unchecked candidate is discarded with the sheet — there is no "later" anywhere.
  it('discards what was not checked when the sheet is closed', async () => {
    const user = userEvent.setup()
    const creates: FakeMemoriesOptions['creates'] = []
    renderButton({ candidates: CANDIDATES, creates })

    await user.click(screen.getByRole('button', { name: '기억으로 저장' }))
    const sheet = within(await screen.findByRole('dialog'))
    await user.click(sheet.getByRole('button', { name: '닫기' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(creates).toEqual([])
    expect(screen.queryByText('나중에')).not.toBeInTheDocument()
    // The button is offered again: the extraction is repeatable on demand.
    expect(screen.getByRole('button', { name: '기억으로 저장' })).toBeEnabled()
  })

  it('says so, and offers no save, when the post yielded nothing', async () => {
    const user = userEvent.setup()
    renderButton({ candidates: [] })

    await user.click(screen.getByRole('button', { name: '기억으로 저장' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(
      await sheet.findByText(
        '이 글에서 저장할 만한 사실을 찾지 못했어요. 나중에 다시 눌러도 돼요.',
      ),
    ).toBeInTheDocument()
    expect(sheet.queryByRole('button', { name: /저장$/ })).not.toBeInTheDocument()
  })

  it('reports a failed extraction and stores nothing', async () => {
    const user = userEvent.setup()
    const creates: FakeMemoriesOptions['creates'] = []
    renderButton(
      { candidates: CANDIDATES, creates },
      { jobs: [{ id: 'extract-job', kind: 'extract_memory', status: 'failed' }] },
    )

    await user.click(screen.getByRole('button', { name: '기억으로 저장' }))

    expect(await screen.findByText('기억을 뽑지 못했어요. 다시 시도해 주세요.')).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(creates).toEqual([])
  })
})
