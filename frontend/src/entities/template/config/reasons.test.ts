import { describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { PARSE_REASONS } from '../lib/grammar'

// A reason with no copy renders its own key at the user, which is how a new grammar rule ships
// half-translated. The source mode is the surface that shows one.
describe.each(['ko', 'en'] as const)('every parse reason has copy in %s', (language) => {
  it('renders a sentence rather than a key', () => {
    const i18n = initializeI18n(language)
    for (const reason of PARSE_REASONS) {
      const text = i18n.t(`builder.reasons.${reason}`, { ns: 'templates', max: 4, askMax: 3 })
      expect(text, reason).not.toBe(`builder.reasons.${reason}`)
      expect(text, reason).not.toBe('')
      // A reason whose number never arrived reads as `{{max}}` at the user.
      expect(text, reason).not.toContain('{{')
    }
  })
})
