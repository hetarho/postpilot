import type { Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import { ChoiceButton } from '@/shared/ui'
import {
  TEMPLATE_ROLE_DEFAULTS,
  templateTargetAt,
  type RoleKey,
  type TargetNode,
} from '@/entities/template'
import type { SectionDraft, WizardEvent, WizardState } from '../model/wizard-machine'

/** One narrowing question (TMPL-72): the choices are peers, and picking one moves on. Back shows
 *  the earlier question with its answer still chosen. */
export function TargetStep({ state, send }: { state: WizardState; send: Dispatch<WizardEvent> }) {
  const { t } = useTranslation('templateTargets')
  const node = templateTargetAt(state.path.slice(0, state.depth))
  const seeds = (child: TargetNode): SectionDraft[] =>
    (child.roles ?? []).map((role: RoleKey) => {
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
  return (
    <ul className="grid gap-2 sm:grid-cols-2">
      {(node?.children ?? []).map((child) => (
        <li key={child.key}>
          <ChoiceButton
            className="w-full"
            title={t(`targets.${child.key as 'restaurant'}.name`)}
            description={t(`targets.${child.key as 'restaurant'}.help`)}
            selected={state.path[state.depth] === child.key}
            onClick={() =>
              send({
                type: 'choose',
                key: child.key,
                seeds: child.roles ? seeds(child) : undefined,
              })
            }
          />
        </li>
      ))}
    </ul>
  )
}
