import { describe, expect, it } from 'vitest'

import * as sharedConfig from './index'

/** ARCH-21 / T258: `shared/config` holds env-derived and cross-slice values only.
 *  A key naming one domain is a product limit or a slice's own UI tuning, and
 *  belongs in that slice's `config` segment — one file every slice edits is a
 *  merge hotspot (review/arch-260919 F4). */
describe('shared/config', () => {
  it('exports no key that names a single domain', () => {
    const owned = Object.keys(sharedConfig).filter((key) =>
      /^(CLIP|POST|TEMPLATE|PROMO|LISTBOX|POPOVER)_/.test(key),
    )
    expect(owned).toEqual([])
  })

  it('reads every VITE_* override through one channel', () => {
    expect(Object.keys(sharedConfig.ENV_LIMIT_OVERRIDES).length).toBeGreaterThan(0)
    expect(sharedConfig.positiveIntEnv('12', 4)).toBe(12)
    expect(sharedConfig.positiveIntEnv(undefined, 4)).toBe(4)
    expect(sharedConfig.positiveIntEnv('0', 4)).toBe(4)
    expect(sharedConfig.positiveIntEnv('nope', 4)).toBe(4)
  })
})
