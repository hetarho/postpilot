import type { Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import { SortableList, Typography } from '@/shared/ui'
import { checkedSections, type WizardEvent, type WizardState } from '../model/wizard-machine'

/** The checked sections in the order the post will hold them, moved by drag or by buttons
 *  (TMPL-28). */
export function OrderStep({ state, send }: { state: WizardState; send: Dispatch<WizardEvent> }) {
  const { t } = useTranslation('templates')
  return (
    <SortableList
      items={checkedSections(state).map((section) => ({
        id: section.id,
        content: (
          <Typography variant="body" as="span" className="min-w-0 break-words">
            {section.name}
          </Typography>
        ),
      }))}
      onReorder={(from, to) => send({ type: 'reorder', from, to })}
      labels={{
        drag: t('wizard.order.drag'),
        up: t('wizard.order.up'),
        down: t('wizard.order.down'),
      }}
    />
  )
}
