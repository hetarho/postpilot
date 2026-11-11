import type { Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import { ChoiceButton } from '@/shared/ui'
import { templateTargetAt } from '@/entities/template'
import type { WizardEvent, WizardState } from '../model/wizard-machine'

/** One narrowing question (TMPL-72): the choices are peers, and a tap only chooses one — the round
 *  mark says which — while the step bar's 다음 moves on, as on every other choice in the writing
 *  journey. Back shows the earlier question with its answer still chosen. */
export function TargetStep({ state, send }: { state: WizardState; send: Dispatch<WizardEvent> }) {
  const { t } = useTranslation('templateTargets')
  const node = templateTargetAt(state.path.slice(0, state.depth))
  return (
    <ul className="grid gap-2 sm:grid-cols-2">
      {(node?.children ?? []).map((child) => (
        <li key={child.key}>
          <ChoiceButton
            className="w-full"
            title={t(`targets.${child.key as 'restaurant'}.name`)}
            description={t(`targets.${child.key as 'restaurant'}.help`)}
            selected={state.path[state.depth] === child.key}
            onClick={() => send({ type: 'pick', key: child.key })}
          />
        </li>
      ))}
    </ul>
  )
}
