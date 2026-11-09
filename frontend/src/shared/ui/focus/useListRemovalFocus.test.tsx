import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { useListRemovalFocus } from './useListRemovalFocus'

afterEach(() => document.querySelectorAll('[data-test-control]').forEach((node) => node.remove()))

function control() {
  const button = document.createElement('button')
  button.dataset.testControl = ''
  document.body.append(button)
  return button
}

function mount(keys = ['first', 'middle', 'last']) {
  const targets = new Map(keys.map((key) => [key, control()]))
  const fallback = control()
  const hook = renderHook(
    ({ keys }) =>
      useListRemovalFocus({
        keys,
        getTarget: (key) => targets.get(key),
        getFallback: () => fallback,
      }),
    { initialProps: { keys } },
  )
  const update = async (keys: string[]) => {
    await act(async () => hook.rerender({ keys }))
  }
  return { ...hook, update, targets, fallback }
}

describe('useListRemovalFocus', () => {
  it('waits for the requested identity to leave, through an unrelated reorder', async () => {
    const hook = mount()
    const focused = hook.targets.get('middle')!
    focused.focus()
    act(() => {
      hook.result.current('middle', 1)
    })
    await hook.update(['last', 'middle', 'first'])
    expect(focused).toHaveFocus()
    focused.remove()
    await hook.update(['first', 'last'])
    expect(hook.targets.get('last')).toHaveFocus()
  })

  it.each([
    { remove: 'last', index: 2, remaining: ['first', 'middle'], next: 'middle' },
    { remove: 'first', index: 0, remaining: [], next: null },
  ])('chooses the preceding row or existing empty action for $remove', async (scenario) => {
    const hook = mount()
    act(() => {
      hook.result.current(scenario.remove, scenario.index)
    })
    await hook.update(scenario.remaining)
    expect(scenario.next ? hook.targets.get(scenario.next) : hook.fallback).toHaveFocus()
  })

  it('checks focus after same-commit overlay cleanup, then settles exactly once', async () => {
    const hook = mount()
    const overlay = control()
    overlay.focus()
    act(() => {
      hook.result.current('middle', 1)
    })
    await act(async () => {
      hook.rerender({ keys: ['first', 'last'] })
      overlay.remove()
    })
    expect(hook.targets.get('last')).toHaveFocus()
    hook.targets.get('last')!.blur()
    await hook.update(['first', 'last'])
    expect(document.body).toHaveFocus()
  })

  it('preserves focus elsewhere and never reclaims it on another reread', async () => {
    const hook = mount()
    const elsewhere = control()
    elsewhere.focus()
    act(() => {
      hook.result.current('middle', 1)
    })
    await hook.update(['first', 'last'])
    expect(elsewhere).toHaveFocus()
    elsewhere.blur()
    await hook.update(['last', 'first'])
    expect(document.body).toHaveFocus()
  })

  it('cancels a failed optimistic intent before a later unrelated removal', async () => {
    const hook = mount()
    let cancel = () => {}
    act(() => {
      cancel = hook.result.current('middle', 1)
    })
    act(() => cancel())
    await hook.update(['first', 'last'])
    expect(document.body).toHaveFocus()
  })

  it('does not let an older refusal cancel a newer intent', async () => {
    const hook = mount()
    let cancelFirst = () => {}
    act(() => {
      cancelFirst = hook.result.current('first', 0)
    })
    act(() => {
      hook.result.current('middle', 1)
    })
    act(() => cancelFirst())
    await hook.update(['first', 'last'])
    expect(hook.targets.get('last')).toHaveFocus()
  })

  it('does not move focus after the coordinating view unmounts', async () => {
    const hook = mount()
    act(() => {
      hook.result.current('middle', 1)
    })
    await act(async () => {
      hook.rerender({ keys: ['first', 'last'] })
      hook.unmount()
    })
    expect(document.body).toHaveFocus()
  })
})
