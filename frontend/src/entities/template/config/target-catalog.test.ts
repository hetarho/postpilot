import { describe, expect, it } from 'vitest'
import { templateTargetsI18n as i18n } from './target-i18n'
import { ROLES, TARGET_TREE, branches, leaves } from './target-catalog'

describe('the target catalog (TMPL-72)', () => {
  it('every leaf carries 아쉬운 점 and no medical target', () => {
    const all = leaves()
    expect(all.length).toBeGreaterThan(20)
    for (const { path, roles } of all) {
      expect(roles, path.join('/')).toContain('drawbacks')
      expect(new Set(roles).size, path.join('/')).toBe(roles.length)
    }
    const keys = JSON.stringify(TARGET_TREE)
    for (const medical of [
      'medical',
      'clinic',
      'dermatology',
      'plastic',
      'dental',
      'procedureClinic',
    ])
      expect(keys).not.toContain(medical)
    for (const word of ['병원', '의원', '시술 후기', '성형'])
      expect(JSON.stringify(i18n.ko.targets)).not.toContain(word)
  })

  it('keeps the six targets and the product kind and review form steps', () => {
    expect(TARGET_TREE.children!.map((child) => child.key)).toEqual([
      'restaurant',
      'cafe',
      'beauty',
      'stay',
      'activity',
      'product',
    ])
    const product = TARGET_TREE.children!.find((child) => child.key === 'product')!
    for (const kind of product.children!)
      expect(kind.children!.map((form) => form.key)).toEqual(['unboxing', 'longTerm', 'comparison'])
  })

  it('names every target, question and role in ko and en alike', () => {
    for (const language of ['ko', 'en'] as const) {
      const wizard = i18n[language]
      for (const node of leaves().flatMap((leaf) => leaf.path))
        expect(
          wizard.targets[node as keyof typeof wizard.targets],
          `${language} ${node}`,
        ).toBeDefined()
      for (const branch of branches())
        expect(
          wizard.questions[branch as keyof typeof wizard.questions],
          `${language} ${branch}`,
        ).toBeTruthy()
      for (const role of Object.keys(ROLES)) {
        const words = wizard.roles[role as keyof typeof wizard.roles]
        expect(words?.name, `${language} ${role}`).toBeTruthy()
        expect(words?.help, `${language} ${role}`).toBeTruthy()
      }
    }
    const keys = (value: object): string[] =>
      Object.entries(value).flatMap(([key, child]) =>
        typeof child === 'object' ? keys(child).map((sub) => `${key}.${sub}`) : [key],
      )
    expect(keys(i18n.en).sort()).toEqual(keys(i18n.ko).sort())
  })

  it('starts every role at a default the grammar takes', () => {
    for (const [role, defaults] of Object.entries(ROLES)) {
      expect(defaults.photos, role).toBeGreaterThanOrEqual(0)
      expect(defaults.photos, role).toBeLessThanOrEqual(3)
      if ('repeat' in defaults && defaults.repeat) expect(defaults.photos, role).toBeGreaterThan(0)
    }
  })
})
