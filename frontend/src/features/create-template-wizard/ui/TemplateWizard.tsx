import { useRef, useState } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { useTranslation } from 'react-i18next'
import { useBlocker } from '@tanstack/react-router'
import { useCreateTemplate } from '@/entities/template'
import { formatAppFailure } from '@/shared/lib'
import { Button, Dialog, Typography, WizardFrame, WizardPreviewDock } from '@/shared/ui'
import { templateTargetAt } from '@/entities/template'
import { toWizardBody } from '../lib/to-body'
import {
  checkedSections,
  finishProblem,
  wizardHasAnswers,
  wizardProgress,
  wizardMachine,
  wizardStateOf,
  type WizardState,
} from '../model/wizard-machine'
import { FinishStep } from './FinishStep'
import { HeadingsStep } from './HeadingsStep'
import { OrderStep } from './OrderStep'
import { SectionStep } from './SectionStep'
import { SectionsStep } from './SectionsStep'
import { TargetStep } from './TargetStep'
import { WizardPreview } from './WizardPreview'
import { useWizardHeadingSuggestions as useHeadingSuggestions } from '@/entities/template'
import { saveTemplateMachine } from '../model/save-template-machine'

interface Props {
  ownerId: string
  onSaved: (templateId: string) => void
  seedHeadings?: readonly string[]
  beforeSave?: () => boolean
  onSaveFailure?: () => void
}

/** 새 템플릿 as a guided wizard (TMPL-71): one question per screen, Back that keeps every answer,
 *  one emphasized next action, and nothing written before 저장 — which saves an ordinary template
 *  through the standard create. Leaving with answers asks before discarding them. */
export function TemplateWizard(props: Props) {
  return <OwnedTemplateWizard key={props.ownerId} {...props} />
}

