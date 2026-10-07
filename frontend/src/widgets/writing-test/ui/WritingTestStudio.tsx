import { useWritingTestTranslation } from '@/features/writing-test'
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
} from 'react'
import {
  useModels,
  useSelections,
  useStageSelection,
  filterForStage,
  modelChoiceIssue,
  refKey,
  type CatalogModel,
} from '@/entities/model-catalog'
import { useVoices } from '@/entities/voice'
import { useTemplates, templateAskFields, TEMPLATE_PARSE_OPTIONS } from '@/entities/template'
import { useGuidelines, GuidelineFieldPicker } from '@/entities/guideline'
import {
  useCandidatePreparationClient,
  useWritingTestClient,
  useWritingTestSource,
  useWritingTestSources,
  type TestCount,
  type TestEntrant,
  type TestFactor,
  type TestStage,
  type TestSettingKind,
  type WritingTestPlan,
} from '@/entities/writing-test'
import {
  useWritingTestFlow,
  useCandidatePreparation,
  currentWritingTestMatch,
  writingTestDraftProblem,
  writingTestOperationBusy,
  WRITING_TEST_DIRECTION_MAX_CHARS,
  WRITING_TEST_NEW_GUIDELINE_SLOT,
} from '@/features/writing-test'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Checkbox,
  ChoiceButton,
  ContextualReturn,
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  Notice,
  SegmentedControl,
  Textarea,
  TextField,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import {
  POST_TARGET_LENGTH_MIN,
  POST_TARGET_LENGTH_MAX,
  POST_TAG_COUNT_MIN,
  POST_TAG_COUNT_MAX,
} from '@/entities/post'
import { BLOG_FIELD_IDS } from '@/entities/blog-field'
import { WritingTestPair } from './WritingTestPair'
import { WritingTestBracket } from './WritingTestBracket'
import { WritingTestChampionOutput } from './WritingTestChampionOutput'

const SETTING_KIND: Record<Exclude<TestFactor, 'model'>, TestSettingKind> = {
  voice: 'writing-voice',
  template: 'post-template',
  guideline: 'post-guideline',
}
const COUNTS: readonly TestCount[] = [2, 4, 8, 16]
interface SettingChoice {
  id: string
  name: string
  revision: string
  body?: string
  titleArea?: string
}
export interface WritingTestStudioProps {
  ownerId: string
  seedKey: string
  initialPlan: WritingTestPlan
  testId?: string
  onOpenTest?: (id: string) => void
}

