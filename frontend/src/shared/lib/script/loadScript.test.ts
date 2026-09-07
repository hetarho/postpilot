import { describe, expect, it } from 'vitest'
import { loadScript } from './loadScript'

describe('loadScript', () => {
  it('appends a source once and shares its in-flight load', async () => {
    const src = 'https://example.test/sdk-once.js'
    const first = loadScript(src)
    const second = loadScript(src)

    const scripts = Array.from(document.scripts).filter((script) => script.src === src)
    expect(scripts).toHaveLength(1)
    scripts[0].dispatchEvent(new Event('load'))

    await expect(first).resolves.toBeUndefined()
    await expect(second).resolves.toBeUndefined()
    expect(loadScript(src)).toBe(first)
  })

  it('removes a failed insertion so a later attempt can retry', async () => {
    const src = 'https://example.test/sdk-retry.js'
    const first = loadScript(src)
    document
      .querySelector<HTMLScriptElement>(`script[src="${src}"]`)
      ?.dispatchEvent(new Event('error'))
    await expect(first).rejects.toThrow('Could not load script')
    expect(document.querySelector(`script[src="${src}"]`)).toBeNull()

    const retry = loadScript(src)
    expect(retry).not.toBe(first)
    document
      .querySelector<HTMLScriptElement>(`script[src="${src}"]`)
      ?.dispatchEvent(new Event('load'))
    await expect(retry).resolves.toBeUndefined()
  })
})
