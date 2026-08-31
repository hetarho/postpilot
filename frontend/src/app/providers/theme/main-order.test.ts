import { beforeEach, describe, expect, it, vi } from 'vitest'
import indexHtml from '../../../../index.html?raw'

const runtime = vi.hoisted(() => ({
  calls: [] as string[],
  themeAtCreateRoot: undefined as string | undefined,
}))

vi.mock('@/app/providers/theme', () => ({
  bootstrapTheme: () => {
    runtime.calls.push('bootstrap-theme')
    document.documentElement.dataset.theme = 'night'
    return {
      preference: 'system',
      effectiveTheme: 'night',
      prefersDark: true,
    }
  },
}))

vi.mock('@/app/providers/i18n', () => ({
  initializeI18n: () => runtime.calls.push('initialize-i18n'),
}))

vi.mock('@/app', () => ({ App: () => null }))

vi.mock('react-dom/client', () => ({
  createRoot: () => {
    runtime.calls.push('create-root')
    runtime.themeAtCreateRoot = document.documentElement.dataset.theme
    return { render: () => runtime.calls.push('render') }
  },
}))

describe('application entrypoint', () => {
  beforeEach(() => {
    vi.resetModules()
    runtime.calls.length = 0
    runtime.themeAtCreateRoot = undefined
    document.documentElement.removeAttribute('data-theme')
    document.body.innerHTML = '<div id="root"></div>'
  })

  it('completes synchronous theme bootstrap before React creates its root', async () => {
    await import('@/main')

    expect(runtime.calls).toEqual(['bootstrap-theme', 'initialize-i18n', 'create-root', 'render'])
    expect(runtime.themeAtCreateRoot).toBe('night')
  })

  it('loads the single application module from a render-blocking head script', () => {
    const entryDocument = new DOMParser().parseFromString(indexHtml, 'text/html')
    const entryScript = entryDocument.querySelector<HTMLScriptElement>(
      'script[src="/src/main.tsx"]',
    )

    expect(entryScript).not.toBeNull()
    expect(entryScript?.parentElement).toBe(entryDocument.head)
    expect(entryScript?.getAttribute('type')).toBe('module')
    expect(entryScript?.getAttribute('blocking')).toBe('render')
    expect(entryDocument.body.querySelector('script[src="/src/main.tsx"]')).toBeNull()
  })
})
