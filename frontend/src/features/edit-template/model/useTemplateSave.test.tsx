import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { Template } from '@/entities/template'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeTemplatesOptions } from '@/test/templates'
import { useTemplateDraft } from './useTemplateDraft'
import { useTemplateSave, type TemplateSaveRequest } from './useTemplateSave'

interface BlockerOptions {
  shouldBlockFn: () => boolean
  enableBeforeUnload: () => boolean
}

/** The router as the hook meets it: a navigate to record, and a blocker whose guard a test can
 *  ask directly and whose question it can open. */
const router = vi.hoisted(() => ({
  navigate: vi.fn(),
  status: 'idle' as 'idle' | 'blocked',
  options: undefined as BlockerOptions | undefined,
  proceed: vi.fn(),
  reset: vi.fn(),
}))
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useNavigate: () => router.navigate,
  useBlocker: (options: BlockerOptions) => {
    router.options = options
    return { status: router.status, proceed: router.proceed, reset: router.reset }
  },
}))

const OWNER = 'alice'

const REVIEW: Template = {
  id: 'template-review',
  name: '정보성 식당 리뷰',
  description: '협찬 방문 리뷰',
  body: '<write>인트로를 씁니다</write>',
  titleArea: '',
  postCount: 0,
  createdAt: '2026-08-28T12:00:00Z',
  updatedAt: '2026-08-28T12:00:00Z',
}

const IDLE: TemplateSaveRequest = { running: false, cancel: () => undefined }

function setup(
  stored: Template | undefined,
  { request = IDLE, others = [] }: { request?: TemplateSaveRequest; others?: Template[] } = {},
) {
  const updates: NonNullable<FakeTemplatesOptions['updates']> = []
  const creates: NonNullable<FakeTemplatesOptions['creates']> = []
  const transport = createFakeAuthTransport({
    user: { id: OWNER },
    templates: { templates: [...(stored ? [stored] : []), ...others], updates, creates },
  })
  const view = renderHook(
    ({ request }: { request: TemplateSaveRequest }) => {
      const draft = useTemplateDraft(stored)
      const saving = useTemplateSave({ ownerId: OWNER, stored, draft, request })
      return { draft, saving }
    },
    { wrapper: withProviders(transport, createTestQueryClient()), initialProps: { request } },
  )
  return { ...view, updates, creates }
}

/** Whether leaving now would be asked about, as the router would ask it. */
const guarded = () => router.options?.shouldBlockFn() ?? false

beforeEach(() => {
  router.navigate.mockReset()
  router.proceed.mockReset()
  router.reset.mockReset()
  router.status = 'idle'
  router.options = undefined
})

