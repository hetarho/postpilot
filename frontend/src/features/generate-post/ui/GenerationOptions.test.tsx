import { cleanup, render, renderHook, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { usePost, type GenerationOptionsSet } from '@/entities/post'
import type { FakeOptionsSave, FakePostsOptions } from '@/test/posts'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { GenerationOptions, type RunOptionsForm } from './GenerationOptions'

afterEach(cleanup)

const SAVED: GenerationOptionsSet = {
  tagCount: 4,
  useMemory: false,
  qualityRules: ['title_saturation'],
  field: 'cafe',
}

/** Stands in for the brief's three controls: shows what the form holds and changes one member. */
function Child({ form }: { form: RunOptionsForm }) {
  return (
    <div>
      <output aria-label="form values">{JSON.stringify(form.values)}</output>
      <output aria-label="form state">
        {JSON.stringify({ disabled: form.disabled, jobRunning: form.jobRunning })}
      </output>
      <button type="button" onClick={() => form.change({ useMemory: true })}>
        기억 켜기
      </button>
    </div>
  )
}

function renderOptions(
  overrides: Partial<Parameters<typeof GenerationOptions>[0]> = {},
  posts: Partial<FakePostsOptions> = {},
) {
  const calls: string[] = []
  const optionSaves: FakeOptionsSave[] = []
  const transport = createFakeAuthTransport({
    posts: { posts: [{ slug: 'post-a', tagCount: 4 }], calls, optionSaves, ...posts },
  })
  const queryClient = createTestQueryClient()
  const onSaved = vi.fn()
  const onClose = vi.fn()
  render(
    <GenerationOptions
      slug="post-a"
      saved={SAVED}
      locked={false}
      jobRunning={false}
      onSaved={onSaved}
      onClose={onClose}
      {...overrides}
    >
      {(form) => <Child form={form} />}
    </GenerationOptions>,
    { wrapper: withProviders(transport, queryClient) },
  )
  return { onSaved, onClose, calls, optionSaves, transport, queryClient }
}

const formState = () => JSON.parse(screen.getByLabelText('form state').textContent ?? '')
const formValues = () => JSON.parse(screen.getByLabelText('form values').textContent ?? '')

describe('GenerationOptions', () => {
  // POST-63: a required upper bound, with no enabling tick or demand to fill it with tags.
  it('shows the required maximum beside the length and explains fewer grounded tags are valid', () => {
    renderOptions()
    expect(screen.getByLabelText('목표 글자 수 사용')).not.toBeChecked()
    const maximum = screen.getByRole('spinbutton', { name: '최대 태그 수' })
    expect(maximum).toHaveValue(4)
    expect(maximum).toBeRequired()
    expect(maximum).toHaveAttribute('min', '1')
    expect(maximum).toHaveAttribute('max', '10')
    expect(maximum).toHaveAccessibleDescription(
      '글의 내용과 자료로 뒷받침되는 태그만 사용해요. 설정한 수보다 적거나 없어도 괜찮아요.',
    )
    expect(screen.queryByRole('checkbox', { name: /태그/ })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('목표 글자 수')).not.toBeInTheDocument()
    expect(formValues()).toEqual({
      useMemory: false,
      qualityRules: ['title_saturation'],
      field: 'cafe',
    })
  })

  it('refuses a count outside 1–10 and says the range', async () => {
    const user = userEvent.setup()
    renderOptions()
    // Nothing has changed yet, so there is nothing to save.
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    const field = screen.getByLabelText('최대 태그 수')
    await user.clear(field)
    await user.type(field, '11')
    expect(screen.getByText('최대 태그 수는 1–10 사이의 정수로 입력해 주세요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.clear(field)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.type(field, '10')
    expect(
      screen.queryByText('최대 태그 수는 1–10 사이의 정수로 입력해 주세요.'),
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
  })

  // POST-89: one 저장 is one request carrying all five, the child's members included; the length
  // stays natural when the box was never ticked.
  it('sends the whole set in one request on 저장 and reports it back', async () => {
    const user = userEvent.setup()
    const { onSaved, onClose, calls, optionSaves } = renderOptions()
    const field = screen.getByLabelText('최대 태그 수')
    await user.clear(field)
    await user.type(field, '7')
    await user.click(screen.getByRole('button', { name: '기억 켜기' }))
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(screen.getByRole('button', { name: '저장' }))
    const sent = { ...SAVED, targetLength: undefined, tagCount: 7, useMemory: true }
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(sent))
    expect(optionSaves).toEqual([{ slug: 'post-a', ...sent }])
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('saves a lower maximum without trimming existing tags or changing the content state', async () => {
    const user = userEvent.setup()
    const existingTags = ['첫 태그', '둘째 태그', '셋째 태그']
    const { calls, optionSaves, onClose, transport, queryClient } = renderOptions(
      {},
      {
        posts: [
          {
            slug: 'post-a',
            tagCount: 4,
            tags: existingTags,
            status: 'review',
            contentRevision: 7n,
            machineBaselineRevision: 7n,
          },
        ],
      },
    )
    const view = renderHook(() => usePost('post-a'), {
      wrapper: withProviders(transport, queryClient),
    })
    await waitFor(() => expect(view.result.current.post?.content?.tags).toEqual(existingTags))
    const content = view.result.current.post?.content
    const field = screen.getByRole('spinbutton', { name: '최대 태그 수' })
    await user.clear(field)
    await user.type(field, '1')
    expect(optionSaves).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    await waitFor(() => expect(view.result.current.post?.tagCount).toBe(1))
    expect(view.result.current.post).toMatchObject({
      status: 'review',
      contentRevision: 7n,
      machineBaselineRevision: 7n,
      content,
    })
    expect(view.result.current.post?.content?.tags).toEqual(existingTags)
    expect(optionSaves).toEqual([{ slug: 'post-a', ...SAVED, tagCount: 1 }])
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('StartRevision')
  })

  // The reviewer's race: a second press or an Enter while the first request is out must not send
  // a second one, and nothing in the form may change under the request.
  it('sends one request per 저장, holding the form while it is out', async () => {
    const user = userEvent.setup()
    let release!: () => void
    const optionSaveGate = new Promise<void>((resolve) => {
      release = resolve
    })
    const { onClose, optionSaves } = renderOptions({}, { optionSaveGate })
    const field = screen.getByLabelText('최대 태그 수')
    await user.clear(field)
    await user.type(field, '7')

    const save = screen.getByRole('button', { name: '저장' })
    await user.click(save)
    await waitFor(() => expect(optionSaves).toHaveLength(1))
    await waitFor(() => expect(save).toBeDisabled())
    expect(screen.getByLabelText('최대 태그 수')).toBeDisabled()
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeDisabled()
    expect(formState()).toEqual({ disabled: true, jobRunning: false })

    await user.click(save)
    await user.type(screen.getByLabelText('최대 태그 수'), '{Enter}')
    expect(optionSaves).toHaveLength(1)

    release()
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    expect(optionSaves).toHaveLength(1)
  })

  it('keeps the form open with the refusal’s reason', async () => {
    const user = userEvent.setup()
    const { onSaved, onClose } = renderOptions({}, { optionSaveFails: true })
    const field = screen.getByLabelText('최대 태그 수')
    await user.clear(field)
    await user.type(field, '7')
    await user.click(screen.getByRole('button', { name: '기억 켜기' }))
    await user.click(screen.getByRole('button', { name: '저장' }))

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(onSaved).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByLabelText('최대 태그 수')).toHaveValue(7)
    expect(formValues()).toMatchObject({ useMemory: true })
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
  })

  // A ticked checkbox over a blank number field is an invalid form nobody asked for: the range
  // error renders under a control the user has not touched yet.
  it('fills 목표 글자 수 with a usable default the moment the box is ticked', async () => {
    const user = userEvent.setup()
    renderOptions()
    expect(screen.queryByLabelText('목표 글자 수')).not.toBeInTheDocument()

    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    const field = screen.getByLabelText('목표 글자 수')
    expect(field).toHaveValue(1000)
    expect(field).not.toHaveAttribute('aria-invalid')
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()

    // What the user typed outranks the default, so unticking and reticking never loses it.
    await user.clear(field)
    await user.type(field, '2400')
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue(2400)
  })

  it('restores and explicitly clears a stored target length without starting generation', async () => {
    const user = userEvent.setup()
    const stored = {
      targetLength: 1200,
      tagCount: 6,
      useMemory: true,
      qualityRules: [],
      field: '',
    } as const
    const { calls, optionSaves } = renderOptions(
      { saved: stored },
      { posts: [{ slug: 'post-a', targetLength: 1200, tagCount: 6, useMemory: true }] },
    )
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeChecked()
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue(1200)
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    await user.click(screen.getByRole('button', { name: '저장' }))
    // One whole set: natural length, and the post's other four as they stand.
    await waitFor(() =>
      expect(optionSaves).toEqual([{ slug: 'post-a', ...stored, targetLength: undefined }]),
    )
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('StartWriteExperiment')
  })

  it('greys the numbers while a job runs', () => {
    renderOptions({ jobRunning: true })
    expect(screen.getByLabelText('최대 태그 수')).toBeDisabled()
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeDisabled()
    // 분야 and 기억 사용 stay usable; the ticks read `jobRunning` themselves.
    expect(formState()).toEqual({ disabled: false, jobRunning: true })
  })
})
