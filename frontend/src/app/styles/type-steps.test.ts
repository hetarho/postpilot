import { readFileSync, readdirSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { TYPE_STEPS, twMerge } from '@/shared/lib'
import { typographyStyles, type TypographyVariant } from '@/shared/ui'

const here = path.dirname(fileURLToPath(import.meta.url))
const stylesheet = readFileSync(path.join(here, 'index.css'), 'utf8')
const src = path.resolve(here, '../..')

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const file = path.join(dir, name)
    if (statSync(file).isDirectory()) return sources(file)
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [file] : []
  })
}

// THEME-11 →THEME-19: the Guide's type steps are `--text-*` tokens in the stylesheet, and the
// call sites name them; nothing sizes text with an arbitrary value any more.
describe('the Guide type steps', () => {
  it('registers exactly the steps tailwind-merge is told about, none below 12px', () => {
    const theme = /\n@theme \{([\s\S]*?)\n\}/.exec(stylesheet)?.[1] ?? ''
    const steps = [...theme.matchAll(/^\s*--text-([a-z-]+?):\s*([\d.]+)rem;/gm)]
    expect(steps.map(([, name]) => name)).toEqual([...TYPE_STEPS])
    for (const [, , rem] of steps) expect(Number(rem) * 16).toBeGreaterThanOrEqual(12)
  })

  it('sizes no text with an arbitrary value in frontend source', () => {
    const offenders = sources(src).filter((file) =>
      /(?<![\w-])text-\[(?:\d|\.)/.test(readFileSync(file, 'utf8')),
    )
    expect(offenders.map((file) => path.relative(src, file))).toEqual([])
  })

  // Stock tailwind-merge reads `text-meta` as a colour, so a caller's colour erased the size.
  it('keeps every role its size when a caller adds a colour', () => {
    const roles: TypographyVariant[] = [
      'largeTitle',
      'display',
      'stepTitle',
      'title',
      'fieldTitle',
      'body',
      'label',
      'meta',
      'figure',
      'groupLabel',
      'eyebrow',
    ]
    for (const variant of roles) {
      const alone = typographyStyles({ variant }).split(' ')
      const coloured = typographyStyles({ variant, className: 'text-content-secondary' })
      const sizes = alone.filter((name) => /^(sm:)?text-(?!content-)/.test(name))
      expect(sizes.length).toBeGreaterThan(0)
      for (const size of sizes) expect(coloured.split(' ')).toContain(size)
    }
    expect(twMerge('text-meta', 'text-body')).toBe('text-body')
  })

  it('reaches tailwind-merge only through the configured instance', () => {
    const direct = sources(src).filter(
      (file) =>
        !file.endsWith(path.join('shared', 'lib', 'tw-merge.ts')) &&
        /from 'tailwind-merge'/.test(readFileSync(file, 'utf8')),
    )
    expect(direct.map((file) => path.relative(src, file))).toEqual([])
  })
})