function OwnedTemplateWizard({ ownerId, onSaved, seedHeadings, beforeSave, onSaveFailure }: Props) {
  const { t } = useTranslation('templates')
  const actor = useActorRef(wizardMachine, { input: { seedHeadings } })
  const state = useSelector(actor, wizardStateOf)
  const send = actor.send
  const headings = useHeadingSuggestions(
    ownerId,
    'post',
    state.path.map((key) => t(`targets.${key as 'restaurant'}.name`, { ns: 'templateTargets' })),
    checkedSections(state).map((section) => ({
      id: section.id,
      role: section.name,
      description: section.help,
    })),
  )
  const create = useCreateTemplate(ownerId)
  const leaving = useRef(false)
  const saveActor = useActorRef(saveTemplateMachine, {
    input: {
      ownerId,
      beforeSave,
      onFailure: onSaveFailure,
      save: async (fields) => (await create.create(fields)).template?.id ?? '',
      onSaved: (id) => {
        leaving.current = true
        onSaved(id)
      },
    },
  })
  const savingState = useSelector(saveActor, (snapshot) => snapshot)
  const saving = savingState.matches('saving')
  const refusal = savingState.context.failure ? formatAppFailure(savingState.context.failure) : ''
  const guard = () => wizardHasAnswers(state) && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })

  const save = () => {
    if (finishProblem(state)) {
      send({ type: 'next' })
      return
    }
    saveActor.send({
      type: 'save',
      ownerId,
      fields: {
        name: state.name.trim(),
        description: state.description.trim(),
        body: toWizardBody(state.sections),
        titleArea: '',
      },
    })
  }

  const back = { label: t('wizard.back'), onClick: () => send({ type: 'back' }) }
  const next = (
    <Button variant="cta" onClick={() => send({ type: 'next' })}>
      {t('wizard.next')}
    </Button>
  )
  // The preview stands beside every step from sections on (TMPL-75). Counting the body's changes
  // during render — React's pattern for a value derived from the previous one — tells the
  // phone's 미리보기 button when to glow without an effect.
  const body = toWizardBody(state.sections)
  const [seen, setSeen] = useState({ body, changes: 0 })
  if (seen.body !== body) setSeen({ body, changes: seen.changes + 1 })
  const previewing = state.step !== 'target'
  const currentId = state.step === 'section' ? checkedSections(state)[state.cursor]?.id : undefined

  const known = wizardProgress(state)
  const progress = known
    ? {
        text: t('wizard.progress', known),
        label: t('wizard.progressLabel'),
        current: known.current,
        total: known.total,
      }
    : { text: t('wizard.targetProgress'), label: t('wizard.progressLabel') }

  return (
    <>
      {state.step !== 'headings' && headings.pending && (
        <Typography variant="body" role="status">
          {t('wizard.headings.running')}
        </Typography>
      )}
      {previewing ? (
        <div className="lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,22rem)] lg:items-start lg:gap-12">
          <div className="min-w-0 lg:max-w-2xl">
            <WizardFrame
              screen={screenOf(state)}
              progress={progress}
              {...frame(state)}
              beforeActions={
                <WizardPreviewDock
                  changes={seen.changes}
                  label={t('wizard.preview.open')}
                  closeLabel={t('wizard.preview.close')}
                >
                  <WizardPreview sections={state.sections} currentId={currentId} titled={false} />
                </WizardPreviewDock>
              }
            />
          </div>
          {/* Below the app's sticky chrome, like the authoring editor's preview, with its own
              scroll so a long post never pushes the step's actions away. */}
          <aside className="sticky top-[calc(var(--spacing-chrome)+1.5rem)] hidden max-h-[calc(100dvh-var(--spacing-chrome)-3rem)] min-w-0 overflow-y-auto lg:block">
            <WizardPreview sections={state.sections} currentId={currentId} />
          </aside>
        </div>
      ) : (
        <WizardFrame screen={screenOf(state)} progress={progress} {...frame(state)} />
      )}
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('wizard.leaveTitle')}
        confirmLabel={t('wizard.leave')}
        onClose={() => blocker.reset?.()}
        onConfirm={() => blocker.proceed?.()}
      >
        {t('wizard.leaveBody')}
      </Dialog>
    </>
  )

  /** The current screen's title, help, body and actions. */
  function frame(current: WizardState) {
    switch (current.step) {
      case 'target': {
        const node = templateTargetAt(current.path.slice(0, current.depth))
        return {
          title: t(`questions.${(node?.key ?? 'root') as 'root'}`, { ns: 'templateTargets' }),
          help: t('wizard.targetHelp'),
          back: current.depth > 0 ? back : undefined,
          children: <TargetStep state={current} send={send} />,
        }
      }
      case 'sections':
        return {
          title: t('wizard.sections.title'),
          help: t('wizard.sections.help'),
          back,
          next,
          children: <SectionsStep state={current} send={send} />,
        }
      case 'order':
        return {
          title: t('wizard.order.title'),
          help: t('wizard.order.help'),
          back,
          next,
          children: <OrderStep state={current} send={send} />,
        }
      case 'headings':
        return {
          title: t('wizard.headings.title'),
          help: t('wizard.headings.help'),
          back,
          next,
          children: <HeadingsStep state={current} send={send} operation={headings} />,
        }
      case 'section': {
        const section = checkedSections(current)[current.cursor]!
        return {
          title: t('wizard.section.title', { heading: section.heading.trim() || section.name }),
          back,
          next,
          children: (
            <SectionStep
              key={section.id}
              state={current}
              section={section}
              send={send}
              suggestions={headings.suggestions(section.id)}
            />
          ),
        }
      }
      case 'finish':
        return {
          title: t('wizard.finish.title'),
          back: saving ? undefined : back,
          next: (
            <Button variant="cta" pending={saving} onClick={save}>
              {t('wizard.save')}
            </Button>
          ),
          children: <FinishStep state={current} send={send} refusal={refusal} disabled={saving} />,
        }
    }
  }
}

/** Which screen is on show, so moving between two sections or two questions is a navigation. */
function screenOf(state: WizardState): string {
  if (state.step === 'target') return `target:${state.depth}`
  if (state.step === 'section') return `section:${state.cursor}`
  return state.step
}
