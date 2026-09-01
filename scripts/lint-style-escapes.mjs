#!/usr/bin/env node
// Fails when frontend code escapes the design tokens (spec/tech/design-language.md §2, §3).
//
// Four escapes are caught:
//   1. A Tailwind STOCK colour utility (`bg-gray-100`, `text-red-400`, `border-zinc-800/50`).
//      The stock palette is removed in app/styles/index.css, so such a class silently emits no CSS
//      — the UI degrades without an error. This is the only gate that notices.
//   2. A RETIRED design-token utility (`bg-bg`, `text-text-muted`, `divide-border`). These aliases
//      were removed after the functional-role migration and must not return.
//   3. A raw colour literal (`#fff`, `rgb(…)`, `oklch(…)`, `hsl(…)`) or an arbitrary colour
//      utility (`bg-[#…]`, `text-[oklch(…)]`) anywhere under frontend/src except the token file.
//   4. An ad-hoc TYPE utility — a text size, font weight/family, tracking, or leading — in a
//      non-test `.tsx` outside `shared/ui`. The §3 type roles live in the Typography primitive
//      (`shared/ui/typography`); a raw recipe in a slice is hierarchy drift by construction.
//      Alignment/wrapping utilities (`text-center`, `text-balance`, …) are not type and pass.
//
// A line that is not UI colour/type at all — a canvas compositing fill, a test fixture, a control
// state the §3 roles do not model — opts out with an inline `// style-escape: <why>` pragma on
// the same line. The reason is mandatory and is what a reviewer reads.
//
// Usage: node scripts/lint-style-escapes.mjs           (wired as `pnpm lint:style`)
//        node scripts/lint-style-escapes.mjs --probe   (scanner self-test)

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, sep } from 'node:path'

const ROOT = join(import.meta.dirname, '..', 'frontend', 'src')
const TOKEN_FILE = join(ROOT, 'app', 'styles', 'index.css')
const EXT = new Set(['.ts', '.tsx', '.css'])

const STOCK_HUES =
  'slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black'
