import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { TEMPLATE_BODY_MAX_CHARS, TEMPLATE_PHOTO_ROW_MAX } from '@/shared/config'
import { PARSE_REASONS, parse } from '../lib/grammar'
import { GUIDE_EXAMPLE_BODY, formatGuide } from './guide'

afterEach(() => initializeI18n('ko'))

/** The five constructs a person authors today, as the guide has to teach them. Derived from the
 *  grammar rather than from the guide's prose (TEMPLATE-41): when TEMPLATE-18 changes, this list
 *  and the shared fixture both fail until the guide follows. */
const CONSTRUCTS = [
  '<write>',
  '</write>',
  '<slot kind="photo"',
  'count="',
  '<repeat each="photo">',
  '</repeat>',
  '<note>',
  '</note>',
]

/** Retired in r2 and never taught again: a guide that mentioned one would have an outside AI
 *  write a body this app parses but the builder immediately rewrites (TEMPLATE-37). */
const RETIRED = ['place', 'link', 'label=']

describe.each(['ko', 'en'] as const)('the format guide in %s', (language) => {
  const guide = () => {
    initializeI18n(language)
    return formatGuide()
  }

  // The example is a BODY, and this is what stops it drifting from the grammar it teaches.
  it('carries an example the real parser accepts', () => {
    const result = parse(GUIDE_EXAMPLE_BODY, { photoRowMax: TEMPLATE_PHOTO_ROW_MAX })
    expect(result.ok).toBe(true)
    expect(guide()).toContain(GUIDE_EXAMPLE_BODY)
  })

  it('teaches every construct a person can author', () => {
    const text = guide()
    for (const construct of CONSTRUCTS) {
      expect(text, construct).toContain(construct)
    }
  })

  it('states both ceilings as the numbers they actually are', () => {
    const text = guide()
    expect(text).toContain(String(TEMPLATE_BODY_MAX_CHARS))
    expect(text).toContain(String(TEMPLATE_PHOTO_ROW_MAX))
  })

  it('teaches no retired construct and leaks no unresolved placeholder', () => {
    const text = guide()
    for (const word of RETIRED) {
      expect(text, word).not.toContain(word)
    }
    // `escapeValue: false` is what keeps the example's `<` a `<`; a missing interpolation would
    // hand the AI a literal `{{example}}`.
    expect(text).not.toContain('{{')
    expect(text).not.toContain('&lt;write')
  })
})

// A reason with no copy renders its own key at the user, which is how a new grammar rule ships
// half-translated. The source mode is the surface that shows one.
describe.each(['ko', 'en'] as const)('every parse reason has copy in %s', (language) => {
  it('renders a sentence rather than a key', async () => {
    const i18n = initializeI18n(language)
    for (const reason of PARSE_REASONS) {
      const text = i18n.t(`builder.reasons.${reason}`, { ns: 'templates', max: 4 })
      expect(text, reason).not.toBe(`builder.reasons.${reason}`)
      expect(text, reason).not.toBe('')
    }
  })
})
