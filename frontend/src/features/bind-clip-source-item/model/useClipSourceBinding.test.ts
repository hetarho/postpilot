import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ClipProject } from '@/entities/clip-project'
import * as entity from '@/entities/clip-project'
import { useClipSourceBinding } from './useClipSourceBinding'

vi.mock('@/entities/clip-project', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/entities/clip-project')>()
  return {
    ...actual,
    projectCompositionDocument: vi.fn(actual.projectCompositionDocument),
    useClipProjectMutations: () => ({
      save: { mutateAsync: vi.fn(), isPending: false, error: null },
    }),
  }
})

const projectWith = (body: string) =>
  ({ id: 'p1', composition: { snapshot: { body } } }) as unknown as ClipProject

describe('useClipSourceBinding', () => {
  it('parses the composition body once per body', () => {
    const spy = vi.mocked(entity.projectCompositionDocument)
    spy.mockClear()
    const { rerender } = renderHook(({ p }) => useClipSourceBinding('o1', p), {
      initialProps: { p: projectWith('{}') },
    })
    rerender({ p: projectWith('{}') })
    expect(spy).toHaveBeenCalledTimes(1)
    rerender({ p: projectWith('{"a":1}') })
    expect(spy).toHaveBeenCalledTimes(2)
  })
})
