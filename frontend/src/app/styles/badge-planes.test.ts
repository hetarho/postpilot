import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const stylesheet = readFileSync(
  path.join(path.dirname(fileURLToPath(import.meta.url)), 'index.css'),
  'utf8',
)

/** The surface a role token is declared to sit on. */
function plane(role: string) {
  return new RegExp(`${role}:\\s*var\\((--surface-[a-z]+)\\)`).exec(stylesheet)?.[1]
}

describe('neutral chip plane', () => {
  // Guide sets every list row on a raised card. A neutral chip in that same raised plane had no
  // edge: 초안 read as loose text and the draft tile as a glyph with no square under it.
  it('sits in a plane apart from the card, the hovered card and the page', () => {
    const chip = plane('--badge-neutral-bg')
    expect(chip).toBe('--surface-recessed')
    for (const role of ['--card-bg', '--card-bg-hover']) expect(plane(role)).not.toBe(chip)
  })
})