describe('the save', () => {
  it('refuses until the draft differs, then writes every field in one update', async () => {
    const { result, updates } = setup(REVIEW)
    expect(result.current.saving.blocked).toBe(true)
    await act(() => result.current.saving.save())
    expect(updates).toHaveLength(0)

    act(() => result.current.draft.setText('name')('정보성 식당 리뷰 2편 '))
    act(() => result.current.draft.setTagCount({ enabled: true }))
    expect(result.current.saving.blocked).toBe(false)
    await act(() => result.current.saving.save())

    expect(updates).toEqual([
      {
        id: 'template-review',
        name: '정보성 식당 리뷰 2편',
        description: '협찬 방문 리뷰',
        body: '<write>인트로를 씁니다</write>',
        titleArea: '',
        targetLength: undefined,
        tagCount: 4,
      },
    ])
    expect(router.navigate).not.toHaveBeenCalled()
  })

  // The baseline comes from the mutation's own answer, not the directory refetch that lags it.
  it('goes clean the moment the update lands, on what the server stored', async () => {
    const { result } = setup(REVIEW)
    act(() => result.current.draft.setText('body')('  <write>새 인트로</write>  '))

    await act(() => result.current.saving.save())

    // The fake server trims the body at its edges, as the real one does (TMPL-6).
    expect(result.current.draft.draft.body).toBe('<write>새 인트로</write>')
    expect(result.current.draft.dirty).toBe(false)
    expect(result.current.draft.saved).toBe(true)
    expect(result.current.saving.blocked).toBe(true)
    expect(guarded()).toBe(false)
  })

  it('creates a new template and lands on it, without its own guard intercepting', async () => {
    const { result, creates } = setup(undefined)
    let guardedAtRedirect: boolean | undefined
    router.navigate.mockImplementation(async () => {
      guardedAtRedirect = guarded()
    })
    act(() => result.current.draft.setText('name')('카페 방문기'))
    act(() => result.current.draft.setText('body')('<write>첫인상을 씁니다</write>'))
    expect(guarded()).toBe(true)

    await act(() => result.current.saving.save())

    expect(creates).toHaveLength(1)
    expect(creates[0]).toMatchObject({ name: '카페 방문기', titleArea: '' })
    // `replace`: Back from the saved template goes to the list, not to a stale `new` screen.
    expect(router.navigate).toHaveBeenCalledExactlyOnceWith({
      to: '/templates/$templateId',
      params: { templateId: 'template-1' },
      replace: true,
    })
    expect(guardedAtRedirect).toBe(false)
  })

  it('keeps the draft dirty and reports the refusal when the server says no', async () => {
    const other: Template = { ...REVIEW, id: 'template-other', name: '카페 리뷰' }
    const { result, updates } = setup(REVIEW, { others: [other] })
    act(() => result.current.draft.setText('name')('카페 리뷰'))

    await act(() => result.current.saving.save())

    expect(updates).toHaveLength(1)
    await waitFor(() => expect(result.current.saving.failed).toBe(true))
    expect(result.current.saving.errorMessage).not.toBe('')
    expect(result.current.draft.dirty).toBe(true)
    expect(result.current.draft.saved).toBe(false)
    expect(guarded()).toBe(true)
  })
})

describe('the leave guard', () => {
  it('asks only while there is something unsaved, and on a tab close too', () => {
    const { result } = setup(REVIEW)
    expect(guarded()).toBe(false)
    expect(router.options?.enableBeforeUnload()).toBe(false)

    act(() => result.current.draft.setText('name')('바뀐 이름'))
    expect(guarded()).toBe(true)
    expect(router.options?.enableBeforeUnload()).toBe(true)
  })

  it('stays on the screen, or leaves, as the question is answered', () => {
    router.status = 'blocked'
    const { result } = setup(REVIEW)
    expect(result.current.saving.leave.asking).toBe(true)

    act(() => result.current.saving.leave.stay())
    expect(router.reset).toHaveBeenCalledOnce()
    act(() => result.current.saving.leave.go())
    expect(router.proceed).toHaveBeenCalledOnce()
  })
})

// TMPL-63: a running request locks the draft, is something to lose, and leaving cancels it.
describe('a running request', () => {
  it('locks the draft and refuses the save while it runs', async () => {
    const { result, rerender, updates } = setup(REVIEW)
    act(() => result.current.draft.setText('name')('바뀐 이름'))
    expect(result.current.saving.locked).toBe(false)

    rerender({ request: { running: true, cancel: () => undefined } })
    expect(result.current.saving.locked).toBe(true)
    expect(result.current.saving.blocked).toBe(true)
    await act(() => result.current.saving.save())
    expect(updates).toHaveLength(0)
  })

  it('guards a clean draft while it runs, and leaving cancels it', () => {
    const cancel = vi.fn()
    router.status = 'blocked'
    const { result } = setup(REVIEW, { request: { running: true, cancel } })
    expect(result.current.draft.dirty).toBe(false)
    expect(guarded()).toBe(true)

    act(() => result.current.saving.leave.go())
    expect(cancel).toHaveBeenCalledOnce()
    expect(router.proceed).toHaveBeenCalledOnce()
  })

  it('is not cancelled by leaving once it has finished', () => {
    const cancel = vi.fn()
    router.status = 'blocked'
    const { result } = setup(REVIEW, { request: { running: false, cancel } })

    act(() => result.current.saving.leave.go())
    expect(cancel).not.toHaveBeenCalled()
    expect(router.proceed).toHaveBeenCalledOnce()
  })
})
