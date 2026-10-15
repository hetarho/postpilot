import { useRef } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { useTranslation } from 'react-i18next'
import { useBlocker } from '@tanstack/react-router'
import { useCreateTemplate } from '@/entities/template'
import { Button, Dialog, Typography, WizardFrame } from '@/shared/ui'
import { nodeAt } from '../config/catalog'
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
import { useHeadingSuggestions } from '../model/useHeadingSuggestions'

/** 새 템플릿 as a guided wizard (TMPL-71): one question per screen, Back that keeps every answer,
 *  one emphasized next action, and nothing written before 저장 — which saves an ordinary template
 *  through the standard create. Leaving with answers asks before discarding them. */
export function TemplateWizard({
  ownerId,
  onSaved,
}: {
  ownerId: string
  onSaved: (templateId: string) => void
}) {
  return <OwnedTemplateWizard key={ownerId} ownerId={ownerId} onSaved={onSaved} />
}

function OwnedTemplateWizard({
  ownerId,
  onSaved,
}: {
  ownerId: string
  onSaved: (templateId: string) => void
}) {
  const { t } = useTranslation('templates')
  const actor = useActorRef(wizardMachine)
  const state = useSelector(actor, wizardStateOf)
  const send = actor.send
  const headings = useHeadingSuggestions(
    ownerId,
    'post',
    state.path.map((key) => t(`wizard.targets.${key as 'restaurant'}.name`)),
    checkedSections(state).map((section) => ({
      id: section.id,
      role: section.name,
      description: section.help,
    })),
  )
  const create = useCreateTemplate(ownerId)
  const leaving = useRef(false)
  const guard = () => wizardHasAnswers(state) && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })

  const save = async () => {
    if (finishProblem(state) || create.isPending) {
      send({ type: 'next' })
      return
    }
    try {
      const saved = await create.create({
        name: state.name.trim(),
        description: state.description.trim(),
        body: toWizardBody(state.sections),
        titleArea: '',
      })
      if (!saved.template) return
      leaving.current = true
      onSaved(saved.template.id)
    } catch {
      /* The refusal is shown on the name step; every answer stays. */
    }
  }

  const back = { label: t('wizard.back'), onClick: () => send({ type: 'back' }) }
  const next = (
    <Button variant="cta" onClick={() => send({ type: 'next' })}>
      {t('wizard.next')}
    </Button>
  )
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
      <WizardFrame screen={screenOf(state)} progress={progress} {...frame(state)} />
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
        const node = nodeAt(current.path.slice(0, current.depth))
        return {
          title: t(`wizard.questions.${(node?.key ?? 'root') as 'root'}`),
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
          back,
          next: (
            <Button variant="cta" pending={create.isPending} onClick={() => void save()}>
              {t('wizard.save')}
            </Button>
          ),
          children: <FinishStep state={current} send={send} refusal={create.errorMessage} />,
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
