import { QueryClient } from '@tanstack/react-query'
import { isRedirect } from '@tanstack/react-router'
import { expect, it, vi } from 'vitest'
import { redirectLegacyVoice } from './voices'
import type { RouterContext } from './tree'

vi.mock('@/entities/voice', () => ({
  loadVoices: (...args: unknown[]) => loadVoices(...args),
  defaultVoice: (voices: { id: string; isDefault?: boolean }[]) =>
    voices.find((voice) => voice.isDefault) ?? voices[0],
}))
let loadVoices: (...args: unknown[]) => Promise<unknown> = async () => []

function context(): RouterContext & { user: { id: string } } {
  return {
    queryClient: new QueryClient(),
    transport: {} as RouterContext['transport'],
    user: { id: 'alice' },
  }
}

/** Where a `throw redirect(...)` was pointing, with its params filled in. */
async function target(tab: string) {
  try {
    await redirectLegacyVoice(context(), tab)
  } catch (thrown) {
    if (!isRedirect(thrown)) throw thrown
    const { to = '', params = {} } = thrown.options as {
      to?: string
      params?: Record<string, string>
    }
    return Object.entries(params).reduce((path, [key, value]) => path.replace(`$${key}`, value), to)
  }
  throw new Error('redirectLegacyVoice must always redirect')
}

it('sends every legacy tab to the same tab of the default voice', async () => {
  loadVoices = async () => [
    { id: 'voice-a', name: '가' },
    { id: 'voice-b', name: '나', isDefault: true },
  ]
  // The account's actual default, not the first voice, and nothing is created on the way.
  expect(await target('')).toBe('/voices/voice-b')
  expect(await target('versions')).toBe('/voices/voice-b/versions')
  expect(await target('import')).toBe('/voices/voice-b/import')
  expect(await target('rules')).toBe('/voices/voice-b/rules')
  expect(await target('validations')).toBe('/voices/voice-b/validations')
  // A tab that never existed lands on the profile rather than on an empty screen.
  expect(await target('whatever')).toBe('/voices/voice-b')
})

it('sends an account with nothing to show to the directory', async () => {
  loadVoices = async () => []
  expect(await target('rules')).toBe('/voices')
  // An outage is not a reason to invent a voice: the directory is what answers for it.
  loadVoices = async () => {
    throw new Error('offline')
  }
  expect(await target('')).toBe('/voices')
})