const STOCK_UTILITY = new RegExp(
  `(?<![\\w-])(?:[\\w-]+:)*(?:bg|text|border|ring|outline|shadow|from|via|to|fill|stroke|accent|caret|decoration|divide|placeholder)-(?:${STOCK_HUES})(?:-\\d{2,3})?(?:/\\d{1,3})?(?![\\w-])`,
  'g',
)
const RETIRED_ROLES = [
  'surface-hover',
  'surface',
  'text-muted',
  'text-subtle',
  'text-faint',
  'text',
  'border-strong',
  'border',
  'primary-hover',
  'primary-foreground',
  'primary-surface',
  'primary',
  'danger-foreground',
  'danger-surface',
  'danger',
  'success-foreground',
  'success-surface',
  'success',
  'warning-foreground',
  'warning-surface',
  'warning',
  'info-foreground',
  'info-surface',
  'info',
  'overlay',
  'depth',
  'bg',
].join('|')
const RETIRED_UTILITY = new RegExp(
  `(?<![\\w-])(?:[\\w-]+:)*(?:bg|text|border|ring|outline|shadow|from|via|to|fill|stroke|accent|caret|decoration|divide|placeholder)-(?:${RETIRED_ROLES})(?:/\\d{1,3})?(?![\\w-])`,
  'g',
)
const RAW_COLOUR = /(?<![\w-])(?:#[0-9a-fA-F]{3,8}\b|(?:rgba?|hsla?|oklch|oklab|color)\()/g
const ARBITRARY_COLOUR_UTILITY =
  /(?:bg|text|border|ring|outline|shadow|fill|stroke)-\[(?:#|rgba?\(|hsla?\(|oklch\(|oklab\()/g
// Sizes, weights/families, tracking, leading — the §3 recipe vocabulary. The open named forms
// catch Tailwind theme extensions (`text-display`, `font-brand`, `tracking-display`, …); the
// negative lookahead keeps alignment/wrap and the project's semantic colour roles out. Arbitrary
// text sizes recognise numeric lengths, CSS size keywords and math spellings while colour
// arbitraries stay finding #3 because they begin with a colour literal/function instead.
const TYPE_UTILITY = new RegExp(
  '(?<![\\w-])(?:[\\w-]+:)*(?:' +
    'text-(?:xs|sm|base|lg|[2-9]?xl)' +
    '|text-(?!(?:(?:left|center|right|justify|start|end|wrap|nowrap|balance|pretty|ellipsis|clip|transparent|current|inherit)(?![\\w-])|(?:badge|button|content|field|link|media|notice)-))[a-z][\\w-]*' +
    '|text-\\[(?:\\d|\\.|length:|xx-small|x-small|small|medium|large|x-large|xx-large|xxx-large|larger|smaller|math|clamp\\(|calc\\(|min\\(|max\\(|round\\(|mod\\(|rem\\(|sin\\(|cos\\(|tan\\(|asin\\(|acos\\(|atan\\(|atan2\\(|pow\\(|sqrt\\(|hypot\\(|log\\(|exp\\(|abs\\(|sign\\()[^\\]]*\\]' +
    '|text-\\(length:[^)]*\\)' +
    '|font-(?:[a-z][\\w-]*|\\[[^\\]]*\\]|\\([^)]*\\))' +
    '|-?tracking-(?:[a-z0-9][\\w-]*|\\[[^\\]]*\\]|\\([^)]*\\))' +
    '|leading-(?:[a-z0-9][\\w-]*|\\[[^\\]]*\\]|\\([^)]*\\))' +
    ')(?![\\w-])',
  'g',
)

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (name === 'gen' || name === 'node_modules') continue
    if (statSync(p).isDirectory()) walk(p, out)
    else if (EXT.has(p.slice(p.lastIndexOf('.')))) out.push(p)
  }
  return out
}

function scanLine(line, { isTokenFile = false, isTypeGated = false } = {}) {
  const findings = []
  const trimmed = line.trim()
  if (/\/\/\s*style-escape:\s*\S/.test(line)) return findings
  // The token file may name colours; everywhere else, comments are not exempt (they get copied).
  if (
    isTokenFile &&
    (trimmed.startsWith('/*') || trimmed.startsWith('*') || trimmed.startsWith('--'))
  ) {
    return findings
  }
  const report = (kind, match) => findings.push({ kind, match })
  for (const match of line.matchAll(STOCK_UTILITY)) report('stock colour utility', match[0])
  for (const match of line.matchAll(RETIRED_UTILITY)) report('retired token utility', match[0])
  if (!isTokenFile) {
    for (const match of line.matchAll(RAW_COLOUR)) report('raw colour literal', match[0])
    for (const match of line.matchAll(ARBITRARY_COLOUR_UTILITY)) {
      report('arbitrary colour utility', match[0])
    }
  }
  if (isTypeGated) {
    for (const match of line.matchAll(TYPE_UTILITY)) report('ad-hoc type utility', match[0])
  }
  return findings
}

function runProbe() {
  const typeCases = [
    { source: 'text-sm', gated: true },
    { source: 'sm:text-lg', gated: true },
    { source: 'text-3xl', gated: true },
    { source: 'text-display', gated: true },
    { source: 'text-[10px]', gated: true },
    { source: 'text-[.875rem]', gated: true },
    { source: 'text-[medium]', gated: true },
    { source: 'text-[larger]', gated: true },
    { source: 'text-[length:var(--type-size)]', gated: true },
    { source: 'text-[clamp(1rem,2vw,2rem)]', gated: true },
    { source: 'text-(length:--type-size)', gated: true },
    { source: 'font-medium', gated: true },
    { source: 'font-mono', gated: true },
    { source: 'font-[650]', gated: true },
    { source: 'font-[var(--weight)]', gated: true },
    { source: 'font-(--weight)', gated: true },
    { source: 'font-(family-name:--font-family)', gated: true },
    { source: 'font-brand', gated: true },
    { source: 'tracking-tight', gated: true },
    { source: '-tracking-tight', gated: true },
    { source: '-tracking-[0.02em]', gated: true },
    { source: 'tracking-(--letter-spacing)', gated: true },
    { source: 'tracking-display', gated: true },
    { source: 'leading-relaxed', gated: true },
    { source: 'leading-(--line-height)', gated: true },
    { source: 'leading-golden', gated: true },
    { source: 'text-[var(--font-size)]', gated: false },
    { source: 'text-center', gated: false },
    { source: 'text-balance', gated: false },
    { source: 'text-content-secondary', gated: false },
    { source: 'text-field-error', gated: false },
    { source: 'text-button-cta-fg', gated: false },
    { source: 'text-badge-accent-fg', gated: false },
    { source: 'text-media-scrim-fg', gated: false },
    { source: 'text-link-fg', gated: false },
    { source: 'text-notice-danger-fg', gated: false },
    { source: 'text-transparent', gated: false },
  ]
  const typeFailures = typeCases.filter(({ source, gated }) => {
    const inSlice = scanLine(source, { isTypeGated: true }).some(
      ({ kind }) => kind === 'ad-hoc type utility',
    )
    // shared/ui owns the recipes, so the same line must pass without the flag.
    const inSharedUi = scanLine(source).some(({ kind }) => kind === 'ad-hoc type utility')
    return inSlice !== gated || inSharedUi
  })
  if (typeFailures.length) {
    for (const failure of typeFailures) {
      console.error(`lint:style:probe — ${failure.source} expected type-gated=${failure.gated}`)
    }
    process.exit(1)
  }

  const cases = [
    { source: 'bg-bg', retired: true },
    { source: 'hover:bg-surface-hover/60', retired: true },
    { source: 'text-text-muted', retired: true },
    { source: 'focus:text-primary-foreground', retired: true },
    { source: 'divide-border', retired: true },
    { source: 'shadow-depth', retired: true },
    { source: 'placeholder:text-text-faint', retired: true },
    { source: 'bg-danger-surface', retired: true },
    { source: 'ring-success', retired: true },
    { source: 'outline-warning', retired: true },
    { source: 'fill-info', retired: true },
    { source: 'bg-surface-base', retired: false },
    { source: 'text-content-primary', retired: false },
    { source: 'bg-button-cta-bg', retired: false },
    { source: 'text-notice-danger-fg', retired: false },
    { source: 'divide-divider', retired: false },
  ]

  const failures = cases.filter(({ source, retired }) => {
    const detected = scanLine(source).some(({ kind }) => kind === 'retired token utility')
    return detected !== retired
  })
  if (failures.length) {
    for (const failure of failures) {
      console.error(`lint:style:probe — ${failure.source} expected retired=${failure.retired}`)
    }
    process.exit(1)
  }
  console.log(`lint:style:probe — ok (${cases.length + typeCases.length} scanner cases)`)
}

if (process.argv.includes('--probe')) {
  runProbe()
  process.exit(0)
}

const SHARED_UI_DIR = join(ROOT, 'shared', 'ui') + sep

const findings = []
for (const file of walk(ROOT)) {
  const isTokenFile = file === TOKEN_FILE
  // Type recipes live in shared/ui (Typography and the other primitives own them); everywhere
  // else a .tsx renders text through those primitives. Tests assert on recipe classes, so they
  // stay out of the type gate (they still go through the colour gates above).
  const isTypeGated =
    file.endsWith('.tsx') && !file.endsWith('.test.tsx') && !file.startsWith(SHARED_UI_DIR)
  const lines = readFileSync(file, 'utf8').split('\n')
  lines.forEach((line, index) => {
    for (const { kind, match } of scanLine(line, {
      isTokenFile,
      isTypeGated,
    })) {
      findings.push(`${relative(process.cwd(), file)}:${index + 1}  ${kind}: ${match}`)
    }
  })
}

if (findings.length) {
  console.error(`lint:style — ${findings.length} escape(s) from the design tokens:\n`)
  for (const f of findings) console.error('  ' + f)
  console.error(
    '\nUse a functional role (bg-button-cta-bg, text-notice-danger-fg) or a compositional',
    'foundation (bg-surface-base, text-content-secondary). Roles are defined in',
    'frontend/src/app/styles/index.css; the rules are in spec/tech/design-language.md §2.',
    '\nFor an ad-hoc type utility, render the text through shared/ui Typography (or',
    'typographyStyles for a self-semantic element) — the type roles are design-language §3.',
  )
  process.exit(1)
}
console.log('lint:style — ok (no colour or type escapes)')
