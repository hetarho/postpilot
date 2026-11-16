import { useRef, useState, type ReactNode } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { useTranslation } from 'react-i18next'
import { useBlocker } from '@tanstack/react-router'
import { useCreateTemplate } from '@/entities/template'
import { formatAppFailure } from '@/shared/lib'
import { Dialog, Typography, WizardFrame, WizardPreviewDock } from '@/shared/ui'
import { templateTargetAt, templateTargetQuestionCount } from '@/entities/template'
import { toWizardBody } from '../lib/to-body'
import {
  checkedSections,
  finishProblem,
  pickedTarget,
  wizardHasAnswers,
  wizardMachine,
  wizardStateOf,
  type WizardState,
} from '../model/wizard-machine'
import { contentKindHelpHidden } from '../lib/content-kind-help'
import { ContentKindHelp } from './ContentKindHelp'
import { FinishStep } from './FinishStep'
import { HeadingsStep } from './HeadingsStep'
import { OrderStep } from './OrderStep'
import { SectionStep } from './SectionStep'
import { SectionsStep } from './SectionsStep'
import { TargetStep } from './TargetStep'
import { WizardPreview } from './WizardPreview'
import { useWizardHeadingSuggestions as useHeadingSuggestions } from '@/entities/template'
import { saveTemplateMachine } from '../model/save-template-machine'
import { useTargetSeeds } from '../model/target-seeds'

interface Props {
  ownerId: string
  onSaved: (templateId: string) => void
  seedHeadings?: readonly string[]
  beforeSave?: () => boolean
  onSaveFailure?: () => void
  /** A flow the wizard sits in (setup): its way back once the wizard is on its first question,
   *  and its own action under the step bar's row, so the screen keeps ONE bar. */
  host?: { back: () => void; backDisabled?: boolean; footer?: ReactNode }
}

/** 새 템플릿 as a guided wizard (TMPL-71): one question per screen, Back that keeps every answer,
 *  one emphasized next action in the step bar every stepped screen shares, and nothing written
 *  before 저장 — which saves an ordinary template through the standard create. Leaving with
 *  answers asks before discarding them. */
export function TemplateWizard(props: Props) {
  return <OwnedTemplateWizard key={props.ownerId} {...props} />
}

function OwnedTemplateWizard({
  ownerId,
  onSaved,
  seedHeadings,
  beforeSave,
  onSaveFailure,
  host,
}: Props) {
  const { t } = useTranslation('templates')
  const seedsFor = useTargetSeeds()
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

  // 이 자리에 들어갈 것's explanation opens by itself the first time the owner reaches a part's
  // screen, unless they asked not to see it again; ⓘ beside the choice opens it any time.
  // Adjusted during render — React's pattern for state that follows another — so the sheet is
  // open on the very frame the first part arrives, with no effect cascading a second render.
  const [helpOpen, setHelpOpen] = useState(false)
  const [helpOffered, setHelpOffered] = useState(false)
  if (!helpOffered && state.step === 'section' && state.cursor === 0) {
    setHelpOffered(true)
    if (!contentKindHelpHidden()) setHelpOpen(true)
  }

  const back = { label: t('wizard.back'), onClick: () => send({ type: 'back' }) }
  const next = { label: t('wizard.next'), forward: true, onClick: () => send({ type: 'next' }) }
  // The preview stands beside every step from sections on (TMPL-75). Counting the body's changes
  // during render — React's pattern for a value derived from the previous one — tells the
  // phone's 미리보기 button when to glow without an effect.
  const body = toWizardBody(state.sections)
  const [seen, setSeen] = useState({ body, changes: 0 })
  if (seen.body !== body) setSeen({ body, changes: seen.changes + 1 })
  const previewing = state.step !== 'target'
  const currentId = state.step === 'section' ? checkedSections(state)[state.cursor]?.id : undefined

  // One segment per question the owner actually answers: each "what is it" level is its own step
  // (종류, then 세부 종류 — and 형식 for a product), and the per-part screens share 내용, where
  // the position names the part.
  const stepValues = [
    ...Array.from({ length: templateTargetQuestionCount(state.path) }, (_, at) => `target:${at}`),
    'sections',
    'order',
    'headings',
    'section',
    'finish',
  ]
  const currentStep = state.step === 'target' ? `target:${state.depth}` : state.step
  const at = Math.max(0, stepValues.indexOf(currentStep)) + 1
  const progress = {
    steps: stepValues.map((value) => ({
      value,
      label: t(
        `wizard.steps.${value.startsWith('target:') ? `target${value.slice(7)}` : value}` as 'wizard.steps.sections',
      ),
    })),
    current: currentStep,
    label: t('wizard.progressLabel'),
    position:
      state.step === 'section'
        ? t('wizard.positionPart', {
            current: at,
            total: stepValues.length,
            part: state.cursor + 1,
            parts: checkedSections(state).length,
          })
        : t('wizard.position', { current: at, total: stepValues.length }),
  }

  return (
    <>
      {state.step !== 'headings' && headings.pending && (
        <Typography variant="body" role="status">
          {t('wizard.headings.running')}
        </Typography>
      )}
      {previewing ? (
        // Flex columns down to the frame, so its step bar meets the screen's bottom edge; on a
        // desk the grid's one row stretches to the column's height for the same reason.
        <div className="flex flex-1 flex-col lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,22rem)] lg:gap-12">
          <div className="flex min-w-0 flex-1 flex-col lg:max-w-2xl">
            <WizardFrame
              screen={screenOf(state)}
              progress={progress}
              {...frame(state)}
              barFooter={host?.footer}
              barAction={
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
          <aside className="sticky top-[calc(var(--spacing-chrome)+1.5rem)] hidden max-h-[calc(100dvh-var(--spacing-chrome)-3rem)] min-w-0 self-start overflow-y-auto lg:block">
            <WizardPreview sections={state.sections} currentId={currentId} />
          </aside>
        </div>
      ) : (
        <WizardFrame
          screen={screenOf(state)}
          progress={progress}
          {...frame(state)}
          barFooter={host?.footer}
        />
      )}
      <ContentKindHelp open={helpOpen} onClose={() => setHelpOpen(false)} />
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
        const picked = pickedTarget(current)
        return {
          title: t(`questions.${(node?.key ?? 'root') as 'root'}`, { ns: 'templateTargets' }),
          help: t('wizard.targetHelp'),
          // On the first question the way back is the host flow's, when there is one.
          back:
            current.depth > 0
              ? back
              : host && {
                  label: t('wizard.back'),
                  onClick: host.back,
                  disabled: host.backDisabled,
                },
          // A tap only chooses; 다음 moves on with the chosen answer, held until there is one.
          next: {
            label: t('wizard.next'),
            forward: true,
            disabled: !picked,
            reason: picked ? undefined : t('wizard.pickFirst'),
            onClick: () => {
              if (picked) send({ type: 'choose', key: picked.key, seeds: seedsFor(picked) })
            },
          },
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
          // Suggestions asked for are waited for here, so the owner picks them on this screen
          // instead of meeting them a step later (owner 2026-11-16).
          next: headings.pending
            ? { ...next, disabled: true, pending: true, reason: t('wizard.headings.waiting') }
            : next,
          children: (
            <HeadingsStep ownerId={ownerId} state={current} send={send} operation={headings} />
          ),
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
              onHelp={() => setHelpOpen(true)}
            />
          ),
        }
      }
      case 'finish':
        return {
          title: t('wizard.finish.title'),
          back: saving ? undefined : back,
          next: { label: t('wizard.save'), pending: saving, onClick: save },
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