export function WritingTestStudio(props: WritingTestStudioProps) {
  return (
    <OwnedStudio
      key={JSON.stringify([props.ownerId, props.seedKey, props.testId ?? ''])}
      {...props}
    />
  )
}
function OwnedStudio({
  ownerId,
  seedKey,
  initialPlan,
  testId,
  onOpenTest,
}: WritingTestStudioProps) {
  const { t } = useWritingTestTranslation()
  const models = useModels(),
    writer = useStageSelection('write'),
    observer = useStageSelection('observe')
  const activeSelections = useSelections()
  const voices = useVoices(ownerId),
    templates = useTemplates(ownerId),
    guidelines = useGuidelines(ownerId)
  const sources = useWritingTestSources(ownerId)
  const client = useWritingTestClient(),
    preparationClient = useCandidatePreparationClient()
  const flow = useWritingTestFlow({ ownerId, seedKey, initialDraft: initialPlan, testId, client })
  const plan = flow.context.draft ?? initialPlan
  const test = flow.context.test
  const kind: TestSettingKind =
    plan.factor === 'model' ? 'writing-voice' : SETTING_KIND[plan.factor]
  const preparation = useCandidatePreparation({
    ownerId,
    seedKey,
    client: preparationClient,
    draft: { kind, count: plan.count, prompt: '', writeModel: writer.selected ?? undefined },
  })
  const source = useWritingTestSource(ownerId, plan.context.sourcePostSlug)
  const appliedSource = useRef(initialPlan.context.sourcePostSlug)
  const appliedPreparation = useRef('')
  const refreshedPublication = useRef('')
  const [candidateMethod, setCandidateMethod] = useState<'owned' | 'generate'>(() =>
    plan.entrants.some((entrant) => entrant.type === 'authoring') ? 'generate' : 'owned',
  )
  const [attempted, setAttempted] = useState(false)
  const [confirmCancel, setConfirmCancel] = useState<'test' | 'preparation' | null>(null)
  const headingId = useId()
  const previousView = useRef(flow.view)
  const sourceName = source.data?.name
  const { send } = flow
  const draft = useCallback((next: WritingTestPlan) => send({ type: 'DRAFT', draft: next }), [send])
  const context = (patch: Partial<WritingTestPlan['context']>) =>
    draft({ ...plan, context: { ...plan.context, ...patch } })
  const prepBusy = ['quoting', 'creating', 'starting', 'reading', 'running', 'cancelling'].includes(
    preparation.phase,
  )
  const prepLocked =
    prepBusy || preparation.phase === 'uncertain' || preparation.phase === 'created'
  const operationBusy = writingTestOperationBusy(flow.phase)
  const choices: SettingChoice[] = useMemo(() => {
    if (plan.factor === 'voice')
      return voices.active
        .filter((v) => v.made && !!v.updatedAt)
        .map((v) => ({ id: v.id, name: v.name, revision: v.updatedAt }))
    if (plan.factor === 'template')
      return templates.templates
        .filter((v) => !!v.updatedAt)
        .map((v) => ({
          id: v.id,
          name: v.name,
          revision: v.updatedAt,
          body: v.body,
          titleArea: v.titleArea,
        }))
    if (plan.factor === 'guideline')
      return guidelines.guidelines
        .filter((v) => v.kind === 'post' && !!v.updatedAt)
        .map((v) => ({ id: v.id, name: v.title || v.text, revision: v.updatedAt, body: v.text }))
    return []
  }, [plan.factor, voices.active, templates.templates, guidelines.guidelines])
  const hasVideo =
    source.data?.attachments.some(
      (attachment) =>
        attachment.kind === 'video' && plan.context.material.attachmentIds.includes(attachment.id),
    ) ?? false
  const modelIssue = (model: CatalogModel, stage: TestStage) =>
    modelChoiceIssue({ ...model, affordable: true }, stage) ||
    (stage === 'observe' && hasVideo && (!model.videoInput || !model.signedVideoUrl)
      ? t('unsupportedVideo')
      : '')
  const modelOptions = (stage: TestStage) =>
    filterForStage(models.models, stage).map((m) => {
      const issue = modelIssue(m, stage)
      return {
        value: refKey(m.ref),
        label: issue ? `${m.label} · ${issue}` : m.label,
        disabled: !!issue,
      }
    })
  const modelEligibilityProblem =
    plan.factor === 'model' &&
    plan.entrants.some(
      (entrant) =>
        entrant.type === 'model' &&
        !filterForStage(models.models, plan.modelStage).some(
          (model) =>
            refKey(model.ref) === refKey(entrant.model) && !modelIssue(model, plan.modelStage),
        ),
    )
  const fixedModelProblem =
    (!(plan.factor === 'model' && plan.modelStage === 'write') &&
      !!plan.context.writeModel &&
      !filterForStage(models.models, 'write').some(
        (model) =>
          refKey(model.ref) === refKey(plan.context.writeModel!) && !modelIssue(model, 'write'),
      )) ||
    (plan.context.material.attachmentIds.length > 0 &&
      !(plan.factor === 'model' && plan.modelStage === 'observe') &&
      !!plan.context.observeModel &&
      !filterForStage(models.models, 'observe').some(
        (model) =>
          refKey(model.ref) === refKey(plan.context.observeModel!) && !modelIssue(model, 'observe'),
      ))
  const placeholder = (): TestEntrant =>
    plan.factor === 'model'
      ? { type: 'model', model: { providerId: '', modelId: '' } }
      : { type: 'setting', setting: { kind, id: '', revision: '' } }

  const choiceKey = (entrant: TestEntrant) =>
    entrant.type === 'model'
      ? refKey(entrant.model)
      : entrant.type === 'setting'
        ? entrant.setting.id
        : entrant.authoring.candidateId
  const choose = (position: number, value: string) => {
    let entrant: TestEntrant | undefined
    if (plan.factor === 'model') {
      const model = filterForStage(models.models, plan.modelStage).find(
        (m) => refKey(m.ref) === value,
      )
      if (model && !modelIssue(model, plan.modelStage))
        entrant = { type: 'model', model: model.ref }
    } else {
      const owned = choices.find((c) => c.id === value)
      if (owned)
        entrant = { type: 'setting', setting: { kind, id: owned.id, revision: owned.revision } }
    }
    const selected = Array.from(
      { length: plan.count },
      (_, index) => plan.entrants[index] ?? placeholder(),
    )
    selected[position] = entrant ?? placeholder()
    draft({ ...plan, entrants: selected })
  }
  const candidateProblem =
    plan.entrants.length !== plan.count ||
    plan.entrants.some(
      (entrant) =>
        !entrant ||
        (entrant.type === 'model'
          ? !entrant.model.providerId || !entrant.model.modelId
          : entrant.type === 'setting'
            ? !entrant.setting.id || !entrant.setting.revision
            : !entrant.authoring.sessionId ||
              !entrant.authoring.candidateId ||
              entrant.authoring.revision <= 0),
    ) ||
    new Set(plan.entrants.map(choiceKey)).size !== plan.count ||
    modelEligibilityProblem
  const asks = useMemo(() => {
    const entries =
      plan.factor === 'template'
        ? plan.entrants.flatMap<{ body?: string; titleArea?: string }>((entrant) => {
            if (entrant.type === 'setting')
              return choices.filter((c) => c.id === entrant.setting.id)
            if (entrant.type === 'authoring')
              return (
                preparation.context.session?.candidates.filter(
                  (c) => c.id === entrant.authoring.candidateId,
                ) ?? []
              )
            return []
          })
        : templates.templates.filter((item) => item.id === plan.context.templateId)
    const found = new Map<string, { label: string; required: boolean; prompt: string }>()
    for (const entry of entries)
      for (const ask of templateAskFields(
        entry.titleArea ?? '',
        entry.body ?? '',
        TEMPLATE_PARSE_OPTIONS,
      )) {
        const prior = found.get(ask.label)
        found.set(ask.label, { ...ask, required: ask.required || prior?.required === true })
      }
    return [...found.values()]
  }, [
    plan.factor,
    plan.entrants,
    plan.context.templateId,
    choices,
    templates.templates,
    preparation.context.session,
  ])
  const missingFacts = asks.some(
    (ask) =>
      ask.required &&
      !plan.context.material.templateAnswers.some(
        (a) => a.label === ask.label && a.enabled && a.text.trim(),
      ),
  )
  const sourcePending =
    !!plan.context.sourcePostSlug &&
    (source.isPending ||
      source.isError ||
      source.data?.context.sourcePostSlug !== plan.context.sourcePostSlug)
  useEffect(() => {
    if (flow.phase !== 'editing' || flow.context.test) return
    const next = { ...plan.context }
    let changed = false
    if (
      !(plan.factor === 'model' && plan.modelStage === 'write') &&
      !next.writeModel &&
      writer.selected
    ) {
      next.writeModel = writer.selected
      changed = true
    }
    if (
      !(plan.factor === 'model' && plan.modelStage === 'observe') &&
      !next.observeModel &&
      observer.selected
    ) {
      next.observeModel = observer.selected
      changed = true
    }
    if (changed) draft({ ...plan, context: next })
  }, [writer.selected, observer.selected, flow.phase, flow.context.test, plan, draft])
  useEffect(() => {
    const incoming = source.data
    if (
      !incoming ||
      incoming.context.sourcePostSlug !== plan.context.sourcePostSlug ||
      appliedSource.current === plan.context.sourcePostSlug ||
      flow.phase !== 'editing'
    )
      return
    appliedSource.current = plan.context.sourcePostSlug
    draft({
      ...plan,
      context: {
        ...incoming.context,
        writeModel: plan.context.writeModel,
        observeModel: plan.context.observeModel,
        guidelineSlotId: plan.context.guidelineSlotId,
      },
    })
  }, [source.data, plan, flow.phase, draft])
  useEffect(() => {
    const session = preparation.context.session
    if (
      preparation.phase !== 'ready' ||
      !session ||
      plan.factor === 'model' ||
      session.kind !== SETTING_KIND[plan.factor] ||
      session.count !== plan.count ||
      flow.phase !== 'editing'
    )
      return
    const key = JSON.stringify([session.sessionId, session.revision])
    if (key === appliedPreparation.current) return
    appliedPreparation.current = key
    draft({ ...plan, entrants: session.candidates.map((c) => c.source) })
    setCandidateMethod('generate')
  }, [preparation.phase, preparation.context.session, plan, flow.phase, draft])
  useEffect(() => {
    if (previousView.current !== flow.view) {
      previousView.current = flow.view
      document.getElementById(headingId)?.focus({ preventScroll: true })
    }
  }, [flow.view, headingId])
  useEffect(() => {
    if (test?.id) onOpenTest?.(test.id)
  }, [test?.id, onOpenTest])
  useEffect(() => {
    const publication = flow.context.publication
    if (!test || publication?.status !== 'confirmed') return
    const key = JSON.stringify([
      test.id,
      publication.id,
      publication.action,
      publication.requestKey,
    ])
    if (refreshedPublication.current === key) return
    refreshedPublication.current = key
    if (publication.action === 'apply-output') void source.refetch()
    else if (test.factor === 'model') activeSelections.refetch()
    else if (test.factor === 'voice') void voices.refetch()
    else if (test.factor === 'template') void templates.refetch()
    else void guidelines.refetch()
  }, [flow.context.publication, test, source, activeSelections, voices, templates, guidelines])

  const quoteCount = flow.context.retryEstimate?.candidateIds.length ?? test?.count ?? plan.count
  const workTitle = t(
    flow.phase === 'quoting'
      ? 'checkingCost'
      : flow.phase === 'publishing'
        ? 'publicationPending'
        : flow.phase === 'cancelling'
          ? 'endingTest'
          : flow.phase === 'loading'
            ? 'loading'
            : flow.phase === 'starting'
              ? 'starting'
              : flow.phase === 'deciding'
                ? 'confirmingDecision'
                : 'working',
  )
  const title =
    flow.view === 'factor'
      ? t('factorTitle')
      : flow.view === 'candidates'
        ? t('candidatesTitle')
        : flow.view === 'material'
          ? t('materialTitle')
          : flow.view === 'estimate'
            ? t('estimateTitle', { count: quoteCount })
            : flow.view === 'compare'
              ? t('compareTitle')
              : flow.view === 'champion'
                ? t('winnerTitle', { kind: t(`factor.${test?.factor ?? plan.factor}`) })
                : flow.view === 'publication'
                  ? t('publicationTitle', { kind: t(`factor.${test?.factor ?? plan.factor}`) })
                  : flow.view === 'work'
                    ? workTitle
                    : t(
                        flow.phase === 'partial'
                          ? 'partial'
                          : flow.phase === 'expired'
                            ? 'expired'
                            : flow.phase === 'cancelled'
                              ? 'cancelled'
                              : flow.phase === 'conflict'
                                ? 'conflict'
                                : flow.phase === 'failed'
                                  ? 'failed'
                                  : flow.phase === 'quoteExpired'
                                    ? 'quoteExpired'
                                    : 'uncertain',
                      )
  const next = () => {
    setAttempted(true)
    if (flow.view === 'candidates' && candidateProblem) return
    flow.sendPresentation({ type: 'NEXT' })
    setAttempted(false)
  }
  const winner = test?.candidates.find((c) => c.id === test.winnerCandidateId)
  const match = currentWritingTestMatch(test)
  const ready = test?.candidates.filter((c) => c.status === 'succeeded').length ?? 0
  const failedIds = test?.candidates.filter((c) => c.status === 'failed').map((c) => c.id) ?? []
  const publicationName = flow.presentation.choices.name
  const settingPublished =
    ((flow.context.publication?.status === 'confirmed' &&
      flow.context.publication.action !== 'apply-output') ||
      test?.publications.some(
        (publication) =>
          publication.status === 'confirmed' && publication.action !== 'apply-output',
      )) ??
    false
  const outputApplied =
    ((flow.context.publication?.status === 'confirmed' &&
      flow.context.publication.action === 'apply-output') ||
      test?.publications.some(
        (publication) =>
          publication.status === 'confirmed' && publication.action === 'apply-output',
      )) ??
    false
  const sourceApplicable =
    !!test &&
    test.factor === 'model' &&
    !!winner &&
    test.sourcePostSlug === plan.context.sourcePostSlug &&
    !!test.sourcePostSlug &&
    !!source.data &&
    ['draft', 'review'].includes(source.data.status) &&
    source.data.context.expectedInputRevision === plan.context.expectedInputRevision &&
    source.data.context.expectedContentRevision === plan.context.expectedContentRevision &&
    !outputApplied
  const applyOutput = () => {
    if (test && winner && sourceApplicable)
      flow.send({
        type: 'APPLY',
        input: {
          testId: test.id,
          expectedRevision: test.revision,
          winnerCandidateId: winner.id,
          expectedInputRevision: plan.context.expectedInputRevision,
          expectedContentRevision: plan.context.expectedContentRevision,
        },
      })
  }

  const openPublication = () => {
    if (!test || !winner?.identity) return
    const action =
      test.factor === 'model'
        ? 'adopt-model'
        : winner.identity.source.type === 'setting' &&
            !winner.identity.synthetic &&
            test.factor === 'voice'
          ? 'use-setting'
          : 'save-setting'
    flow.sendPresentation({
      type: 'PUBLICATION_CHOICES',
      choices: {
        ...flow.presentation.choices,
        action,
        name: winner.identity.label,
        scope: test.factor === 'guideline' ? '' : 'global',
      },
    })
    flow.sendPresentation({ type: 'OPEN_PUBLICATION' })
  }
  const publish = () => {
    if (!test || !winner) return
    setAttempted(true)
    const selection = flow.presentation.choices
    const action = selection.action
    if (action === 'apply-output') return
    if (
      selection.action === 'save-setting' &&
      test.factor !== 'guideline' &&
      !publicationName.trim()
    )
      return
    if (
      test.factor === 'guideline' &&
      (!selection.scope || (selection.scope !== 'global' && !selection.scopeIds.length))
    )
      return
    flow.send({
      type: 'PUBLISH',
      input: {
        testId: test.id,
        expectedRevision: test.revision,
        winnerCandidateId: winner.id,
        ...selection,
        action,
        name: publicationName,
      },
    })
  }
  return (
    <div className="min-w-0">
      <div className="flex flex-wrap items-center gap-2">
        <ContextualReturn />
        {!testId && flow.view === 'factor' && (
          <a href="/tests/history" className={buttonStyles({ variant: 'ghost' })}>
            {t('history')}
          </a>
        )}
      </div>
      <Typography
        variant="stepTitle"
        id={headingId}
        as="h2"
        tabIndex={-1}
        className="mt-6 break-words"
      >
        {title}
      </Typography>
      <div role="status" aria-live="polite" className="mt-3">
        {test && (
          <Typography variant="meta">
            {t('matchProgress', {
              done: test.matches.filter((m) => m.winnerCandidateId).length,
              total: test.count - 1,
            })}
          </Typography>
        )}
      </div>
      {flow.context.failure && (
        <Notice tone="danger" role="alert" className="mt-4">
          <AppFailureMessage failure={flow.context.failure} />
        </Notice>
      )}
      {flow.view === 'factor' && (
        <section className="mt-5 space-y-6">
          <Typography variant="body" className="max-w-measure">
            {t('factorHelp')}
          </Typography>
          <div className="grid gap-3 sm:grid-cols-2">
            {(['model', 'voice', 'template', 'guideline'] as const).map((factor) => (
              <ChoiceButton
                key={factor}
                title={t(`factor.${factor}`)}
                description={t('factorHelp')}
                selected={factor === plan.factor}
                onClick={() => {
                  if (factor !== plan.factor)
                    draft({
                      ...plan,
                      factor,
                      modelStage: factor === 'model' ? plan.modelStage : 'write',
                      entrants: [],
                      context: {
                        ...plan.context,
                        voiceId: factor === 'voice' ? '' : plan.context.voiceId,
                        templateId: factor === 'template' ? '' : plan.context.templateId,
                        guidelineSlotId:
                          factor === 'guideline' ? WRITING_TEST_NEW_GUIDELINE_SLOT : '',
                        writeModel:
                          factor === 'model' && plan.modelStage === 'write'
                            ? undefined
                            : (writer.selected ?? undefined),
                      },
                    })
                  flow.sendPresentation({ type: 'NEXT' })
                  setAttempted(false)
                }}
              />
            ))}
          </div>
          <WritingSelect
            label={t('format')}
            disabled={prepLocked}
            value={String(plan.count)}
            options={COUNTS.map((count) => ({
              value: String(count),
              label: t(`formatLabel.${count}`),
            }))}
            onChange={(value) =>
              draft({
                ...plan,
                count: Number(value) as TestCount,
                entrants: plan.entrants.slice(0, Number(value)),
              })
            }
          />
        </section>
      )}
      {flow.view === 'candidates' && (
        <section className="mt-5 space-y-6">
          <WritingSelect
            label={t('format')}
            disabled={prepLocked}
            value={String(plan.count)}
            options={COUNTS.map((count) => ({
              value: String(count),
              label: t(`formatLabel.${count}`),
            }))}
            onChange={(value) => {
              draft({ ...plan, count: Number(value) as TestCount, entrants: [] })
              appliedPreparation.current = ''
            }}
          />
          {plan.factor === 'model' ? (
            <WritingSelect
              label={t('modelStage')}
              disabled={prepLocked}
              value={plan.modelStage}
              options={(['write', 'observe'] as const).map((stage) => ({
                value: stage,
                label: t(`stage.${stage}`),
              }))}
              onChange={(value) =>
                draft({
                  ...plan,
                  modelStage: value as TestStage,
                  entrants: [],
                  context: {
                    ...plan.context,
                    writeModel: value === 'write' ? undefined : (writer.selected ?? undefined),
                    observeModel:
                      value === 'observe' ? undefined : (observer.selected ?? undefined),
                  },
                })
              }
            />
          ) : (
            <SegmentedControl
              disabled={prepLocked}
              ariaLabel={t('candidatesTitle')}
              value={candidateMethod}
              options={[
                { value: 'owned', label: t('owned') },
                { value: 'generate', label: t('generate') },
              ]}
              onChange={(value) => {
                if (!prepLocked) {
                  setCandidateMethod(value)
                  const session = preparation.context.session
                  const entrants =
                    value === 'generate' &&
                    preparation.phase === 'ready' &&
                    session?.kind === kind &&
                    session.count === plan.count
                      ? session.candidates.map((candidate) => candidate.source)
                      : []
                  draft({ ...plan, entrants })
                  setAttempted(false)
                }
              }}
            />
          )}
          {(plan.factor === 'model' || candidateMethod === 'owned') && (
            <>
              {(plan.factor === 'model'
                ? models.isError
                : plan.factor === 'voice'
                  ? voices.isError
                  : plan.factor === 'template'
                    ? templates.isError
                    : guidelines.isError) && (
                <Notice tone="danger">
                  <Button
                    variant="ghost"
                    onClick={() => {
                      models.refetch()
                      voices.refetch()
                      templates.refetch()
                      guidelines.refetch()
                    }}
                  >
                    {t('refresh')}
                  </Button>
                </Notice>
              )}
              <div className="grid gap-4 sm:grid-cols-2">
                {Array.from({ length: plan.count }, (_, index) => (
                  <WritingSelect
                    key={index}
                    label={t('contender', { number: index + 1 })}
                    value={plan.entrants[index] ? choiceKey(plan.entrants[index]) : ''}
                    options={[
                      { value: '', label: t('choose') },
                      ...(plan.factor === 'model'
                        ? modelOptions(plan.modelStage)
                        : choices.map((c) => ({ value: c.id, label: c.name }))),
                    ]}
                    onChange={(value) => choose(index, value)}
                  />
                ))}
              </div>
              {!choices.length && plan.factor !== 'model' && (
                <Typography variant="body">{t('emptyChoices')}</Typography>
              )}
              {plan.factor === 'model' && !modelOptions(plan.modelStage).length && (
                <Typography variant="body">
                  {t('noModels')}{' '}
                  <a href="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
                    {t('settings')}
                  </a>
                </Typography>
              )}
            </>
          )}
          {plan.factor !== 'model' && candidateMethod === 'generate' && (
            <section className="space-y-4">
              <Typography variant="body">{t('preparationHelp')}</Typography>
              <FieldLabel htmlFor="test-candidate-direction">{t('direction')}</FieldLabel>
              <Textarea
                id="test-candidate-direction"
                autoGrow
                rows={4}
                value={preparation.context.draft.prompt}
                maxLength={WRITING_TEST_DIRECTION_MAX_CHARS}
                placeholder={t('directionHint')}
                disabled={prepLocked}
                onChange={(e) =>
                  preparation.send({
                    type: 'EDIT',
                    draft: {
                      kind,
                      count: plan.count,
                      writeModel: writer.selected ?? undefined,
                      prompt: e.target.value,
                    },
                  })
                }
              />
              {!preparation.context.draft.prompt.trim() && (
                <Typography variant="label">{t('requiredDirection')}</Typography>
              )}
              {preparation.context.failure && (
                <Notice tone="danger" role="alert">
                  <AppFailureMessage failure={preparation.context.failure} />
                </Notice>
              )}
              {['idle', 'failed'].includes(preparation.phase) && (
                <Button
                  variant="secondary"
                  onClick={() => {
                    preparation.send({
                      type: 'EDIT',
                      draft: {
                        ...preparation.context.draft,
                        writeModel: writer.selected ?? undefined,
                      },
                    })
                    preparation.send({ type: 'ESTIMATE' })
                  }}
                  disabled={!writer.selected || !preparation.context.draft.prompt.trim()}
                >
                  {t('estimatePreparation')}
                </Button>
              )}
              {!writer.selected && (
                <Typography variant="body">
                  {t('requiredWriter')}{' '}
                  <a href="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
                    {t('settings')}
                  </a>
                </Typography>
              )}
              {preparation.phase === 'quoted' && (
                <>
                  <Typography variant="body">
                    {t('preparationTitle', { kind: t(`factor.${plan.factor}`), count: plan.count })}{' '}
                    {preparation.context.estimate?.free
                      ? t('free')
                      : t('credits', { count: preparation.context.estimate?.credits ?? 0 })}
                  </Typography>
                  <Button variant="cta" onClick={() => preparation.send({ type: 'CONFIRM' })}>
                    {t('prepare', { count: plan.count })}
                  </Button>
                  <Button variant="ghost" onClick={() => preparation.send({ type: 'BACK' })}>
                    {t('back')}
                  </Button>
                </>
              )}
              {preparation.phase === 'created' && (
                <Button variant="cta" onClick={() => preparation.send({ type: 'CONFIRM' })}>
                  {t('prepare', { count: plan.count })}
                </Button>
              )}
              {prepBusy && (
                <Typography variant="body" role="status">
                  {t('preparing')}
                </Typography>
              )}
              {preparation.phase === 'uncertain' && (
                <>
                  <Typography variant="body">{t('uncertainHelp')}</Typography>
                  <Button variant="secondary" onClick={() => preparation.send({ type: 'RETRY' })}>
                    {t('resumeRequest')}
                  </Button>
                </>
              )}
              {preparation.context.session && (
                <Button
                  variant="ghost"
                  onClick={() => preparation.send({ type: 'REFRESH' })}
                  disabled={prepBusy}
                >
                  {t('refresh')}
                </Button>
              )}
              {preparation.phase === 'running' && (
                <Button variant="ghost" onClick={() => setConfirmCancel('preparation')}>
                  {t('cancelPreparation')}
                </Button>
              )}
              {preparation.phase === 'ready' && (
                <>
                  <Typography variant="body" role="status">
                    {t('prepared', { count: plan.count })}
                  </Typography>
                  <ul className="grid gap-4 sm:grid-cols-2">
                    {preparation.context.session?.candidates.map((c) => (
                      <li key={c.id}>
                        <Typography variant="fieldTitle" as="h3">
                          {c.name}
                        </Typography>
                        <Typography variant="body" className="mt-2 break-words">
                          {c.description}
                        </Typography>
                        <Typography variant="label">{t('generatedCandidate')}</Typography>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </section>
          )}
          {attempted && candidateProblem && (
            <FieldMessage>{t('requiredChoices', { count: plan.count })}</FieldMessage>
          )}
          <ActionBar ariaLabel={title}>
            <div className="flex flex-wrap gap-3">
              <Button
                variant="ghost"
                onClick={() => flow.sendPresentation({ type: 'BACK' })}
                disabled={prepLocked}
              >
                {t('back')}
              </Button>
              {!(candidateMethod === 'generate' && prepLocked) && (
                <Button variant="cta" onClick={next}>
                  {t('next')}
                </Button>
              )}
            </div>
          </ActionBar>
        </section>
      )}
      {flow.view === 'material' && (
        <section className="mt-5 space-y-5">
          {sourceName && (
            <Typography variant="body">{t('source', { name: sourceName })}</Typography>
          )}
          <WritingSelect
            label={t('sourceLabel')}
            value={plan.context.sourcePostSlug}
            options={[
              { value: '', label: t('none') },
              ...sources.sources.map((item) => ({ value: item.slug, label: item.name })),
            ]}
            onChange={(slug) => {
              appliedSource.current = ''
              context({
                sourcePostSlug: slug,
                ...(!slug
                  ? {
                      expectedInputRevision: 0n,
                      expectedContentRevision: 0n,
                      material: { ...plan.context.material, attachmentIds: [] },
                    }
                  : {}),
              })
            }}
          />
          {sourcePending && (
            <Typography variant="body" role="status">
              {t(source.isError ? 'failed' : 'loading')}{' '}
              {source.isError && (
                <Button variant="ghost" onClick={() => void source.refetch()}>
                  {t('refresh')}
                </Button>
              )}
            </Typography>
          )}
          <Typography variant="body">{t('materialHint')}</Typography>
          <FieldLabel htmlFor="test-material">{t('material')}</FieldLabel>
          <Textarea
            id="test-material"
            autoGrow
            viewportAllocation={{ reservedBottom: 176 }}
            rows={6}
            value={plan.context.material.text}
            onChange={(e) =>
              context({ material: { ...plan.context.material, text: e.target.value } })
            }
          />
          <WritingCheck
            label={t('fictional')}
            checked={plan.context.material.fictional}
            onChange={(checked) =>
              context({ material: { ...plan.context.material, fictional: checked } })
            }
          />
          {source.data?.attachments.length !== 0 &&
            source.data?.attachments.map((attachment) => (
              <Typography key={attachment.id} variant="label">
                {t('attachment', { name: attachment.name })}
              </Typography>
            ))}
          <div className="grid gap-4 sm:grid-cols-2">
            <WritingSelect<'ko' | 'en'>
              label={t('language')}
              value={plan.context.targetLanguage}
              options={[
                { value: 'ko', label: t('ko') },
                { value: 'en', label: t('en') },
              ]}
              onChange={(targetLanguage) => context({ targetLanguage })}
            />
            <WritingOptionField
              label={t('targetLength')}
              type="number"
              min={POST_TARGET_LENGTH_MIN}
              max={POST_TARGET_LENGTH_MAX}
              value={plan.context.targetLength || ''}
              onChange={(e) => context({ targetLength: Number(e.target.value) })}
            />
            <WritingOptionField
              label={t('tags')}
              type="number"
              min={POST_TAG_COUNT_MIN}
              max={POST_TAG_COUNT_MAX}
              value={plan.context.tagCount}
              onChange={(e) => context({ tagCount: Number(e.target.value) })}
            />
          </div>
          {!(plan.factor === 'model' && plan.modelStage === 'write') && (
            <WritingSelect
              label={t('fixedWriter')}
              value={plan.context.writeModel ? refKey(plan.context.writeModel) : ''}
              options={[{ value: '', label: t('choose') }, ...modelOptions('write')]}
              onChange={(value) =>
                context({ writeModel: models.models.find((m) => refKey(m.ref) === value)?.ref })
              }
            />
          )}
          {plan.context.material.attachmentIds.length > 0 &&
            !(plan.factor === 'model' && plan.modelStage === 'observe') && (
              <WritingSelect
                label={t('fixedObserver')}
                value={plan.context.observeModel ? refKey(plan.context.observeModel) : ''}
                options={[{ value: '', label: t('choose') }, ...modelOptions('observe')]}
                onChange={(value) =>
                  context({ observeModel: models.models.find((m) => refKey(m.ref) === value)?.ref })
                }
              />
            )}
          {plan.factor !== 'voice' && (
            <WritingSelect
              label={t('fixedVoice')}
              value={plan.context.voiceId}
              options={[
                { value: '', label: t('none') },
                ...voices.active.filter((v) => v.made).map((v) => ({ value: v.id, label: v.name })),
              ]}
              onChange={(voiceId) => context({ voiceId })}
            />
          )}
          {plan.factor !== 'template' && (
            <WritingSelect
              label={t('fixedTemplate')}
              value={plan.context.templateId}
              options={[
                { value: '', label: t('none') },
                ...templates.templates.map((v) => ({ value: v.id, label: v.name })),
              ]}
              onChange={(templateId) => context({ templateId })}
            />
          )}
          {plan.factor === 'guideline' && (
            <>
              <WritingSelect
                label={t('guidelineSlot')}
                value={plan.context.guidelineSlotId}
                options={[
                  { value: WRITING_TEST_NEW_GUIDELINE_SLOT, label: t('newGuidelineSlot') },
                  ...guidelines.guidelines
                    .filter((g) => g.kind === 'post')
                    .map((g) => ({ value: g.id, label: g.title || g.text })),
                ]}
                onChange={(guidelineSlotId) => context({ guidelineSlotId })}
              />
              <Typography variant="body">{t('guidelineSlotHelp')}</Typography>
            </>
          )}
          {asks.length > 0 && (
            <section className="space-y-4">
              <Typography variant="title">{t('templateFacts')}</Typography>
              {asks.map((ask) => (
                <div key={ask.label}>
                  <FieldLabel htmlFor={`test-fact-${ask.label}`}>{ask.label}</FieldLabel>
                  <Textarea
                    id={`test-fact-${ask.label}`}
                    rows={2}
                    autoGrow
                    value={
                      plan.context.material.templateAnswers.find((a) => a.label === ask.label)
                        ?.text ?? ''
                    }
                    onChange={(e) =>
                      context({
                        material: {
                          ...plan.context.material,
                          templateAnswers: [
                            ...plan.context.material.templateAnswers.filter(
                              (a) => a.label !== ask.label,
                            ),
                            { label: ask.label, text: e.target.value, enabled: true },
                          ],
                        },
                      })
                    }
                  />
                  {ask.prompt && <Typography variant="body">{ask.prompt}</Typography>}
                </div>
              ))}
            </section>
          )}
          {attempted &&
            (!Number.isInteger(plan.context.targetLength) ||
              plan.context.targetLength < POST_TARGET_LENGTH_MIN ||
              plan.context.targetLength > POST_TARGET_LENGTH_MAX) && (
              <FieldMessage>
                {t('targetLengthBounds', {
                  min: POST_TARGET_LENGTH_MIN,
                  max: POST_TARGET_LENGTH_MAX,
                })}
              </FieldMessage>
            )}
          {attempted &&
            (!Number.isInteger(plan.context.tagCount) ||
              plan.context.tagCount < POST_TAG_COUNT_MIN ||
              plan.context.tagCount > POST_TAG_COUNT_MAX) && (
              <FieldMessage>
                {t('tagCountBounds', { min: POST_TAG_COUNT_MIN, max: POST_TAG_COUNT_MAX })}
              </FieldMessage>
            )}
          {attempted &&
            (writingTestDraftProblem(plan) ||
              missingFacts ||
              modelEligibilityProblem ||
              fixedModelProblem) && (
              <FieldMessage>
                {modelEligibilityProblem || fixedModelProblem
                  ? t(hasVideo ? 'unsupportedVideo' : 'ineligibleModel')
                  : missingFacts
                    ? t('requiredInput')
                    : plan.factor === 'model' &&
                        plan.modelStage === 'observe' &&
                        !plan.context.material.attachmentIds.length
                      ? t('requiredObserver')
                      : t('requiredMaterial')}
              </FieldMessage>
            )}
          <ActionBar ariaLabel={title}>
            <div className="flex flex-wrap gap-3">
              <Button variant="ghost" onClick={() => flow.sendPresentation({ type: 'BACK' })}>
                {t('back')}
              </Button>
              <Button
                variant="cta"
                pending={flow.phase === 'quoting'}
                disabled={sourcePending}
                onClick={() => {
                  setAttempted(true)
                  if (
                    !missingFacts &&
                    !sourcePending &&
                    !modelEligibilityProblem &&
                    !fixedModelProblem
                  )
                    flow.send({ type: 'ESTIMATE' })
                }}
              >
                {t('estimatePosts', { count: plan.count })}
              </Button>
            </div>
          </ActionBar>
        </section>
      )}
      {flow.view === 'estimate' && (
        <section className="mt-5 space-y-5">
          <Typography variant="body">{t('onceOnly')}</Typography>
          <Typography variant="title">
            {flow.context.quote?.free
              ? t('free')
              : t('credits', { count: flow.context.quote?.credits ?? 0 })}
          </Typography>
          <ActionBar ariaLabel={title}>
            <div className="flex flex-wrap gap-3">
              <Button variant="ghost" onClick={() => flow.sendPresentation({ type: 'BACK' })}>
                {t('back')}
              </Button>
              <Button variant="cta" onClick={() => flow.send({ type: 'CONFIRM' })}>
                {t(flow.context.retryEstimate ? 'retry' : 'start', { count: quoteCount })}
              </Button>
            </div>
          </ActionBar>
        </section>
      )}
      {flow.view === 'work' && (
        <section className="mt-5 space-y-5">
          {['running', 'starting', 'retrying'].includes(flow.phase) && (
            <>
              <Typography variant="body" role="status">
                {t('progress', { ready, total: test?.count ?? plan.count })}
              </Typography>
              <Typography variant="body">{t('leaveHelp')}</Typography>
            </>
          )}
          <a href="/tests/history" className={buttonStyles({ variant: 'ghost' })}>
            {t('history')}
          </a>
          {test && !operationBusy && (
            <Button variant="ghost" onClick={() => setConfirmCancel('test')}>
              {t('cancel')}
            </Button>
          )}
        </section>
      )}
      {flow.view === 'compare' && test && match && (
        <section className="mt-5">
          <WritingTestPair
            test={test}
            match={match}
            pending={flow.phase === 'deciding'}
            onWinner={(id) =>
              flow.send({
                type: 'VOTE',
                testId: test.id,
                expectedRevision: test.revision,
                matchId: match.id,
                winnerCandidateId: id,
              })
            }
            visibleCandidateId={flow.presentation.visibleCandidateId}
            onShowCandidate={(candidateId) =>
              flow.sendPresentation({ type: 'SHOW_CANDIDATE', candidateId })
            }
            reading={flow.presentation.reading}
            onReading={(candidateId, position) =>
              flow.sendPresentation({ type: 'READING', candidateId, position })
            }
          />
          <WritingTestBracket test={test} />
        </section>
      )}
      {flow.view === 'champion' && test && winner && (
        <section className="mt-5 space-y-5">
          <Typography variant="fieldTitle">{winner.identity?.label}</Typography>
          <Typography variant="body">{t('winnerHelp')}</Typography>
          {settingPublished ? (
            <Notice tone="success" role="status">
              {t('confirmed', {
                kind: t(`factor.${test.factor}`),
                name:
                  flow.context.publication?.action === 'save-setting'
                    ? publicationName || winner.identity?.label
                    : (winner.identity?.label ?? publicationName),
              })}
            </Notice>
          ) : (
            <Button variant="cta" onClick={openPublication}>
              {t(test.factor === 'model' ? 'adoptModel' : 'saveWinner', {
                kind: t(`factor.${test.factor}`),
              })}
            </Button>
          )}
          {sourceApplicable && (
            <Button variant="secondary" onClick={applyOutput}>
              {t('applyOutput')}
            </Button>
          )}
          {outputApplied && (
            <Notice tone="success" role="status">
              {t('outputApplied')}
            </Notice>
          )}
          <WritingTestBracket test={test} />
          <WritingTestChampionOutput test={test} />
        </section>
      )}
      {flow.view === 'publication' && test && winner && (
        <section className="mt-5 space-y-5">
          {test.factor !== 'model' && flow.presentation.choices.action === 'save-setting' && (
            <WritingOptionField
              label={t('name')}
              value={publicationName}
              onChange={(e) =>
                flow.sendPresentation({
                  type: 'PUBLICATION_CHOICES',
                  choices: { ...flow.presentation.choices, name: e.target.value },
                })
              }
            />
          )}
          {test.factor === 'voice' && (
            <>
              <Typography variant="body">
                {t(winner.identity?.synthetic ? 'synthetic' : 'useWinner', {
                  kind: t('factor.voice'),
                })}
              </Typography>
              <WritingCheck
                label={t('makeDefault')}
                checked={flow.presentation.choices.makeDefault}
                onChange={(makeDefault) =>
                  flow.sendPresentation({
                    type: 'PUBLICATION_CHOICES',
                    choices: { ...flow.presentation.choices, makeDefault },
                  })
                }
              />
            </>
          )}
          {test.factor === 'guideline' && (
            <>
              <WritingSelect
                label={t('scope')}
                value={flow.presentation.choices.scope}
                options={[
                  { value: '', label: t('choose') },
                  { value: 'global', label: t('global') },
                  { value: 'templates', label: t('templates') },
                  { value: 'fields', label: t('fields') },
                ]}
                onChange={(scope) =>
                  flow.sendPresentation({
                    type: 'PUBLICATION_CHOICES',
                    choices: { ...flow.presentation.choices, scope, scopeIds: [] },
                  })
                }
              />
              {flow.presentation.choices.scope === 'templates' && (
                <fieldset className="space-y-3">
                  <Typography variant="fieldTitle" as="legend">
                    {t('scopeTargets')}
                  </Typography>
                  {templates.templates.map((template) => (
                    <WritingCheck
                      key={template.id}
                      label={template.name}
                      checked={flow.presentation.choices.scopeIds.includes(template.id)}
                      onChange={(checked) =>
                        flow.sendPresentation({
                          type: 'PUBLICATION_CHOICES',
                          choices: {
                            ...flow.presentation.choices,
                            scopeIds: checked
                              ? [...flow.presentation.choices.scopeIds, template.id]
                              : flow.presentation.choices.scopeIds.filter(
                                  (id) => id !== template.id,
                                ),
                          },
                        })
                      }
                    />
                  ))}
                </fieldset>
              )}
            </>
          )}
          {test.factor === 'guideline' && flow.presentation.choices.scope === 'fields' && (
            <GuidelineFieldPicker
              value={BLOG_FIELD_IDS.filter((field) =>
                flow.presentation.choices.scopeIds.includes(field),
              )}
              onChange={(scopeIds) =>
                flow.sendPresentation({
                  type: 'PUBLICATION_CHOICES',
                  choices: { ...flow.presentation.choices, scopeIds },
                })
              }
              legend={t('fields')}
            />
          )}
          {attempted &&
            ((!publicationName.trim() &&
              test.factor !== 'guideline' &&
              flow.presentation.choices.action === 'save-setting') ||
              (test.factor === 'guideline' &&
                (!flow.presentation.choices.scope ||
                  (flow.presentation.choices.scope !== 'global' &&
                    !flow.presentation.choices.scopeIds.length)))) && (
              <FieldMessage>{t('requiredInput')}</FieldMessage>
            )}
          <ActionBar ariaLabel={title}>
            <div className="flex flex-wrap gap-3">
              <Button variant="ghost" onClick={() => flow.sendPresentation({ type: 'BACK' })}>
                {t('back')}
              </Button>
              <Button variant="cta" pending={flow.phase === 'publishing'} onClick={publish}>
                {test.factor === 'model'
                  ? t('adoptModel')
                  : flow.presentation.choices.action === 'use-setting'
                    ? t('useNamedWinner', {
                        kind: t(`factor.${test.factor}`),
                        name: winner.identity?.label ?? publicationName,
                      })
                    : test.factor !== 'guideline' && !publicationName.trim()
                      ? t('enterName')
                      : t('save', {
                          name: publicationName || t('factor.guideline'),
                        })}
              </Button>
            </div>
          </ActionBar>
        </section>
      )}
      {flow.view === 'recovery' && (
        <section className="mt-5 space-y-5">
          <Typography variant="body">
            {t(
              flow.phase === 'partial'
                ? 'partialHelp'
                : flow.phase === 'expired'
                  ? 'expiredHelp'
                  : flow.phase === 'conflict'
                    ? 'conflictHelp'
                    : flow.phase === 'failed'
                      ? 'failedHelp'
                      : flow.phase === 'cancelled'
                        ? 'cancelledHelp'
                        : 'uncertainHelp',
            )}
          </Typography>
          {flow.phase === 'partial' && test && failedIds.length > 0 && (
            <Button
              variant="cta"
              onClick={() =>
                flow.send({
                  type: 'ESTIMATE_RETRY',
                  testId: test.id,
                  expectedRevision: test.revision,
                  candidateIds: failedIds,
                })
              }
            >
              {t('estimateRetry')}
            </Button>
          )}
          {flow.phase === 'quoteExpired' && (
            <Button variant="cta" onClick={() => flow.send({ type: 'ESTIMATE' })}>
              {t('estimatePosts', { count: plan.count })}
            </Button>
          )}
          {['uncertain', 'uncertainPublication'].includes(flow.phase) && (
            <Button variant="secondary" onClick={() => flow.send({ type: 'RETRY_OPERATION' })}>
              {t('resumeRequest')}
            </Button>
          )}
          {flow.phase === 'failed' && !test && !flow.context.testId && !flow.context.command && (
            <Button variant="secondary" onClick={() => draft(plan)}>
              {t('editInput')}
            </Button>
          )}
          {(test || flow.context.testId) && (
            <Button variant="ghost" onClick={() => flow.send({ type: 'RELOAD' })}>
              {t('refresh')}
            </Button>
          )}
          {flow.phase === 'conflict' &&
            winner &&
            test &&
            test.factor !== 'model' &&
            !(test.factor === 'voice' && !winner.identity?.synthetic) && (
              <Button
                variant="secondary"
                onClick={() => {
                  flow.sendPresentation({
                    type: 'PUBLICATION_CHOICES',
                    choices: {
                      ...flow.presentation.choices,
                      action: 'save-setting',
                      name: publicationName,
                    },
                  })
                  flow.sendPresentation({ type: 'OPEN_PUBLICATION' })
                }}
              >
                {t('saveWinner', { kind: t(`factor.${test.factor}`) })}
              </Button>
            )}
          {flow.phase === 'conflict' &&
            test?.factor === 'voice' &&
            !winner?.identity?.synthetic && (
              <Typography variant="body">{t('personalChanged')}</Typography>
            )}
          {test && !['cancelled', 'expired', 'uncertainPublication'].includes(flow.phase) && (
            <Button variant="ghost" onClick={() => setConfirmCancel('test')}>
              {t('cancel')}
            </Button>
          )}
          {test && winner && <WritingTestChampionOutput test={test} />}
          <a href="/tests/history" className={buttonStyles({ variant: 'ghost' })}>
            {t('history')}
          </a>
        </section>
      )}
      <Dialog
        open={confirmCancel !== null}
        title={t(confirmCancel === 'preparation' ? 'cancelPreparationTitle' : 'cancelTitle')}
        confirmLabel={t(confirmCancel === 'preparation' ? 'cancelPreparation' : 'cancel')}
        cancelLabel={t('keep')}
        onClose={() => setConfirmCancel(null)}
        onConfirm={() => {
          if (confirmCancel === 'test' && test)
            flow.send({ type: 'CANCEL', testId: test.id, expectedRevision: test.revision })
          else if (confirmCancel === 'preparation') preparation.send({ type: 'CANCEL' })
          setConfirmCancel(null)
        }}
      >
        <Typography variant="body">
          {t(confirmCancel === 'preparation' ? 'cancelPreparationHelp' : 'cancelHelp')}
        </Typography>
      </Dialog>
    </div>
  )
}

function WritingOptionField({
  label,
  ...props
}: { label: string } & ComponentProps<typeof TextField>) {
  const id = useId()
  return (
    <div className="space-y-2">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <TextField id={id} {...props} />
    </div>
  )
}
function WritingCheck({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <Typography
      variant="body"
      as="label"
      className="flex min-h-11 cursor-pointer items-center gap-4"
    >
      <Checkbox checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </Typography>
  )
}

function WritingSelect<T>({
  label,
  ...props
}: { label: string } & ComponentProps<typeof Listbox<T>>) {
  const id = useId()
  return (
    <div className="space-y-2">
      <FieldLabel id={`${id}-label`} htmlFor={id}>
        {label}
      </FieldLabel>
      <Listbox id={id} aria-labelledby={`${id}-label`} {...props} />
    </div>
  )
}
