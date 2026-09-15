// Bakes the generated preview SVGs into PNGs with the two bundled faces, the
// way the export pipeline hands them to resvg. Paths resolve from this file, so
// it runs from anywhere:
//   node <repo>/tmp/gallery/render.mjs
import { Resvg } from '@resvg/resvg-js'
import { readFileSync, writeFileSync, readdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const dir = dirname(fileURLToPath(import.meta.url))
const fontDir = resolve(dir, '../../backend/assets/fonts')
const fonts = [
  join(fontDir, 'pretendard/PretendardVariable.ttf'),
  join(fontDir, 'paperlogy/Paperlogy-8ExtraBold.ttf'),
]
const svgs = readdirSync(dir).filter((f) => f.endsWith('.svg'))
if (svgs.length === 0) {
  console.error('no SVG in', dir, '— run the generator test first (see README.md)')
  process.exit(1)
}
for (const name of svgs) {
  const r = new Resvg(readFileSync(join(dir, name), 'utf8'), {
    font: { fontFiles: fonts, loadSystemFonts: false, defaultFontFamily: 'Pretendard Variable' },
    fitTo: { mode: 'width', value: 540 },
  })
  writeFileSync(join(dir, name.replace(/\.svg$/, '.png')), r.render().asPng())
  console.log('rendered', name)
}
