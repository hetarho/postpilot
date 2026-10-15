import { expect, it } from 'vitest'
import { templateTargetLeaves, templateTargetsI18n } from '@/entities/template'
import { CLIP_COMPOSITION_LIMITS } from '@/entities/clip-design'
import { parseClipTemplate } from '@/entities/clip-template'
import { clipStageSeeds, subjectOf } from './catalog'
import { i18n } from './i18n'
import { STAGE_ROLES } from '../model/wizard-machine'
import { toClipWizardBody } from '../lib/to-composition'

const fill = (text: string, values: Record<string, string>) =>
  text.replace(/\{\{(\w+)\}\}/g, (_, key: string) => values[key] ?? '')
it('gives every target leaf the seven valid localized stages and explicit information', () => {
  for (const language of ['ko', 'en'] as const)
    for (const { path } of templateTargetLeaves()) {
      const copy = i18n[language],
        targetCopy = templateTargetsI18n[language]
      const target = path
        .map((key) => targetCopy.targets[key as keyof typeof targetCopy.targets].name)
        .join(' · ')
      const subject = copy.subjects[subjectOf(path) as keyof typeof copy.subjects]
      expect(subject, `${language} ${path.join('/')}`).toBeTruthy()
      const values = {
        target,
        subject,
        where: path[0] === 'product' ? copy.whereProduct : copy.wherePlace,
      }
      const labels = Object.fromEntries(
        STAGE_ROLES.map((role) => [
          role,
          {
            name: fill(copy.roles[role].name, values),
            intent: fill(copy.roles[role].intent, values),
          },
        ]),
      ) as Parameters<typeof clipStageSeeds>[0]
      const stages = clipStageSeeds(
        labels,
        path[0] === 'product' ? copy.fieldProduct : copy.fieldPlace,
      )
      expect(stages.map((stage) => stage.role)).toEqual(STAGE_ROLES)
      expect(stages.every((stage) => stage.checked)).toBe(true)
      expect(stages.filter((stage) => stage.asks).map((stage) => stage.role)).toEqual(['info'])
      for (const stage of stages) {
        expect(Array.from(stage.heading).length).toBeLessThanOrEqual(
          CLIP_COMPOSITION_LIMITS.labelChars,
        )
        expect(Array.from(stage.intent).length).toBeLessThanOrEqual(
          CLIP_COMPOSITION_LIMITS.promptChars,
        )
        expect(stage.intent).not.toContain('{{')
      }
      expect(parseClipTemplate(toClipWizardBody(stages)).stages).toHaveLength(7)
    }
})
