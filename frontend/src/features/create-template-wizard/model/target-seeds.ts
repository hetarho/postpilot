import { useTranslation } from 'react-i18next'
import { TEMPLATE_ROLE_DEFAULTS, type RoleKey, type TargetNode } from '@/entities/template'
import type { SectionDraft } from './wizard-machine'

/** The leaf's sections for a target answer, resolved in the owner's language, which the wizard's
 *  다음 hands over with the answer; undefined for an answer that asks another question. */
export function useTargetSeeds(): (child: TargetNode) => SectionDraft[] | undefined {
  const { t } = useTranslation('templateTargets')
  return (child) =>
    child.roles?.map((role: RoleKey) => {
      const name = t(`roles.${role}.name`)
      const defaults = TEMPLATE_ROLE_DEFAULTS[role] as {
        content: SectionDraft['content']
        photos: number
        repeat?: boolean
      }
      return {
        id: role,
        role,
        name,
        help: t(`roles.${role}.help`),
        checked: true,
        heading: name,
        content: defaults.content,
        question: name,
        required: false,
        literal: '',
        photos: defaults.photos,
        repeatable: defaults.repeat !== undefined,
        repeat: defaults.repeat ?? false,
      }
    })
}
