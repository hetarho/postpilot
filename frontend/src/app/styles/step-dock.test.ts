import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const stylesheet = readFileSync(
  path.join(path.dirname(fileURLToPath(import.meta.url)), 'index.css'),
  'utf8',
)

describe('the step bar on a phone', () => {
  // ←, 스토리라인 먼저 and 바로 글 쓰기 share one row; at 24px a side the big action's label broke
  // over two lines on a 360–375px phone.
  it('gives its labels a narrower side below the sm breakpoint only', () => {
    const phone = /@media \(width < 40rem\) \{([\s\S]*?)\n {2}\}/.exec(stylesheet)?.[1] ?? ''
    expect(phone).toMatch(
      /\[data-action-bar='step'\] \.ui-button \{\s*padding-inline: calc\(var\(--spacing\) \* 4\);/,
    )
    expect(phone).toMatch(
      /\[data-action-bar='step'\] \.ui-button\.ui-button-cta \{\s*padding-inline: calc\(var\(--spacing\) \* 5\);/,
    )
    // Every other width keeps the dock's own 24px and 28px sides.
    expect(stylesheet).toMatch(
      /\[data-action-bar\] \.ui-button \{[^}]*padding-inline: calc\(var\(--spacing\) \* 6\);/,
    )
  })
})
