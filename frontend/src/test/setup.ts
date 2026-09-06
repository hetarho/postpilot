import '@testing-library/jest-dom/vitest'
import { configure } from '@testing-library/react'
import { initializeI18n } from '@/app/providers/i18n'

initializeI18n('ko')

// The suite runs 70+ files in parallel, so a page-level `findBy*` can exceed testing-library's
// 1s default from scheduling pressure alone rather than from anything being wrong — PostsPage's
// first render did, on roughly half of full-suite runs, long before this line existed. Only the
// deadline moves; every assertion stays exactly as strict, and a genuinely broken query still
// fails, just later.
configure({ asyncUtilTimeout: 5_000 })

// jsdom has no layout engine, so every router navigation would otherwise log
// "Not implemented: Window's scrollTo()" and bury the real test output.
window.scrollTo = () => {}

// jsdom implements no ResizeObserver, and the virtualized catalog list measures each mounted row
// with one. A no-op is the honest stand-in rather than a fake measurement: with no layout engine
// every element is 0×0, so the virtualizer keeps its estimated sizes and still renders the rows
// a test asserts on — which is what those tests are about, not pixel geometry.
class NoopResizeObserver implements ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= NoopResizeObserver

// Node 26 installs its own `localStorage` global whose getter returns undefined unless the
// process was started with --localstorage-file, and in vitest's jsdom environment `globalThis`
// IS the window, so that getter takes the place jsdom's storage would occupy — a bare
// `localStorage` in app code reads undefined here while it works in a browser and on CI's
// pinned Node 24. Installing a spec-shaped in-memory Storage makes every runner agree; jsdom's
// own implementation is in-memory per origin too, so nothing about the tests changes.
class MemoryStorage implements Storage {
  #entries = new Map<string, string>()
  get length() {
    return this.#entries.size
  }
  key(index: number) {
    return [...this.#entries.keys()][index] ?? null
  }
  getItem(key: string) {
    return this.#entries.get(String(key)) ?? null
  }
  setItem(key: string, value: string) {
    this.#entries.set(String(key), String(value))
  }
  removeItem(key: string) {
    this.#entries.delete(String(key))
  }
  clear() {
    this.#entries.clear()
  }
}

for (const name of ['localStorage', 'sessionStorage'] as const) {
  if (globalThis[name] === undefined) {
    Object.defineProperty(globalThis, name, {
      value: new MemoryStorage(),
      configurable: true,
      writable: true,
    })
  }
}
