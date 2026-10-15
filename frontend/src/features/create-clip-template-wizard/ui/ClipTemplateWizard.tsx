import { useId, useRef, useState } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { useTranslation } from 'react-i18next'
import { useBlocker } from '@tanstack/react-router'
import { CLIP_CAPTION_STYLES, CLIP_COMPOSITION_LIMITS } from '@/entities/clip-design'
import {
  CompositionPreview,
  parseClipTemplate,
  validateClipRecipe,
  useClipTemplateMutations,
  type ClipRecipe,
} from '@/entities/clip-template'
import { templateTargetAt, useWizardHeadingSuggestions } from '@/entities/template'
import { formatAppFailure } from '@/shared/lib'
import {
  Button,
  Checkbox,
  ChoiceButton,
  Dialog,
  Disclosure,
  FieldCount,
  FieldLabel,
  FieldMessage,
  TextField,
  Typography,
  WizardFrame,
} from '@/shared/ui'
import { clipStageSeeds, subjectOf } from '../config/catalog'
import { toClipWizardBody } from '../lib/to-composition'
import { saveClipTemplateMachine } from '../model/save-clip-template-machine'
import {
  checkedStages,
  clipWizardMachine,
  clipWizardStateOf,
  nameProblem,
  stageProblem,
  STAGE_ROLES,
  type ClipWizardEvent,
} from '../model/wizard-machine'
import { HeadingChoices, HeadingSuggestions } from './HeadingSuggestions'

