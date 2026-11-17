import { ReturnIcon, StepBarActions, StepBarReason } from '@/shared/ui'
import { ModelContestantList, useWritingTestTranslation } from '@/features/writing-test'
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
} from 'react'
import {
  useModels,
  useSelections,
  useStageSelection,
  filterForStage,
  modelChoiceIssue,
  postCreditLabel,
  refKey,
  stagePostFigure,
  type CatalogModel,
} from '@/entities/model-catalog'
import { useVoices } from '@/entities/voice'
import { useTemplates, templateAskFields, TEMPLATE_PARSE_OPTIONS } from '@/entities/template'
import { useGuidelines, GuidelineFieldPicker } from '@/entities/guideline'
import {
  useCandidatePreparationClient,
  useWritingTestClient,
  useWritingTestOwnerRefresh,
  useWritingTestSource,
  useWritingTestSources,
  type TestCount,
  type PreparationCount,
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
  writingTestOperationBusy,
  WRITING_TEST_DIRECTION_MAX_CHARS,
  WRITING_TEST_NEW_GUIDELINE_SLOT,
} from '@/features/writing-test'
import {
  StepBar,
  StepBarCta,
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
import { WritingFieldLabel, WritingTestTemplateFacts } from './WritingTestTemplateFacts'
import { factAnswered, factFieldId, type FactGroup } from '../lib/template-facts'

const SETTING_KIND: Record<Exclude<TestFactor, 'model'>, TestSettingKind> = {
  voice: 'writing-voice',
  template: 'post-template',
  guideline: 'post-guideline',
}
const COUNTS: readonly TestCount[] = [2, 4, 8, 16]
/** A writing test's model factor is always the write stage; photo reading is its own test (MODEL-97). */
const MODEL_STAGE: TestStage = 'write'
interface SettingChoice {
  id: string
  name: string
  revision: string
  body?: string
  titleArea?: string
}
const emptySlot = (entrant: TestEntrant | undefined) =>
  !entrant ||
  (entrant.type === 'setting'
    ? !entrant.setting.id
    : entrant.type === 'authoring'
      ? !entrant.authoring.sessionId && !entrant.authoring.candidateId
      : !entrant.model.providerId && !entrant.model.modelId)

const choiceKey = (entrant: TestEntrant) =>
  entrant.type === 'model'
    ? refKey(entrant.model)
    : entrant.type === 'setting'
      ? entrant.setting.id
      : `ai:${entrant.authoring.sessionId}:${entrant.authoring.candidateId}:${entrant.authoring.revision}`

export interface WritingTestStudioProps {
  /** The page's heading, set beside ← on the studio's top row. */
  heading?: ReactNode
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
  heading,
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
    draft: {
      kind,
      count: plan.count,
      testCount: plan.count,
      retainedRefs: plan.entrants.filter(
        (entrant): entrant is Extract<TestEntrant, { type: 'authoring' }> =>
          entrant.type === 'authoring',
      ),
      prompt: '',
      writeModel: writer.selected ?? undefined,
    },
  })
  const source = useWritingTestSource(ownerId, plan.context.sourcePostSlug)
  const appliedSource = useRef(initialPlan.context.sourcePostSlug)
  const appliedPreparation = useRef('')
  const refreshedPublication = useRef('')
  const refreshOwner = useWritingTestOwnerRefresh(ownerId)
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
  const prepLocked = prepBusy || ['quoted', 'uncertain', 'created'].includes(preparation.phase)
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
      const figure = stagePostFigure(m, stage)
      const cost =
        m.access?.[stage]?.grade === 'free' ? t('free') : figure ? postCreditLabel(figure) : ''
      const named = cost ? `${m.label} · ${cost}` : m.label
      return {
        value: refKey(m.ref),
        label: issue ? `${named} · ${issue}` : named,
        disabled: !!issue,
      }
    })
  const modelEligibilityProblem =
    plan.factor === 'model' &&
    plan.entrants.some(
      (entrant) =>
        entrant.type === 'model' &&
        !filterForStage(models.models, MODEL_STAGE).some(
          (model) => refKey(model.ref) === refKey(entrant.model) && !modelIssue(model, MODEL_STAGE),
        ),
    )
  const writerFixed = plan.factor !== 'model'
  const observerFixed = plan.context.material.attachmentIds.length > 0
  const placeholder = useCallback(
    (): TestEntrant =>
      plan.factor === 'model'
        ? { type: 'model', model: { providerId: '', modelId: '' } }
        : { type: 'setting', setting: { kind, id: '', revision: '' } },
    [plan.factor, kind],
  )

  const choose = (position: number, value: string) => {
    if (prepLocked) return
    let entrant: TestEntrant | undefined
    const current = plan.entrants[position]
    if (current?.type === 'authoring' && choiceKey(current) === value) entrant = current
    else if (plan.factor === 'model') {
      const model = filterForStage(models.models, MODEL_STAGE).find((m) => refKey(m.ref) === value)
      if (model && !modelIssue(model, MODEL_STAGE)) entrant = { type: 'model', model: model.ref }
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
  const missingSlots = Array.from({ length: plan.count }, (_, index) => index).filter((index) =>
    emptySlot(plan.entrants[index]),
  )
  const assigned = plan.entrants.filter((entrant) => !emptySlot(entrant))
  const assignedProblem =
    assigned.some((entrant) =>
      entrant.type === 'model'
        ? !entrant.model.providerId || !entrant.model.modelId
        : entrant.type === 'setting'
          ? !entrant.setting.revision || entrant.setting.kind !== kind
          : !entrant.authoring.sessionId ||
            !entrant.authoring.candidateId ||
            entrant.authoring.revision <= 0,
    ) ||
    new Set(assigned.map(choiceKey)).size !== assigned.length ||
    modelEligibilityProblem
  const candidateProblem =
    missingSlots.length > 0 || plan.entrants.length !== plan.count || assignedProblem
  const preparedMetadata = useCallback(
    (entrant: Extract<TestEntrant, { type: 'authoring' }>) =>
      [...preparation.context.artifacts, ...(preparation.context.session?.candidates ?? [])].find(
        (candidate) => choiceKey(candidate.source) === choiceKey(entrant),
      ),
    [preparation.context.artifacts, preparation.context.session],
  )
  const askGroups = useMemo<FactGroup[]>(() => {
    // One group per distinct contestant template, in slot order; a fixed template is one group.
    const owners: { key: string; name: string; body?: string; titleArea?: string }[] = []
    if (plan.factor === 'template') {
      for (const entrant of plan.entrants) {
        if (entrant.type === 'setting') {
          const saved = choices.find((c) => c.id === entrant.setting.id)
          if (saved) owners.push({ ...saved, key: `setting:${saved.id}` })
        } else if (entrant.type === 'authoring') {
          const prepared = preparedMetadata(entrant)
          if (prepared)
            owners.push({
              key: choiceKey(entrant),
              name: prepared.name ?? t('preparedSlot'),
              body: prepared.body,
              titleArea: prepared.titleArea,
            })
        }
      }
    } else {
      const fixed = templates.templates.find((item) => item.id === plan.context.templateId)
      if (fixed) owners.push({ ...fixed, key: `fixed:${fixed.id}` })
    }
    // Two slots holding one template are one group.
    const groups = [...new Map(owners.map((owner) => [owner.key, owner])).values()]
      .map((owner) => ({
        key: owner.key,
        name: owner.name,
        asks: templateAskFields(owner.titleArea ?? '', owner.body ?? '', TEMPLATE_PARSE_OPTIONS),
      }))
      .filter((group) => group.asks.length > 0)
    const askedIn = new Map<string, number>()
    for (const group of groups)
      for (const ask of group.asks) askedIn.set(ask.label, (askedIn.get(ask.label) ?? 0) + 1)
    return groups.map((group) => ({
      ...group,
      asks: group.asks.map((ask) => ({
        label: ask.label,
        required: ask.required,
        prompt: ask.prompt,
        shared: (askedIn.get(ask.label) ?? 0) > 1,
      })),
    }))
  }, [
    plan.factor,
    plan.entrants,
    plan.context.templateId,
    choices,
    templates.templates,
    preparedMetadata,
    t,
  ])
  const sourcePending =
    !!plan.context.sourcePostSlug &&
    (source.isPending ||
      source.isError ||
      source.data?.context.sourcePostSlug !== plan.context.sourcePostSlug)
  // Every refusal of the material step in the order its field stands on screen: the step bar names
  // the first and counts them all, each field carries its own message, and a refused press moves
  // to the first (MODEL-99). A refusal with no field belongs to a choice of an earlier step.
  const [factTab, setFactTab] = useState('')
  const [focusRequest, setFocusRequest] = useState<{ fieldId: string } | null>(null)
  const { problems, fieldMessages } = (() => {
    const found: { fieldId: string | null; label: string; message: string; groupKey?: string }[] =
      []
    const messages: Record<string, string> = {}
    const refuse = (fieldId: string, label: string, message: string, groupKey?: string) => {
      messages[fieldId] = message
      found.push({ fieldId, label, message, groupKey })
    }
    const { material } = plan.context
    const hasAttachments = material.attachmentIds.length > 0
    if (!hasAttachments && !material.text.trim())
      refuse('test-material', t('material'), t('requiredMaterial'))
    const length = plan.context.targetLength
    if (
      !Number.isInteger(length) ||
      length < POST_TARGET_LENGTH_MIN ||
      length > POST_TARGET_LENGTH_MAX
    )
      refuse(
        'test-target-length',
        t('targetLength'),
        t('targetLengthBounds', { min: POST_TARGET_LENGTH_MIN, max: POST_TARGET_LENGTH_MAX }),
      )
    const tags = plan.context.tagCount
    if (!Number.isInteger(tags) || tags < POST_TAG_COUNT_MIN || tags > POST_TAG_COUNT_MAX)
      refuse(
        'test-tag-count',
        t('tags'),
        t('tagCountBounds', { min: POST_TAG_COUNT_MIN, max: POST_TAG_COUNT_MAX }),
      )
    const fixedModelMessage = (ref: WritingTestPlan['context']['writeModel'], stage: TestStage) => {
      if (!ref) return t('requiredChoice')
      const usable = filterForStage(models.models, stage).some(
        (model) => refKey(model.ref) === refKey(ref) && !modelIssue(model, stage),
      )
      return usable
        ? ''
        : t(hasVideo && stage === 'observe' ? 'unsupportedVideo' : 'ineligibleModel')
    }
    const writerMessage = writerFixed ? fixedModelMessage(plan.context.writeModel, 'write') : ''
    if (writerMessage) refuse('test-fixed-writer', t('fixedWriter'), writerMessage)
    const observerMessage = observerFixed
      ? fixedModelMessage(plan.context.observeModel, 'observe')
      : ''
    if (observerMessage) refuse('test-fixed-observer', t('fixedObserver'), observerMessage)
    if (plan.factor === 'guideline' && !plan.context.guidelineSlotId.trim())
      refuse('test-guideline-slot', t('guidelineSlot'), t('requiredChoice'))
    const askedLabels = new Set<string>()
    askGroups.forEach((group, groupIndex) =>
      group.asks.forEach((ask) => {
        if (!ask.required || factAnswered(material.templateAnswers, ask.label)) return
        const fieldId = factFieldId(groupIndex, ask.label)
        // A question two templates share is one problem, marked in both of its groups.
        if (askedLabels.has(ask.label)) messages[fieldId] = t('requiredInput')
        else refuse(fieldId, ask.label, t('requiredInput'), group.key)
        askedLabels.add(ask.label)
      }),
    )
    if (modelEligibilityProblem)
      found.push({
        fieldId: null,
        label: '',
        message: t(hasVideo ? 'unsupportedVideo' : 'ineligibleModel'),
      })
    return { problems: found, fieldMessages: attempted ? messages : {} }
  })()
  const shownProblems = attempted ? problems : []
  const materialReason = !shownProblems.length
    ? ''
    : !shownProblems[0].fieldId
      ? shownProblems[0].message
      : shownProblems.length === 1
        ? t('fixOne', { first: shownProblems[0].label })
        : t('fixMany', { first: shownProblems[0].label, count: shownProblems.length })
  useEffect(() => {
    if (!focusRequest) return
    // The tab switch has committed by the next frame, so the field is mounted to be reached.
    const frame = window.requestAnimationFrame(() => {
      const field = document.getElementById(focusRequest.fieldId)
      field?.focus({ preventScroll: true })
      field?.scrollIntoView?.({ block: 'center' })
    })
    return () => window.cancelAnimationFrame(frame)
  }, [focusRequest])
  useEffect(() => {
    if (flow.phase !== 'editing' || flow.context.test) return
    const next = { ...plan.context }
    let changed = false
    if (plan.factor !== 'model' && !next.writeModel && writer.selected) {
      next.writeModel = writer.selected
      changed = true
    }
    if (!next.observeModel && observer.selected) {
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
      (preparation.context.draft.testCount ?? session.count) !== plan.count ||
      flow.phase !== 'editing'
    )
      return
    const key = JSON.stringify([session.sessionId, session.revision])
    if (key === appliedPreparation.current) return
    appliedPreparation.current = key
    const indices =
      preparation.context.draft.slotIndices ??
      Array.from({ length: session.count }, (_, index) => index)
    if (indices.length !== session.count || indices.some((index) => index >= plan.count)) return
    const entrants = Array.from(
      { length: plan.count },
      (_, index) => plan.entrants[index] ?? placeholder(),
    )
    let changed = false
    indices.forEach((position, index) => {
      if (emptySlot(entrants[position])) {
        entrants[position] = session.candidates[index].source
        changed = true
      }
    })
    if (changed) {
      draft({ ...plan, entrants })
      if (flow.view === 'candidates' && entrants.every((entrant) => !emptySlot(entrant))) {
        flow.sendPresentation({ type: 'NEXT' })
      }
    }
  }, [
    preparation.phase,
    preparation.context.session,
    preparation.context.draft,
    plan,
    flow,
    draft,
    placeholder,
  ])
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
    void refreshOwner(test, publication)
    if (publication.action === 'apply-output') void source.refetch()
    else if (test.factor === 'model') activeSelections.refetch()
    else if (test.factor === 'voice') void voices.refetch()
    else if (test.factor === 'template') void templates.refetch()
    else void guidelines.refetch()
  }, [
    flow.context.publication,
    test,
    source,
    activeSelections,
    voices,
    templates,
    guidelines,
    refreshOwner,
  ])

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
                ? t(`winnerTitle.${test?.factor ?? plan.factor}`)
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
  const choicesReason =
    attempted && (plan.factor === 'model' ? candidateProblem : assignedProblem)
      ? t('requiredChoices', { count: plan.count })
      : ''
  const next = () => {
    setAttempted(true)
    if (flow.view === 'candidates') {
      if (plan.factor === 'model' ? candidateProblem : assignedProblem) return
      if (plan.factor !== 'model' && missingSlots.length) {
        if (prepLocked || !writer.selected) return
        setAttempted(false)
        preparation.send({
          type: 'EDIT',
          draft: {
            kind,
            count: missingSlots.length as PreparationCount,
            testCount: plan.count,
            slotIndices: missingSlots,
            retainedRefs: plan.entrants.filter(
              (entrant): entrant is Extract<TestEntrant, { type: 'authoring' }> =>
                entrant.type === 'authoring',
            ),
            prompt: preparation.context.draft.prompt.trim() || t(`defaultDirection.${plan.factor}`),
            writeModel: writer.selected,
          },
        })
        preparation.send({ type: 'ESTIMATE' })
        return
      }
    }
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
    <div className="flex min-w-0 flex-1 flex-col">
      <div className="flex min-h-11 flex-wrap items-center gap-2">
        <ContextualReturn />
        {heading && <div className="min-w-0 flex-1">{heading}</div>}
      </div>
      <Typography
        variant="stepTitle"
        id={headingId}
        as="h2"
        tabIndex={-1}
        className="mt-3 break-words sm:mt-6"
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
      {flow.context.failure && flow.view !== 'material' && (
        <Notice tone="danger" role="alert" className="mt-4">
          <AppFailureMessage failure={flow.context.failure} />
        </Notice>
      )}
      {flow.view === 'factor' && (
        <section className="mt-4 flex flex-1 flex-col gap-4 sm:mt-5 sm:gap-6">
          <Typography variant="body" id={`${headingId}-help`} className="max-w-measure">
            {t('factorHelp')}
          </Typography>
          <div className="grid grid-cols-2 gap-3">
            {(['model', 'voice', 'template', 'guideline'] as const).map((factor) => (
              <ChoiceButton
                key={factor}
                title={t(`factor.${factor}`)}
                aria-describedby={`${headingId}-help`}
                selected={factor === plan.factor}
                // A tap only chooses; what the choice compares shows under the tiles and 다음
                // moves on (THEME-52).
                onClick={() => {
                  if (factor !== plan.factor)
                    draft({
                      ...plan,
                      factor,
                      modelStage: MODEL_STAGE,
                      entrants: [],
                      context: {
                        ...plan.context,
                        voiceId: factor === 'voice' ? '' : plan.context.voiceId,
                        templateId: factor === 'template' ? '' : plan.context.templateId,
                        guidelineSlotId:
                          factor === 'guideline' ? WRITING_TEST_NEW_GUIDELINE_SLOT : '',
                        writeModel: factor === 'model' ? undefined : (writer.selected ?? undefined),
                      },
                    })
                  setAttempted(false)
                }}
              />
            ))}
          </div>
          <div role="status" className="bg-card-well rounded-xl p-4">
            <Typography variant="fieldTitle" as="p">
              {t(`factor.${plan.factor}`)}
            </Typography>
            <Typography variant="body" as="p" className="text-content-secondary mt-1">
              {t(`factorWhat.${plan.factor}`)}
            </Typography>
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
          <StepBar ariaLabel={title}>
            <StepBarActions>
              <StepBarCta
                forward
                onClick={() => {
                  flow.sendPresentation({ type: 'NEXT' })
                  setAttempted(false)
                }}
              >
                {t('next')}
              </StepBarCta>
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'candidates' && (
        <section className="mt-5 flex flex-1 flex-col gap-6">
          <WritingSelect
            label={t('format')}
            disabled={prepLocked}
            value={String(plan.count)}
            options={COUNTS.map((count) => ({
              value: String(count),
              label: t(`formatLabel.${count}`),
            }))}
            onChange={(value) => {
              draft({
                ...plan,
                count: Number(value) as TestCount,
                entrants: plan.entrants.slice(0, Number(value)),
              })
              appliedPreparation.current = ''
            }}
          />
          {plan.factor !== 'model' && <Typography variant="body">{t('slotHelp')}</Typography>}
          {
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
              {plan.factor === 'model' ? (
                <ModelContestantList
                  id="test-model-contestants"
                  stage={MODEL_STAGE}
                  count={plan.count}
                  models={models.models}
                  selected={plan.entrants.flatMap((entrant) =>
                    entrant.type === 'model' && !emptySlot(entrant) ? [entrant.model] : [],
                  )}
                  issue={(model) => modelIssue(model, MODEL_STAGE)}
                  disabled={prepLocked}
                  onChange={(refs) =>
                    draft({
                      ...plan,
                      modelStage: MODEL_STAGE,
                      entrants: refs.map((model) => ({ type: 'model', model })),
                    })
                  }
                />
              ) : (
                <div className="grid min-w-0 grid-cols-2 gap-4">
                  {Array.from({ length: plan.count }, (_, index) => (
                    <WritingSelect
                      key={index}
                      label={t('contender', { number: index + 1 })}
                      value={emptySlot(plan.entrants[index]) ? '' : choiceKey(plan.entrants[index])}
                      options={[
                        { value: '', label: t('autoAI') },
                        ...(plan.entrants[index]?.type === 'authoring'
                          ? [
                              {
                                value: choiceKey(plan.entrants[index]),
                                label: `${t('generatedCandidate')} · ${preparedMetadata(plan.entrants[index])?.name ?? t('preparedSlot')}`,
                              },
                            ]
                          : []),
                        ...choices.map((c) => ({ value: c.id, label: c.name })),
                      ]}
                      disabled={prepLocked}
                      onChange={(value) => choose(index, value)}
                    />
                  ))}
                </div>
              )}
              {!choices.length && plan.factor !== 'model' && (
                <Typography variant="body">{t('emptyChoices')}</Typography>
              )}
              {plan.factor === 'model' && !modelOptions(MODEL_STAGE).length && (
                <Typography variant="body">
                  {t('noModels')}{' '}
                  <a href="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
                    {t('settings')}
                  </a>
                </Typography>
              )}
            </>
          }
          {plan.factor !== 'model' &&
            (missingSlots.length > 0 || prepLocked || preparation.phase === 'failed') && (
              <section className="space-y-4">
                <Typography variant="body" role="status">
                  {t('aiSlots', { count: missingSlots.length })}
                </Typography>
                <Typography variant="body">{t('preparationHelp')}</Typography>
                <FieldLabel htmlFor="test-candidate-direction">{t('direction')}</FieldLabel>
                <Textarea
                  id="test-candidate-direction"
                  autoGrow
                  rows={2}
                  value={preparation.context.draft.prompt}
                  maxLength={WRITING_TEST_DIRECTION_MAX_CHARS}
                  placeholder={t('directionHint')}
                  disabled={prepLocked}
                  onChange={(event) =>
                    preparation.send({
                      type: 'EDIT',
                      draft: {
                        kind,
                        count: (missingSlots.length || plan.count) as PreparationCount,
                        testCount: plan.count,
                        retainedRefs: plan.entrants.filter(
                          (entrant): entrant is Extract<TestEntrant, { type: 'authoring' }> =>
                            entrant.type === 'authoring',
                        ),
                        writeModel: writer.selected ?? undefined,
                        prompt: event.target.value,
                      },
                    })
                  }
                />
                {preparation.context.failure && (
                  <Notice tone="danger" role="alert">
                    <AppFailureMessage failure={preparation.context.failure} />
                  </Notice>
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
                  <Typography variant="body">
                    {t('preparationTitle', {
                      kind: t(`factor.${plan.factor}`),
                      count: preparation.context.draft.count,
                    })}{' '}
                    {preparation.context.estimate?.free
                      ? t('free')
                      : t('credits', { count: preparation.context.estimate?.credits ?? 0 })}
                  </Typography>
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
              </section>
            )}
          <StepBar ariaLabel={title}>
            <StepBarProblem id={`${headingId}-choices-reason`} text={choicesReason} />
            <StepBarActions className="flex-wrap">
              <Button
                variant="secondary"
                size="icon"
                aria-label={t('back')}
                title={t('back')}
                disabled={prepBusy || ['uncertain', 'created'].includes(preparation.phase)}
                onClick={() =>
                  preparation.phase === 'quoted'
                    ? preparation.send({ type: 'BACK' })
                    : flow.sendPresentation({ type: 'BACK' })
                }
              >
                <ReturnIcon />
              </Button>
              {['quoted', 'created'].includes(preparation.phase) && plan.factor !== 'model' ? (
                <Button
                  variant="cta"
                  size="hero"
                  className="min-w-0 flex-1"
                  onClick={() => preparation.send({ type: 'CONFIRM' })}
                >
                  {t('prepare', { count: preparation.context.draft.count })}
                </Button>
              ) : (
                !prepLocked && (
                  <Button
                    variant="cta"
                    size="hero"
                    className="min-w-0 flex-1"
                    onClick={next}
                    aria-describedby={choicesReason ? `${headingId}-choices-reason` : undefined}
                    disabled={
                      plan.factor !== 'model' && missingSlots.length > 0 && !writer.selected
                    }
                  >
                    {t('next')}
                  </Button>
                )
              )}
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'material' && (
        <section className="mt-5 flex flex-1 flex-col gap-5">
          {preparation.phase === 'ready' &&
            preparation.context.session &&
            plan.entrants.some(
              (entrant) =>
                entrant.type === 'authoring' &&
                entrant.authoring.sessionId === preparation.context.session?.sessionId,
            ) && (
              <Typography variant="body" role="status">
                {t('prepared', { count: preparation.context.session.count })}
              </Typography>
            )}
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
          <div className="space-y-2">
            <WritingFieldLabel
              htmlFor="test-material"
              required={!plan.context.material.attachmentIds.length}
            >
              {t('material')}
            </WritingFieldLabel>
            <Textarea
              id="test-material"
              autoGrow
              viewportAllocation={{ reservedBottom: 176 }}
              rows={6}
              value={plan.context.material.text}
              aria-invalid={fieldMessages['test-material'] ? true : undefined}
              aria-describedby={
                fieldMessages['test-material'] ? 'test-material-message' : undefined
              }
              onChange={(e) =>
                context({ material: { ...plan.context.material, text: e.target.value } })
              }
            />
            {fieldMessages['test-material'] && (
              <FieldMessage id="test-material-message">
                {fieldMessages['test-material']}
              </FieldMessage>
            )}
          </div>
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
              fieldId="test-target-length"
              required
              message={fieldMessages['test-target-length']}
              label={t('targetLength')}
              type="number"
              min={POST_TARGET_LENGTH_MIN}
              max={POST_TARGET_LENGTH_MAX}
              value={plan.context.targetLength || ''}
              onChange={(e) => context({ targetLength: Number(e.target.value) })}
            />
            <WritingOptionField
              fieldId="test-tag-count"
              required
              message={fieldMessages['test-tag-count']}
              label={t('tags')}
              type="number"
              min={POST_TAG_COUNT_MIN}
              max={POST_TAG_COUNT_MAX}
              value={plan.context.tagCount}
              onChange={(e) => context({ tagCount: Number(e.target.value) })}
            />
          </div>
          {writerFixed && (
            <WritingSelect
              fieldId="test-fixed-writer"
              required
              message={fieldMessages['test-fixed-writer']}
              label={t('fixedWriter')}
              value={plan.context.writeModel ? refKey(plan.context.writeModel) : ''}
              options={[{ value: '', label: t('choose') }, ...modelOptions('write')]}
              onChange={(value) =>
                context({ writeModel: models.models.find((m) => refKey(m.ref) === value)?.ref })
              }
            />
          )}
          {observerFixed && (
            <WritingSelect
              fieldId="test-fixed-observer"
              required
              message={fieldMessages['test-fixed-observer']}
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
                fieldId="test-guideline-slot"
                required
                message={fieldMessages['test-guideline-slot']}
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
          <WritingTestTemplateFacts
            groups={askGroups}
            answers={plan.context.material.templateAnswers}
            messages={fieldMessages}
            activeKey={factTab}
            onActiveKey={setFactTab}
            onAnswer={(label, text) =>
              context({
                material: {
                  ...plan.context.material,
                  templateAnswers: plan.context.material.templateAnswers.some(
                    (answer) => answer.label === label,
                  )
                    ? plan.context.material.templateAnswers.map((answer) =>
                        answer.label === label ? { ...answer, text, enabled: true } : answer,
                      )
                    : [...plan.context.material.templateAnswers, { label, text, enabled: true }],
                },
              })
            }
          />
          <StepBar ariaLabel={title}>
            {flow.context.failure && (
              <Notice tone="danger" role="alert">
                <AppFailureMessage failure={flow.context.failure} />
              </Notice>
            )}
            <StepBarProblem id={`${headingId}-material-reason`} text={materialReason} />
            <StepBarActions className="flex-wrap">
              <Button
                variant="secondary"
                size="icon"
                aria-label={t('back')}
                title={t('back')}
                onClick={() => flow.sendPresentation({ type: 'BACK' })}
              >
                <ReturnIcon />
              </Button>
              <Button
                variant="cta"
                size="hero"
                className="min-w-0 flex-1"
                pending={flow.phase === 'quoting'}
                disabled={sourcePending}
                aria-describedby={materialReason ? `${headingId}-material-reason` : undefined}
                onClick={() => {
                  setAttempted(true)
                  if (sourcePending) return
                  const first = problems[0]
                  if (!first) {
                    flow.send({ type: 'ESTIMATE' })
                    return
                  }
                  if (first.groupKey) setFactTab(first.groupKey)
                  if (first.fieldId) setFocusRequest({ fieldId: first.fieldId })
                }}
              >
                {t('estimatePosts', { count: plan.count })}
              </Button>
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'estimate' && (
        <section className="mt-5 flex flex-1 flex-col gap-5">
          <Typography variant="body">{t('onceOnly')}</Typography>
          <Typography variant="title">
            {flow.context.quote?.free
              ? t('free')
              : t('credits', { count: flow.context.quote?.credits ?? 0 })}
          </Typography>
          <StepBar ariaLabel={title}>
            <StepBarActions className="flex-wrap">
              <Button
                variant="secondary"
                size="icon"
                aria-label={t('back')}
                title={t('back')}
                onClick={() => flow.sendPresentation({ type: 'BACK' })}
              >
                <ReturnIcon />
              </Button>
              <Button
                variant="cta"
                size="hero"
                className="min-w-0 flex-1"
                onClick={() => flow.send({ type: 'CONFIRM' })}
              >
                {t(flow.context.retryEstimate ? 'retry' : 'start', { count: quoteCount })}
              </Button>
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'work' && (
        <section className="mt-5 flex flex-1 flex-col gap-5">
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
            ownerId={ownerId}
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
        <section className="mt-5 flex flex-1 flex-col gap-5">
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
          <WritingTestChampionOutput test={test} ownerId={ownerId} />
        </section>
      )}
      {flow.view === 'publication' && test && winner && (
        <section className="mt-5 flex flex-1 flex-col gap-5">
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
          <StepBar ariaLabel={title}>
            <StepBarActions className="flex-wrap">
              <Button
                variant="secondary"
                size="icon"
                aria-label={t('back')}
                title={t('back')}
                onClick={() => flow.sendPresentation({ type: 'BACK' })}
              >
                <ReturnIcon />
              </Button>
              <Button
                variant="cta"
                size="hero"
                className="min-w-0 flex-1"
                pending={flow.phase === 'publishing'}
                onClick={publish}
              >
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
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'recovery' && (
        <section className="mt-5 flex flex-1 flex-col gap-5">
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
          {test && winner && <WritingTestChampionOutput test={test} ownerId={ownerId} />}
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

/** The refusal above a step bar's row. Its live region is mounted before the text arrives so the
 *  reason is announced, and it collapses while empty so the bar keeps its height (THEME-24/71). */
function StepBarProblem({ id, text }: { id: string; text: string }) {
  return (
    <div aria-live="polite" className="empty:hidden">
      {text ? <StepBarReason id={id}>{text}</StepBarReason> : null}
    </div>
  )
}

/** A form control's refusal, stood under the control with the control marked invalid (THEME-29). */
interface FieldProblemProps {
  /** The control's id, when a refused press has to reach it. */
  fieldId?: string
  required?: boolean
  message?: string
}

function WritingOptionField({
  label,
  fieldId,
  required,
  message,
  ...props
}: { label: string } & FieldProblemProps & ComponentProps<typeof TextField>) {
  const generated = useId()
  const id = fieldId ?? generated
  return (
    <div className="space-y-2">
      <WritingFieldLabel htmlFor={id} required={required}>
        {label}
      </WritingFieldLabel>
      <TextField
        id={id}
        aria-invalid={message ? true : undefined}
        aria-describedby={message ? `${id}-message` : undefined}
        {...props}
      />
      {message && <FieldMessage id={`${id}-message`}>{message}</FieldMessage>}
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
  fieldId,
  required,
  message,
  ...props
}: { label: string } & FieldProblemProps & ComponentProps<typeof Listbox<T>>) {
  const generated = useId()
  const id = fieldId ?? generated
  return (
    <div className="space-y-2">
      <WritingFieldLabel id={`${id}-label`} htmlFor={id} required={required}>
        {label}
      </WritingFieldLabel>
      <Listbox
        id={id}
        aria-labelledby={`${id}-label`}
        aria-invalid={message ? true : undefined}
        aria-describedby={message ? `${id}-message` : undefined}
        {...props}
      />
      {message && <FieldMessage id={`${id}-message`}>{message}</FieldMessage>}
    </div>
  )
}
