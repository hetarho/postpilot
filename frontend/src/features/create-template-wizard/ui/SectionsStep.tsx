import { useId, useState, type Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import {
  Button,
  Checkbox,
  Disclosure,
  FieldLabel,
  FieldMessage,
  TextField,
  typographyStyles,
} from '@/shared/ui'
import { checkedSections, type WizardEvent, type WizardState } from '../model/wizard-machine'

/** The leaf's sections as a checklist, every one checked at first and each with what it covers,
 *  plus 직접 추가 for a section of the owner's own (TMPL-71). */
export function SectionsStep({ state, send }: { state: WizardState; send: Dispatch<WizardEvent> }) {
  const { t } = useTranslation('templates')
  const id = useId()
  const [name, setName] = useState('')
  const [help, setHelp] = useState('')
  const add = () => {
    if (name.trim() === '') return
    send({ type: 'add', name, help })
    setName('')
    setHelp('')
  }
  return (
    <>
      <ul>
        {state.sections.map((section) => (
          <li key={section.id} className="flex items-start gap-2">
            <label
              className={typographyStyles({
                variant: 'body',
                className: 'flex min-h-11 min-w-0 flex-1 items-start gap-3 py-2',
              })}
            >
              <span className="mt-1">
                <Checkbox
                  checked={section.checked}
                  onChange={(event) =>
                    send({ type: 'toggle', id: section.id, checked: event.target.checked })
                  }
                />
              </span>
              <span className="min-w-0">
                <span className="block break-words">{section.name}</span>
                {section.help && (
                  <span className="text-content-secondary block break-words">{section.help}</span>
                )}
              </span>
            </label>
            {section.id.startsWith('custom-') && (
              <Button
                variant="ghost"
                size="compact"
                aria-label={t('wizard.sections.remove', { name: section.name })}
                onClick={() => send({ type: 'remove', id: section.id })}
              >
                <X className="size-4" aria-hidden />
              </Button>
            )}
          </li>
        ))}
      </ul>
      {state.attempted && checkedSections(state).length === 0 && (
        <FieldMessage className="mt-2">{t('wizard.sections.none')}</FieldMessage>
      )}
      <Disclosure size="row" headingLevel={3} title={t('wizard.sections.add')} className="mt-4">
        <div className="space-y-3 pb-2">
          <div>
            <FieldLabel htmlFor={`${id}-name`}>{t('wizard.sections.addName')}</FieldLabel>
            <TextField
              id={`${id}-name`}
              value={name}
              onChange={(event) => setName(event.target.value)}
              className="mt-1"
            />
          </div>
          <div>
            <FieldLabel htmlFor={`${id}-help`}>{t('wizard.sections.addHelp')}</FieldLabel>
            <TextField
              id={`${id}-help`}
              value={help}
              onChange={(event) => setHelp(event.target.value)}
              className="mt-1"
            />
          </div>
          <Button variant="secondary" disabled={name.trim() === ''} onClick={add}>
            {t('wizard.sections.addConfirm')}
          </Button>
        </div>
      </Disclosure>
    </>
  )
}
