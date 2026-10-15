import { describe, expect, it } from 'vitest'
import { parseClipTemplate, emptyClipRecipe, validateClipRecipe } from '@/entities/clip-template'
import { STAGE_ROLES, type StageDraft } from '../model/wizard-machine'
import { toClipWizardBody } from './to-composition'

const stage = (role: StageDraft['role'], asks = false): StageDraft => ({
  id: role,
  role,
  name: role,
  heading: `내 ${role}`,
  intent: '대상 & 내가 느낀 인상',
  checked: true,
  asks,
  fieldLabel: '가격 & 위치',
})
describe('an ordinary clip template from the wizard', () => {
  it.each([...STAGE_ROLES, 'custom' as const])(
    'parses %s with supplied information on and off',
    (role) => {
      for (const asks of [false, true]) {
        const body = toClipWizardBody([stage(role, asks)])
        const parsed = parseClipTemplate(body)
        expect(parsed.stages).toMatchObject([
          { name: `내 ${role}`, intent: '대상 & 내가 느낀 인상' },
        ])
        expect(parsed.fields).toHaveLength(asks ? 1 : 0)
        if (asks) {
          expect(parsed.fields[0].label).toBe('가격 & 위치')
          expect(parsed.elements[0].kind).toBe('fixed')
          expect(parsed.elements[0].role).toBe(
            role === 'hook' || role === 'ending' ? role : 'caption',
          )
        } else if (role === 'hook' || role === 'ending') {
          expect(parsed.elements[0]).toMatchObject({ kind: 'ai', role })
          expect(parsed.elements[0].rows[0].parts[0].literal).toBe('대상 & 내가 느낀 인상')
        }
        expect(
          validateClipRecipe({ ...emptyClipRecipe(), name: '내 영상', compositionBody: body })
            .valid,
        ).toBe(true)
      }
    },
  )
  it('keeps selected order and escapes names and meanings without making them markup', () => {
    const entries = [
      stage('ending'),
      { ...stage('info', true), checked: false },
      { ...stage('custom'), heading: '“내” <부분>', intent: '<stage>가 아니라 내용 & 인상' },
      stage('hook'),
    ]
    const parsed = parseClipTemplate(toClipWizardBody(entries))
    expect(parsed.stages.map((value) => value.name)).toEqual([
      '내 ending',
      '“내” <부분>',
      '내 hook',
    ])
    expect(parsed.stages[1].intent).toBe('<stage>가 아니라 내용 & 인상')
    expect(parsed.fields).toEqual([])
  })
})
