#!/usr/bin/env node
// Fails when frontend code escapes the design tokens (spec/tech/design-language.md §2).
//
// Two escapes are caught:
//   1. A Tailwind STOCK colour utility (`bg-gray-100`, `text-red-400`, `border-zinc-800/50`).
//      The stock palette is removed in app/styles/index.css, so such a class silently emits no CSS
//      — the UI degrades without an error. This is the only gate that notices.
//   2. A raw colour literal (`#fff`, `rgb(…)`, `oklch(…)`, `hsl(…)`) or an arbitrary colour
//      utility (`bg-[#…]`, `text-[oklch(…)]`) anywhere under frontend/src except the token file.
//
// A line that is not UI colour at all — a canvas compositing fill, a test fixture — opts out with
// an inline `// style-escape: <why>` pragma on the same line. The reason is mandatory and is
// what a reviewer reads.
//
// Usage: node scripts/lint-style-escapes.mjs   (wired as `pnpm lint:style`)

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const ROOT = join(import.meta.dirname, '..', 'frontend', 'src')
const TOKEN_FILE = join(ROOT, 'app', 'styles', 'index.css')
const EXT = new Set(['.ts', '.tsx', '.css'])

const STOCK_HUES =
  'slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black'
const STOCK_UTILITY = new RegExp(
  `(?<![\\w-])(?:[\\w-]+:)*(?:bg|text|border|ring|outline|shadow|from|via|to|fill|stroke|accent|caret|decoration|divide|placeholder)-(?:${STOCK_HUES})(?:-\\d{2,3})?(?:/\\d{1,3})?(?![\\w-])`,
  'g',
)
const RAW_COLOUR = /(?<![\w-])(?:#[0-9a-fA-F]{3,8}\b|(?:rgba?|hsla?|oklch|oklab|color)\()/g
const ARBITRARY_COLOUR_UTILITY = /(?:bg|text|border|ring|outline|shadow|fill|stroke)-\[(?:#|rgba?\(|hsla?\(|oklch\(|oklab\()/g

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (name === 'gen' || name === 'node_modules') continue
    if (statSync(p).isDirectory()) walk(p, out)
    else if (EXT.has(p.slice(p.lastIndexOf('.')))) out.push(p)
  }
  return out
}

const findings = []
for (const file of walk(ROOT)) {
  const isTokenFile = file === TOKEN_FILE
  const lines = readFileSync(file, 'utf8').split('\n')
  lines.forEach((line, i) => {
    const trimmed = line.trim()
    if (/\/\/\s*style-escape:\s*\S/.test(line)) return
    // The token file may name colours; everywhere else, comments are not exempt (they get copied).
    if (isTokenFile && (trimmed.startsWith('/*') || trimmed.startsWith('*') || trimmed.startsWith('--'))) return
    const report = (kind, m) =>
      findings.push(`${relative(process.cwd(), file)}:${i + 1}  ${kind}: ${m}`)
    for (const m of line.matchAll(STOCK_UTILITY)) report('stock colour utility', m[0])
    if (!isTokenFile) {
      for (const m of line.matchAll(RAW_COLOUR)) report('raw colour literal', m[0])
      for (const m of line.matchAll(ARBITRARY_COLOUR_UTILITY)) report('arbitrary colour utility', m[0])
    }
  })
}

if (findings.length) {
  console.error(`lint:style — ${findings.length} escape(s) from the design tokens:\n`)
  for (const f of findings) console.error('  ' + f)
  console.error(
    '\nUse a semantic role (bg-surface, text-text-muted, text-danger, …). Roles are defined in',
    'frontend/src/app/styles/index.css; the rules are in spec/tech/design-language.md §2.',
  )
  process.exit(1)
}
console.log('lint:style — ok (no colour escapes)')