interface Props {
  ownerId: string
  onSaved: (id: string) => void
  beforeSave?: () => boolean
  onSaveFailure?: () => void
}
export function ClipTemplateWizard(props: Props) {
  return <OwnedClipTemplateWizard key={props.ownerId} {...props} />
}
function OwnedClipTemplateWizard({ ownerId, onSaved, beforeSave, onSaveFailure }: Props) {
  const { t } = useTranslation(['clipWizard', 'templateTargets', 'clips'])
  const id = useId()
  const actor = useActorRef(clipWizardMachine)
  const state = useSelector(actor, clipWizardStateOf)
  const send = actor.send
  const selected = checkedStages(state)
  const headings = useWizardHeadingSuggestions(
    ownerId,
    'clip',
    state.path.map((key) => t(`targets.${key as 'restaurant'}.name`, { ns: 'templateTargets' })),
    selected.map((stage) => ({ id: stage.id, role: stage.name, description: stage.intent })),
    CLIP_COMPOSITION_LIMITS.labelChars,
  )
  const mutation = useClipTemplateMutations(ownerId)
  const leaving = useRef(false)
  const saveActor = useActorRef(saveClipTemplateMachine, {
    input: {
      ownerId,
      beforeSave,
      onFailure: onSaveFailure,
      save: async (recipe) => (await mutation.save.mutateAsync({ recipe })).id,
      onSaved: (templateId) => {
        leaving.current = true
        onSaved(templateId)
      },
    },
  })
  const savingSnapshot = useSelector(saveActor, (snapshot) => snapshot)
  const saving = savingSnapshot.matches('saving')
  const blocker = useBlocker({
    shouldBlockFn: () => state.path.length > 0 && !leaving.current,
    enableBeforeUnload: () => state.path.length > 0 && !leaving.current,
    withResolver: true,
  })
  const body = toClipWizardBody(state.stages)
  const recipe: ClipRecipe = {
    name: state.name.trim(),
    compositionBody: body,
    introPreset: state.introPreset,
    outroPreset: state.outroPreset,
    allowedCaptionStyles: [...state.allowedCaptionStyles],
  }
  let document: ReturnType<typeof parseClipTemplate> | undefined
  try {
    document = parseClipTemplate(body)
  } catch {
    /* Named field errors keep the draft editable. */
  }
  const save = () => {
    if (nameProblem(state.name) || !validateClipRecipe(recipe).valid) {
      send({ type: 'next' })
      return
    }
    saveActor.send({ type: 'save', ownerId, fields: recipe })
  }
  const back = { label: t('back', { ns: 'clipWizard' }), onClick: () => send({ type: 'back' }) }
  const next = (
    <Button variant="cta" onClick={() => send({ type: 'next' })}>
      {t('next', { ns: 'clipWizard' })}
    </Button>
  )
  const total = 3 + selected.length + 2
  const current =
    state.step === 'stages'
      ? 1
      : state.step === 'order'
        ? 2
        : state.step === 'headings'
          ? 3
          : state.step === 'stage'
            ? 4 + state.cursor
            : state.step === 'design'
              ? total - 1
              : total
  const progress =
    state.step === 'target'
      ? {
          text: t('targetProgress', { ns: 'clipWizard' }),
          label: t('progressLabel', { ns: 'clipWizard' }),
        }
      : {
          text: t('progress', { ns: 'clipWizard', current, total }),
          label: t('progressLabel', { ns: 'clipWizard' }),
          current,
          total,
        }
  const edit = (stageId: string, patch: Extract<ClipWizardEvent, { type: 'edit' }>['patch']) =>
    send({ type: 'edit', id: stageId, patch })
  const error = (problem: ReturnType<typeof stageProblem> | ReturnType<typeof nameProblem>) =>
    problem
      ? t(`problems.${problem}`, {
          ns: 'clipWizard',
          max:
            problem === 'intentLong'
              ? CLIP_COMPOSITION_LIMITS.promptChars
              : CLIP_COMPOSITION_LIMITS.labelChars,
        })
      : ''

  return (
    <>
      {state.step !== 'headings' && headings.pending && (
        <Typography variant="body" role="status">
          {t('headings.running', { ns: 'templateTargets' })}
        </Typography>
      )}
      <WizardFrame
        screen={
          state.step === 'target'
            ? `target:${state.depth}`
            : state.step === 'stage'
              ? `stage:${state.cursor}`
              : state.step
        }
        progress={progress}
        {...frame()}
      />
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('leaveTitle', { ns: 'clipWizard' })}
        confirmLabel={t('leave', { ns: 'clipWizard' })}
        onClose={() => blocker.reset?.()}
        onConfirm={() => blocker.proceed?.()}
      >
        {t('leaveBody', { ns: 'clipWizard' })}
      </Dialog>
    </>
  )

  function preview() {
    return (
      document && (
        <CompositionPreview
          document={document}
          presets={{ intro: state.introPreset, outro: state.outroPreset }}
          captionStyles={state.allowedCaptionStyles}
          onPresetsChange={({ intro, outro }) =>
            send({ type: 'design', patch: { introPreset: intro, outroPreset: outro } })
          }
        >
          <div
            role="group"
            aria-label={t('project.captionStyles', { ns: 'clips' })}
            className="grid gap-2 sm:grid-cols-2"
          >
            {CLIP_CAPTION_STYLES.map((style) => (
              <label key={style} className="flex min-h-11 items-center gap-3">
                <Checkbox
                  checked={state.allowedCaptionStyles.includes(style)}
                  onChange={(event) =>
                    send({
                      type: 'design',
                      patch: {
                        allowedCaptionStyles: event.target.checked
                          ? CLIP_CAPTION_STYLES.filter(
                              (entry) =>
                                entry === style || state.allowedCaptionStyles.includes(entry),
                            )
                          : state.allowedCaptionStyles.filter((entry) => entry !== style),
                      },
                    })
                  }
                />
                <Typography variant="body" as="span">
                  {t(`captionStyles.${style}`, { ns: 'clips' })}
                </Typography>
              </label>
            ))}
          </div>
        </CompositionPreview>
      )
    )
  }
  function frame() {
    switch (state.step) {
      case 'target': {
        const node = templateTargetAt(state.path.slice(0, state.depth))
        return {
          title:
            node?.key === 'root'
              ? t('target', { ns: 'clipWizard' })
              : t(`questions.${(node?.key ?? 'root') as 'root'}`, { ns: 'templateTargets' }),
          help: t('targetHelp', { ns: 'clipWizard' }),
          back: state.depth > 0 ? back : undefined,
          children: (
            <ul className="grid gap-2 sm:grid-cols-2">
              {node?.children?.map((child) => (
                <li key={child.key}>
                  <ChoiceButton
                    className="w-full"
                    title={t(`targets.${child.key as 'restaurant'}.name`, {
                      ns: 'templateTargets',
                    })}
                    description={t(`targets.${child.key as 'restaurant'}.help`, {
                      ns: 'templateTargets',
                    })}
                    selected={state.path[state.depth] === child.key}
                    onClick={() => {
                      const path = [...state.path.slice(0, state.depth), child.key]
                      const target = path
                        .map((key) =>
                          t(`targets.${key as 'restaurant'}.name`, { ns: 'templateTargets' }),
                        )
                        .join(' · ')
                      const subject = t(`subjects.${subjectOf(path) as 'meal'}`, {
                        ns: 'clipWizard',
                      })
                      const where = t(path[0] === 'product' ? 'whereProduct' : 'wherePlace', {
                        ns: 'clipWizard',
                      })
                      const labels = Object.fromEntries(
                        STAGE_ROLES.map((role) => [
                          role,
                          {
                            name: t(`roles.${role}.name`, { ns: 'clipWizard', subject }),
                            intent: t(`roles.${role}.intent`, {
                              ns: 'clipWizard',
                              target,
                              subject,
                              where,
                            }),
                          },
                        ]),
                      ) as Parameters<typeof clipStageSeeds>[0]
                      send({
                        type: 'choose',
                        key: child.key,
                        seeds: child.roles
                          ? clipStageSeeds(
                              labels,
                              t(path[0] === 'product' ? 'fieldProduct' : 'fieldPlace', {
                                ns: 'clipWizard',
                              }),
                            )
                          : undefined,
                      })
                    }}
                  />
                </li>
              ))}
            </ul>
          ),
        }
      }
      case 'stages':
        return {
          title: t('stages', { ns: 'clipWizard' }),
          help: t('stagesHelp', { ns: 'clipWizard' }),
          back,
          next,
          children: (
            <>
              <ul>
                {state.stages.map((stage) => (
                  <li key={stage.id} className="flex items-start gap-2">
                    <label className="flex min-h-11 flex-1 items-start gap-3 py-2">
                      <Checkbox
                        checked={stage.checked}
                        onChange={(event) =>
                          send({ type: 'toggle', id: stage.id, checked: event.target.checked })
                        }
                      />
                      <span>
                        <Typography variant="body" as="span" className="block">
                          {stage.name}
                        </Typography>
                        <Typography variant="meta" as="span" className="block">
                          {stage.intent}
                        </Typography>
                      </span>
                    </label>
                    {stage.role === 'custom' && (
                      <Button
                        variant="ghost"
                        size="compact"
                        onClick={() => send({ type: 'remove', id: stage.id })}
                      >
                        {t('remove', { ns: 'clipWizard', name: stage.name })}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
              {state.attempted &&
                (selected.length === 0 || selected.length > CLIP_COMPOSITION_LIMITS.stages) && (
                  <FieldMessage>
                    {t('problems.selection', {
                      ns: 'clipWizard',
                      max: CLIP_COMPOSITION_LIMITS.stages,
                    })}
                  </FieldMessage>
                )}
              <CustomStage
                send={send}
                atLimit={selected.length >= CLIP_COMPOSITION_LIMITS.stages}
              />
            </>
          ),
        }
      case 'order':
        return {
          title: t('order', { ns: 'clipWizard' }),
          back,
          next,
          children: (
            <ol className="space-y-2">
              {selected.map((stage, index) => (
                <li key={stage.id} className="flex min-h-11 items-center justify-between gap-2">
                  <Typography variant="body">{stage.name}</Typography>
                  <div className="flex gap-2">
                    <Button
                      variant="ghost"
                      size="compact"
                      disabled={index === 0}
                      aria-label={t('up', { ns: 'clipWizard', name: stage.name })}
                      onClick={() => send({ type: 'reorder', from: index, to: index - 1 })}
                    >
                      ↑
                    </Button>
                    <Button
                      variant="ghost"
                      size="compact"
                      disabled={index === selected.length - 1}
                      aria-label={t('down', { ns: 'clipWizard', name: stage.name })}
                      onClick={() => send({ type: 'reorder', from: index, to: index + 1 })}
                    >
                      ↓
                    </Button>
                  </div>
                </li>
              ))}
            </ol>
          ),
        }
      case 'headings':
        return {
          title: t('headings', { ns: 'clipWizard' }),
          help: t('headingHelp', { ns: 'clipWizard' }),
          back,
          children: (
            <div className="space-y-6">
              <HeadingSuggestions
                operation={headings}
                count={selected.length}
                onOwnWords={() => send({ type: 'next' })}
              />
              {selected.map((stage) => (
                <div key={stage.id}>
                  <FieldLabel htmlFor={`${id}-${stage.id}`}>
                    {t('headingField', { ns: 'clipWizard', name: stage.name })}
                  </FieldLabel>
                  <TextField
                    id={`${id}-${stage.id}`}
                    value={stage.heading}
                    onChange={(event) => edit(stage.id, { heading: event.target.value })}
                    className="mt-1"
                  />
                  <FieldCount
                    left={
                      CLIP_COMPOSITION_LIMITS.labelChars - Array.from(stage.heading.trim()).length
                    }
                  />
                  {state.attempted && (
                    <FieldMessage>
                      {error(
                        !stage.heading.trim()
                          ? 'headingMissing'
                          : Array.from(stage.heading.trim()).length >
                              CLIP_COMPOSITION_LIMITS.labelChars
                            ? 'headingLong'
                            : undefined,
                      )}
                    </FieldMessage>
                  )}
                  <HeadingChoices
                    name={stage.name}
                    suggestions={headings.suggestions(stage.id)}
                    onChoose={(heading) => edit(stage.id, { heading })}
                  />
                </div>
              ))}
            </div>
          ),
        }
      case 'stage': {
        const stage = selected[state.cursor]!
        return {
          title: t('stage', { ns: 'clipWizard', heading: stage.heading }),
          back,
          next,
          children: (
            <div className="space-y-6">
              <div>
                <FieldLabel htmlFor={`${id}-heading`}>
                  {t('stageName', { ns: 'clipWizard' })}
                </FieldLabel>
                <TextField
                  id={`${id}-heading`}
                  value={stage.heading}
                  onChange={(event) => edit(stage.id, { heading: event.target.value })}
                  className="mt-1"
                />
                <FieldCount
                  left={
                    CLIP_COMPOSITION_LIMITS.labelChars - Array.from(stage.heading.trim()).length
                  }
                />
                <HeadingChoices
                  name={stage.name}
                  suggestions={headings.suggestions(stage.id)}
                  onChoose={(heading) => edit(stage.id, { heading })}
                />
              </div>
              <div>
                <FieldLabel htmlFor={`${id}-intent`}>
                  {t('intent', { ns: 'clipWizard' })}
                </FieldLabel>
                <TextField
                  id={`${id}-intent`}
                  value={stage.intent}
                  onChange={(event) => edit(stage.id, { intent: event.target.value })}
                  className="mt-1"
                />
                <FieldCount
                  left={
                    CLIP_COMPOSITION_LIMITS.promptChars - Array.from(stage.intent.trim()).length
                  }
                />
                <Typography variant="meta">{t('intentHelp', { ns: 'clipWizard' })}</Typography>
              </div>
              <label className="flex min-h-11 items-center gap-3">
                <Checkbox
                  checked={stage.asks}
                  onChange={(event) => edit(stage.id, { asks: event.target.checked })}
                />
                <Typography variant="body" as="span">
                  {t('asks', { ns: 'clipWizard' })}
                </Typography>
              </label>
              {stage.asks && (
                <div>
                  <FieldLabel htmlFor={`${id}-field`}>
                    {t('fieldLabel', { ns: 'clipWizard' })}
                  </FieldLabel>
                  <TextField
                    id={`${id}-field`}
                    value={stage.fieldLabel}
                    onChange={(event) => edit(stage.id, { fieldLabel: event.target.value })}
                    className="mt-1"
                  />
                  <FieldCount
                    left={
                      CLIP_COMPOSITION_LIMITS.labelChars -
                      Array.from(stage.fieldLabel.trim()).length
                    }
                  />
                </div>
              )}
              {state.attempted && <FieldMessage>{error(stageProblem(stage))}</FieldMessage>}
            </div>
          ),
        }
      }
      case 'design':
        return {
          title: t('design', { ns: 'clipWizard' }),
          help: t('designHelp', { ns: 'clipWizard' }),
          back,
          next,
          children: preview(),
        }
      case 'finish':
        return {
          title: t('finish', { ns: 'clipWizard' }),
          back: saving ? undefined : back,
          next: (
            <Button variant="cta" pending={saving} onClick={save}>
              {t('save', { ns: 'clipWizard' })}
            </Button>
          ),
          children: (
            <div className="space-y-6">
              <div>
                <FieldLabel htmlFor={`${id}-name`}>{t('name', { ns: 'clipWizard' })}</FieldLabel>
                <TextField
                  id={`${id}-name`}
                  disabled={saving}
                  value={state.name}
                  onChange={(event) => send({ type: 'name', value: event.target.value })}
                  className="mt-1"
                />
                {state.attempted && <FieldMessage>{error(nameProblem(state.name))}</FieldMessage>}
                {savingSnapshot.context.failure && (
                  <FieldMessage>{formatAppFailure(savingSnapshot.context.failure)}</FieldMessage>
                )}
              </div>
              <ol>
                {selected.map((stage) => (
                  <li key={stage.id}>
                    <Typography variant="body">{stage.heading}</Typography>
                  </li>
                ))}
              </ol>
            </div>
          ),
        }
    }
  }
}
function CustomStage({
  send,
  atLimit,
}: {
  send: (event: ClipWizardEvent) => void
  atLimit: boolean
}) {
  const { t } = useTranslation('clipWizard')
  const id = useId()
  const [name, setName] = useState(''),
    [intent, setIntent] = useState('')
  return (
    <Disclosure title={t('add')} size="row" headingLevel={3} className="mt-4">
      <div className="space-y-3">
        <div>
          <FieldLabel htmlFor={`${id}-name`}>{t('addName')}</FieldLabel>
          <TextField
            id={`${id}-name`}
            value={name}
            onChange={(event) => setName(event.target.value)}
            className="mt-1"
          />
        </div>
        <div>
          <FieldLabel htmlFor={`${id}-intent`}>{t('addIntent')}</FieldLabel>
          <TextField
            id={`${id}-intent`}
            value={intent}
            onChange={(event) => setIntent(event.target.value)}
            className="mt-1"
          />
        </div>
        {atLimit && (
          <FieldMessage>
            {t('problems.addLimit', { max: CLIP_COMPOSITION_LIMITS.stages })}
          </FieldMessage>
        )}
        <Button
          variant="secondary"
          disabled={atLimit || !name.trim()}
          onClick={() => {
            send({ type: 'add', name, intent })
            setName('')
            setIntent('')
          }}
        >
          {t('addConfirm')}
        </Button>
      </div>
    </Disclosure>
  )
}
