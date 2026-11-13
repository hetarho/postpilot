import { cardStyles } from '@/shared/ui'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  CLIP_PROJECT_LIMITS,
  DEFAULT_CLIP_CHANNEL,
  type ClipChannel,
  ClipCompositionInputFields,
  boundedFreeText,
  emptyClipProject,
  emptyCompositionInputs,
  freeTextLength,
  matchingCompositionInputs,
  normalizeClipProject,
  projectCompositionDocument,
  projectDraft,
  savableClipProject,
  serialClipWrite,
  type ClipProject,
  type ClipProjectDraft,
  useClipProjectMutations,
  retainMintedClipEntry,
  clipChannelPath,
  readClipReturnContext,
  validClipProject,
} from '@/entities/clip-project'
import { useSpokenLibrary } from '@/entities/spoken-voice'
import {
  AdvertiserGuideFields,
  AdvertiserGuideSuggestions,
  EMPTY_GUIDE,
} from '@/entities/advertiser-guide'
import { clipKeywordCoverage } from '../model/requirements'
import type { ClipPreparationContent } from '../model/preparation'
import { clipPreparationRows } from './ClipPreparationSummary'
import { CLIP_ACCENTS, useClipTemplates } from '@/entities/clip-template'
import { CLIP_DEFAULT_REGION_PRESETS, CLIP_DESIGN } from '@/entities/clip-design'
import { appFailureFromConnect } from '@/shared/api'
import {
  mintClipDraft,
  mintedClipId,
  newClipDraftKey,
  peekPendingClipDraft,
  queueClipDraft,
  recordMintedClip,
} from '../model/clip-draft-queue'
import { restoreClipCaret } from '../model/mint-handoff'
import { MoveClipChannel } from './MoveClipChannel'
import { ClipLinkedPost } from './ClipLinkedPost'
import { ClipDesignSelection, type ClipRegionSwitches } from './ClipDesignSelection'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Checkbox,
  ChoiceButton,
  Disclosure,
  useMediaQuery,
  MD_MEDIA_QUERY,
  FieldCount,
  FieldLabel,
  FieldMessage,
  Listbox,
  SegmentedControl,
  Textarea,
  TextField,
  Typography,
  buttonStyles,
} from '@/shared/ui'

const INPUT = {
  autoComplete: 'off',
  autoCapitalize: 'sentences',
  autoCorrect: 'on',
  enterKeyHint: 'next',
} as const
// Channel moves use their own write. They must not make a successfully autosaved
// setup draft look dirty or prevent a later canonical settings read from landing.
function draftIdentity(value: ClipProjectDraft): string {
  const comparable = normalizeClipProject(value)
  delete comparable.channel
  return JSON.stringify(comparable)
}

export function ClipProjectForm({
  ownerId,
  stored,
  channel = DEFAULT_CLIP_CHANNEL,
  onUploadAllowed,
  children,
  disabled = false,
  readOnly = false,
  actions,
  refusal,
  regions,
  dubbing,
  initialTemplateId,
  layout,
  onMinted,
  unmintedKey: givenKey,
}: {
  ownerId: string
  /** `/clips/new` only: the queue key the page reads its save status from (CLIP-130). */
  unmintedKey?: string
  /** `/clips/new` only: the first save minted this project (CLIP-130). */
  onMinted?: (id: string) => void
  layout?: (content: ClipPreparationContent) => ReactNode
  /** `/clips/new` only: the video template the new-clip offer's wizard just saved (CLIP-223). */
  initialTemplateId?: string
  dubbing?: (
    value: NonNullable<ClipProjectDraft['dubbing']>,
    change: (value: NonNullable<ClipProjectDraft['dubbing']>) => void,
  ) => ReactNode
  stored?: ClipProject
  channel?: ClipChannel
  /** Whether each region is used, and its switch — held by the workspace, which saves region
   *  edits for ① and ② alike (CLIP-111). */
  regions?: ClipRegionSwitches
  onUploadAllowed?: (allowed: boolean) => void
  children?: ReactNode
  disabled?: boolean
  /** A finalized project reads its settings here rather than editing them (CLIP-160): the fields
   *  state what the clip was made with and nothing on the panel commits anything. */
  readOnly?: boolean
  actions?: (ready: boolean, dirty: boolean) => ReactNode
  refusal?: ReactNode
}) {
  const { t } = useTranslation('clips')
  const wide = useMediaQuery(MD_MEDIA_QUERY)
  const [designExpanded, setDesignExpanded] = useState({ phone: false, desktop: true })
  const templates = useClipTemplates(ownerId)
  // An unminted `/clips/new` queues under its own key until its first save mints the project,
  // as a post with no slug does (CLIP-130, POST-7).
  const [unmintedKey] = useState(() => (stored ? '' : (givenKey ?? newClipDraftKey())))
  const queueKey = stored?.id ?? unmintedKey
  // A draft the queue still owes the server outranks what the server last reported: it is what
  // the previous mount of this form was in the middle of saving when a step change unmounted it
  // (CLIP-39). Read, not consumed — a component body may run more than once per mount.
  const queued = stored ? peekPendingClipDraft(stored.id) : undefined
  const [draft, setDraft] = useState<ClipProjectDraft>(
    () => queued ?? (stored ? projectDraft(stored) : { ...emptyClipProject(), channel }),
  )
  const [seconds, setSeconds] = useState(
    draft.targetDurationMs ? String(draft.targetDurationMs / 1000) : '',
  )
  const [baseline, setBaseline] = useState(draftIdentity(stored ? projectDraft(stored) : draft))
  const { save } = useClipProjectMutations(ownerId)
  // The workspace the mint's address change mounts puts the caret back where it was (POST-11).
  const restoredFor = useRef('')
  useEffect(() => {
    if (!stored || restoredFor.current === stored.id) return
    restoredFor.current = stored.id
    restoreClipCaret(stored.id)
  }, [stored])
  const selected = templates.templates.find((v) => v.id === draft.videoTemplateId)
  const bodyFor = (next: ClipProjectDraft) => {
    const template = templates.templates.find((v) => v.id === next.videoTemplateId)
    if (template?.compositionBody) return template.compositionBody
    return stored?.videoTemplateId === next.videoTemplateId
      ? stored.composition?.snapshot.body
      : template?.compositionBody
  }
  const document = projectCompositionDocument(bodyFor(draft))
  const templateChanged =
    !!stored?.composition &&
    selected?.id === stored.videoTemplateId &&
    selected.compositionBody !== stored.composition.snapshot.body
  const inputs = document
    ? matchingCompositionInputs(document, draft.compositionInputs ?? emptyCompositionInputs())
    : emptyCompositionInputs()
  const valid = validClipProject(draft, document)
  const savable = savableClipProject(draft)
  const dirty = draftIdentity(draft) !== baseline
  const storedJSON = stored ? draftIdentity(stored) : baseline
  const synced = storedJSON === baseline
  // A save's answer moves `baseline` at once, but the page hands the answered project down a
  // render later: for that render `stored` is still the project from BEFORE the save, and adopting
  // it flipped the fields back and forth — the campaign guide unmounted and came back with its
  // `셀 수 있는 항목` closed and an item being typed gone (CLIP-39). So the project this form was
  // last given is remembered with whether an answer has superseded it: a superseded project is
  // never adopted, and the first one given after the answer is, as saved, while the owner has
  // typed nothing since.
  const [seen, setSeen] = useState({ stored: storedJSON, superseded: false })
  const storedChanged = seen.stored !== storedJSON
  if (storedChanged) setSeen({ stored: storedJSON, superseded: false })
  const superseded = seen.superseded && !storedChanged
  const answered = seen.superseded && storedChanged
  // The FIELDS stay open while an autosave is in flight. Disabling a `fieldset` disables every
  // control inside it, and a browser blurs a control it disables — so locking the form on each
  // save threw the owner out of the field they were typing in, a beat after every pause. Nothing
  // is lost by leaving them open: the queue sends the LATEST draft, so a keystroke made during a
  // flight goes out after it rather than racing it (CLIP-39) — `/clips/new`'s minting save too.
  const locked = disabled || readOnly
  if ((!synced || answered) && !dirty && !save.isPending && !superseded) {
    const refreshed = JSON.parse(storedJSON) as ClipProjectDraft
    setDraft(refreshed)
    setSeconds(refreshed.targetDurationMs ? String(refreshed.targetDurationMs / 1000) : '')
    setBaseline(storedJSON)
  }
  // Neither the source picker nor ①'s committing actions wait for the autosave: gating them on
  // unsaved state unmounted the open quote on every keystroke. Each committing action flushes the
  // queue before it starts instead, so nothing is committed against settings the server has not
  // taken (CLIP-39).
  useEffect(() => {
    onUploadAllowed?.(savable && !disabled && !templateChanged)
  }, [savable, disabled, templateChanged, onUploadAllowed])
  /** The queue's one way to reach the server, and the one place `baseline` moves for an autosave.
   *  A form unmounted before the answer lands simply does not move it — the next mount derives it
   *  from the refreshed `stored` instead. */
  const send = async (next: ClipProjectDraft) => {
    if (!stored) return sendUnminted(next)
    // In the project's write lane: a preset change projects into the plan and the regions, so
    // it must not cross a correction or slot save in flight (CLIP-188).
    const value = await serialClipWrite(stored.id, () =>
      save.mutateAsync({ id: stored.id, draft: next }),
    )
    setBaseline(draftIdentity(value))
    setSeen((current) => ({ ...current, superseded: true }))
  }
  /** `/clips/new`'s sender: the first send creates the project in the channel the page was opened
   *  from with the whole queued draft, an empty title included; every later one — an edit made
   *  while the create was in flight among them — updates what it minted (CLIP-130, POST-4). */
  const sendUnminted = async (next: ClipProjectDraft) => {
    const minted = mintedClipId(unmintedKey)
    if (minted) {
      await serialClipWrite(minted, () => save.mutateAsync({ id: minted, draft: next }))
      return
    }
    const origin = readClipReturnContext(ownerId) ?? {
      version: 1 as const,
      ownerKey: ownerId,
      path: clipChannelPath(channel),
      section: channel,
      filters: { channel },
      scrollY: 0,
      targetId: 'new',
    }
    const value = await save.mutateAsync({ draft: { ...next, channel: next.channel ?? channel } })
    recordMintedClip(unmintedKey, value.id)
    retainMintedClipEntry(ownerId, value.id, undefined, origin)
    onMinted?.(value.id)
  }
  const patch = (fields: Partial<ClipProjectDraft>) => {
    const next = { ...draft, ...fields }
    setDraft(next)
    // An invalid draft is never sent: the server would refuse it, and the field says so itself.
    // It stays local until it is valid again, and then goes out with everything else.
    if (!readOnly && savableClipProject(next)) queueClipDraft(queueKey, next, send)
  }
  /** Mints an unminted clip now with what it holds, for a source picked before anything else
   *  saved; a minted one answers with its id (CLIP-130). */
  const mint = (): Promise<string> =>
    stored ? Promise.resolve(stored.id) : mintClipDraft(unmintedKey, draft, send)
  const change = <K extends keyof ClipProjectDraft>(key: K, value: ClipProjectDraft[K]) =>
    patch(
      key === 'videoTemplateId' && value !== draft.videoTemplateId
        ? { [key]: value, compositionInputs: emptyCompositionInputs(), ...designOf(String(value)) }
        : { [key]: value },
    )
  // Choosing a template takes its design selection, silently — the owner saw that design in its
  // preview — and the server does the same in the same write, so the draft already says what the
  // save answers. Clearing to 없음 keeps the project's own (CLIP-168).
  const designOf = (id: string): Partial<ClipProjectDraft> => {
    const template = templates.templates.find((v) => v.id === id)
    return template
      ? {
          introPreset: template.introPreset,
          outroPreset: template.outroPreset,
          allowedCaptionStyles: [...template.allowedCaptionStyles],
        }
      : {}
  }
  // A handed template is selected once the directory has it, exactly as choosing it would be, and
  // it is the baseline too: arriving with it is not an unsaved change the leave guard should ask
  // about. An id the directory lacks leaves 없음.
  const [handedApplied, setHandedApplied] = useState(false)
  if (
    !stored &&
    initialTemplateId &&
    !handedApplied &&
    templates.templates.some((template) => template.id === initialTemplateId)
  ) {
    const next = {
      ...draft,
      videoTemplateId: initialTemplateId,
      compositionInputs: emptyCompositionInputs(),
      ...designOf(initialTemplateId),
    }
    setHandedApplied(true)
    setDraft(next)
    setBaseline(draftIdentity(next))
  }
  const failure = save.error
  const guided = !!layout && !readOnly
  const voiceLibrary = useSpokenLibrary(
    draft.dubbing?.enabled || draft.dubbing?.voiceId ? ownerId : '',
    false,
    false,
  )
  const voiceReady =
    !draft.dubbing?.enabled ||
    (!voiceLibrary.voicesQuery.isPending &&
      !voiceLibrary.voicesQuery.isError &&
      voiceLibrary.voices.some((voice) => voice.id === draft.dubbing?.voiceId && !voice.removedAt))
  const titleField = (
    <>
      <div>
        <FieldLabel htmlFor="clip-title">{t('project.name')}</FieldLabel>
        <TextField
          id="clip-title"
          type="text"
          inputMode="text"
          {...INPUT}
          value={draft.title}
          placeholder={t('project.titlePlaceholder')}
          aria-describedby="clip-title-hint"
          onChange={(event) => change('title', event.target.value)}
          aria-invalid={Array.from(draft.title.trim()).length > CLIP_PROJECT_LIMITS.title}
        />
        {/* The title is optional, on a new clip too (CLIP-130). */}
        <Typography id="clip-title-hint" variant="body" className="text-content-secondary mt-2">
          {t('project.titleHint')}
        </Typography>
        {Array.from(draft.title.trim()).length > CLIP_PROJECT_LIMITS.title && (
          <FieldMessage>{t('project.titleLimit')}</FieldMessage>
        )}
      </div>
    </>
  )
  const templateListbox = (
    <>
      <div>
        <FieldLabel id="clip-template-label" htmlFor="clip-template">
          {t('project.template')}
        </FieldLabel>
        <Listbox
          id="clip-template"
          aria-labelledby="clip-template-label"
          value={draft.videoTemplateId}
          onChange={(value) => change('videoTemplateId', value)}
          options={[
            { value: '', label: t('project.chooseTemplate') },
            ...templates.templates.map((v) => ({ value: v.id, label: v.name })),
          ]}
        />
        <Typography variant="body" className="text-content-secondary mt-2">
          {t('project.templateOptional')}
        </Typography>
        {templates.isPending && <Typography variant="body">{t('project.loading')}</Typography>}
        {templates.isError && (
          <div role="alert">
            <AppFailureMessage failure={appFailureFromConnect(templates.error)} />
            <Button variant="ghost" onClick={() => void templates.refetch()}>
              {t('project.retry')}
            </Button>
          </div>
        )}
        {!templates.isPending && !templates.isError && !templates.templates.length && (
          <Typography variant="body" className="mt-2">
            {t('project.noTemplates')}{' '}
            <Link to="/video-templates/new" className={buttonStyles({ variant: 'ghost' })}>
              {t('directory.create')}
            </Link>
          </Typography>
        )}
        {!selected && !!draft.videoTemplateId && (
          <FieldMessage>{t('project.detachedTemplate')}</FieldMessage>
        )}
      </div>
    </>
  )
  const answerFields = (
    <>
      {templateChanged && (
        <section className="space-y-3">
          <Typography variant="body">{t('composition.templateChanged')}</Typography>
          <Button
            variant="ghost"
            disabled={
              !document || !validClipProject({ ...draft, compositionInputs: inputs }, document)
            }
            onClick={() => change('compositionInputs', inputs)}
          >
            {t('composition.applyTemplate')}
          </Button>
        </section>
      )}
      {document && (
        <ClipCompositionInputFields
          document={document}
          presets={{
            intro: draft.introPreset || CLIP_DEFAULT_REGION_PRESETS.intro,
            outro: draft.outroPreset || CLIP_DEFAULT_REGION_PRESETS.outro,
          }}
          ratio={draft.ratio}
          value={inputs}
          onChange={(inputs) => change('compositionInputs', inputs)}
        />
      )}
    </>
  )
  const instructionField = (
    <>
      {/* The owner's own instruction sits with the answers it accompanies
              and is optional; an empty one is indistinguishable from never
              having written one (CLIP-121). */}
      <div>
        <FieldLabel htmlFor="clip-instruction">
          {t('project.instruction')} {t('composition.optionalSuffix')}
        </FieldLabel>
        <Typography variant="body" className="text-content-secondary mt-2 mb-2 break-words">
          {t('project.instructionPrompt')}
        </Typography>
        <Textarea
          id="clip-instruction"
          inputMode="text"
          {...INPUT}
          autoGrow
          value={draft.instruction ?? ''}
          onChange={(event) =>
            change(
              'instruction',
              boundedFreeText(event.target.value, CLIP_PROJECT_LIMITS.instruction),
            )
          }
        />
        <FieldCount
          left={CLIP_PROJECT_LIMITS.instruction - freeTextLength(draft.instruction ?? '')}
        />
      </div>
    </>
  )
  const storyFields = (
    <fieldset disabled={locked} className="min-w-0 space-y-6">
      {titleField}
      {templateListbox}
      {answerFields}
      {instructionField}
    </fieldset>
  )
  const guide = draft.advertiserGuide ?? EMPTY_GUIDE
  const campaign = draft.campaign ?? false
  // 영상 종류 is two explicit choices saved as an option save (CLIP-210, THEME-52).
  const kindFields = (
    <fieldset disabled={locked} aria-label={t('project.campaign')} className="min-w-0 space-y-3">
      <ChoiceButton
        title={t('project.kindSponsored')}
        description={t('project.kindSponsoredHelp')}
        selected={campaign}
        onClick={() => change('campaign', true)}
      />
      <ChoiceButton
        title={t('project.kindSelfPaid')}
        description={t('project.kindSelfPaidHelp')}
        selected={!campaign}
        onClick={() => change('campaign', false)}
      />
    </fieldset>
  )
  // 요청 asks as the post asks: the guide expanded with its chips, a clip counting only keywords
  // and hashtags under 영상에 꼭 담을 내용 (CLIP-210, POST-118).
  const requirementFields = (
    <fieldset disabled={locked} className="min-w-0">
      <Typography variant="body" as="p">
        {t('project.requirementsHelp')}
      </Typography>
      <AdvertiserGuideFields
        value={guide}
        items={['keywords', 'hashtags']}
        expanded
        requirementsTitle="clip"
        disabled={disabled || readOnly}
        onChange={(next) => change('advertiserGuide', next)}
        afterText={
          <AdvertiserGuideSuggestions
            guide={guide}
            items={['keywords', 'hashtags']}
            readOnly={disabled || readOnly}
            onChange={(next) => change('advertiserGuide', next)}
          />
        }
      />
    </fieldset>
  )
  // 템플릿 is a choice screen with 없음 beside every template (CLIP-146, THEME-52).
  const templateFields = (
    <fieldset disabled={locked} className="min-w-0 space-y-3">
      <div role="group" aria-label={t('project.template')} className="space-y-3">
        <ChoiceButton
          title={t('project.templateNone')}
          description={t('project.templateNoneHelp')}
          selected={!draft.videoTemplateId}
          onClick={() => change('videoTemplateId', '')}
        />
        {templates.templates.map((template) => (
          <ChoiceButton
            key={template.id}
            title={template.name}
            selected={draft.videoTemplateId === template.id}
            onClick={() => change('videoTemplateId', template.id)}
          />
        ))}
      </div>
      {templates.isPending && <Typography variant="body">{t('project.loading')}</Typography>}
      {templates.isError && (
        <div role="alert">
          <AppFailureMessage failure={appFailureFromConnect(templates.error)} />
          <Button variant="ghost" onClick={() => void templates.refetch()}>
            {t('project.retry')}
          </Button>
        </div>
      )}
      {!selected && !!draft.videoTemplateId && !templates.isPending && (
        <FieldMessage>{t('project.detachedTemplate')}</FieldMessage>
      )}
      {/* The wizard's 저장 comes back to this screen (CLIP-225). */}
      <Link
        to="/video-templates/new"
        search={{ channel: draft.channel ?? channel, clip: stored?.id ?? 'new' }}
        className={buttonStyles({ variant: 'ghost' })}
      >
        {t('directory.create')}
      </Link>
    </fieldset>
  )
  // 내용: the optional title, then the selected template's answers (CLIP-225).
  const contentFields = (
    <fieldset disabled={locked} className="min-w-0 space-y-6">
      {titleField}
      {answerFields}
      {!document?.fields.length && (
        <Typography variant="body" className="text-content-secondary">
          {t('project.noAnswers')}
        </Typography>
      )}
    </fieldset>
  )
  const noteFields = (
    <fieldset disabled={locked} className="min-w-0 space-y-6">
      {instructionField}
    </fieldset>
  )
  const answerTexts = document
    ? [
        ...Object.values(inputs.values),
        ...Object.values(inputs.items).flatMap((items) =>
          items.flatMap((item) => Object.values(item.values)),
        ),
      ].filter((text): text is string => typeof text === 'string')
    : []
  const keywordsHeld = clipKeywordCoverage(
    draft.title,
    draft.instruction ?? '',
    answerTexts,
    guide.keywords,
  )
  const sourceFields = (
    <fieldset disabled={locked} className="min-w-0 space-y-6">
      {stored && (
        <ClipLinkedPost
          key={`${ownerId}:${stored.id}`}
          ownerId={ownerId}
          project={stored}
          disabled={disabled}
          readOnly={readOnly}
        />
      )}
      {stored && (
        <MoveClipChannel
          ownerId={ownerId}
          project={stored}
          disabled={disabled}
          readOnly={readOnly}
        />
      )}
    </fieldset>
  )
  const styleFields = (
    <>
      <fieldset disabled={locked} className="min-w-0 space-y-6">
        <div>
          <Typography variant="fieldTitle" as="p">
            {t('project.ratio')}
          </Typography>
          <Typography variant="body" className="mt-2">
            {t(`ratio.${stored?.ratio ?? 'vertical'}`)}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-2">
            {t(stored ? 'project.fixedRatio' : 'project.verticalOnly')}
          </Typography>
        </div>
        <div>
          <FieldLabel htmlFor="clip-duration">{t('project.duration')}</FieldLabel>
          <TextField
            id="clip-duration"
            type="number"
            inputMode="decimal"
            {...INPUT}
            min={CLIP_PROJECT_LIMITS.minSeconds}
            max={CLIP_PROJECT_LIMITS.maxSeconds}
            step="any"
            value={seconds}
            onChange={(event) => {
              setSeconds(event.target.value)
              change('targetDurationMs', Math.round(Number(event.target.value) * 1000))
            }}
            aria-invalid={
              !Number.isFinite(draft.targetDurationMs) ||
              draft.targetDurationMs < CLIP_PROJECT_LIMITS.minSeconds * 1000 ||
              draft.targetDurationMs > CLIP_PROJECT_LIMITS.maxSeconds * 1000
            }
          />
          <Typography variant="body" className="text-content-secondary mt-2">
            {t('project.durationHelp')}
          </Typography>
        </div>
      </fieldset>

      {stored && !stored.composition && stored.disclosure && (
        <section className="space-y-3">
          <Typography variant="body">
            {t('composition.retainedDisclosure', {
              text: t(`disclosure.${stored.disclosure}`),
            })}
          </Typography>
          <label className="flex min-h-11 items-center gap-3">
            <Checkbox
              disabled={locked}
              checked={!draft.hideDisclosure}
              onChange={(e) => change('hideDisclosure', !e.target.checked)}
            />
            {t('project.showDisclosure')}
          </label>
        </section>
      )}

      {/* Appearance choices stay after duration; phones open them explicitly.
              The project draft and other inputs remain mounted above this view. */}
      {stored && (
        <Disclosure
          title={t('project.designChoices')}
          size="row"
          className="mt-6"
          open={wide ? designExpanded.desktop : designExpanded.phone}
          onOpenChange={(open) =>
            setDesignExpanded((current) => ({
              ...current,
              [wide ? 'desktop' : 'phone']: open,
            }))
          }
          lead={
            <Typography variant="body" as="p" className="text-content-secondary break-words">
              {t('project.designSummary', {
                intro:
                  regions && !regions.enabled.intro
                    ? t('project.regionOff')
                    : t(
                        `composition.design.intro_${draft.introPreset || CLIP_DEFAULT_REGION_PRESETS.intro}`,
                      ),
                outro:
                  regions && !regions.enabled.outro
                    ? t('project.regionOff')
                    : t(
                        `composition.design.outro_${draft.outroPreset || CLIP_DEFAULT_REGION_PRESETS.outro}`,
                      ),
                pace: t(`pace.${draft.captionPace || 'steady'}`),
                accent: t(`accent.${draft.accent || 'none'}`),
                count: draft.allowedCaptionStyles?.length || 1,
              })}
            </Typography>
          }
        >
          <fieldset disabled={locked} className="space-y-3">
            <Typography variant="fieldTitle" as="p">
              {t('pace.label')}
            </Typography>
            <SegmentedControl
              ariaLabel={t('pace.label')}
              value={draft.captionPace || 'steady'}
              options={[
                { value: 'steady' as const, label: t('pace.steady') },
                { value: 'rapid' as const, label: t('pace.rapid') },
              ]}
              onChange={(captionPace) => change('captionPace', captionPace)}
            />
            <Typography variant="body" className="text-content-secondary">
              {t('pace.help')}
            </Typography>
            <Typography variant="fieldTitle" as="p">
              {t('project.accent')}
            </Typography>
            <div
              role="radiogroup"
              aria-label={t('project.accent')}
              className="flex flex-wrap gap-2"
            >
              {CLIP_ACCENTS.map((value) => (
                <Button
                  key={value || 'none'}
                  variant={(draft.accent ?? '') === value ? 'secondary' : 'ghost'}
                  role="radio"
                  aria-checked={(draft.accent ?? '') === value}
                  onClick={() => change('accent', value)}
                >
                  <span className="flex items-center gap-2">
                    {value && (
                      <span
                        aria-hidden="true"
                        className="size-4 rounded-full"
                        style={{ backgroundColor: CLIP_DESIGN.accent[value] }}
                      />
                    )}
                    {t(`accent.${value || 'none'}`)}
                  </span>
                </Button>
              ))}
            </div>
            <Typography variant="body" className="text-content-secondary">
              {t('project.accentHelp')}
            </Typography>

            <ClipDesignSelection
              projectId={stored.id}
              draft={draft}
              onChange={patch}
              regions={regions}
            />
          </fieldset>
        </Disclosure>
      )}

      {stored &&
        dubbing?.(draft.dubbing ?? { enabled: false, voiceId: '' }, (value) =>
          change('dubbing', value),
        )}
    </>
  )
  const notices = (
    <>
      {refusal}
      {failure && (
        <div role="alert" className="mb-3">
          <AppFailureMessage failure={appFailureFromConnect(failure)} />
        </div>
      )}
    </>
  )
  const generationActions = actions?.(valid && voiceReady && !disabled && !templateChanged, dirty)
  const summary = clipPreparationRows({
    t,
    draft,
    document,
    inputs,
    selected,
    templates,
    stored,
    regions,
    voiceLibrary,
  })
  return (
    <>
      <form
        id="clip-project-form"
        className={guided ? 'mt-2 flex flex-1 flex-col' : 'mt-6'}
        onSubmit={(event) => event.preventDefault()}
      >
        {guided && layout ? (
          layout({
            screens: {
              kind: kindFields,
              requirements: requirementFields,
              sources: sourceFields,
              template: templateFields,
              content: contentFields,
              style: styleFields,
              note: noteFields,
            },
            rows: summary,
            actions: generationActions,
            refusal: notices,
            campaign,
            requirements: campaign
              ? { coverage: { keywords: keywordsHeld }, hashtags: guide.hashtags }
              : undefined,
            noteEmpty: !(draft.instruction ?? '').trim(),
            mint,
            inputRefusal: !voiceReady
              ? 'style'
              : !valid || templateChanged
                ? !Number.isFinite(draft.targetDurationMs) ||
                  draft.targetDurationMs < CLIP_PROJECT_LIMITS.minSeconds * 1000 ||
                  draft.targetDurationMs > CLIP_PROJECT_LIMITS.maxSeconds * 1000
                  ? 'style'
                  : 'content'
                : undefined,
          })
        ) : (
          // New and finalized read-only views keep the one spaced column; a new clip has no
          // source fields yet.
          <div className={cardStyles({ className: 'min-w-0 space-y-6' })}>
            {storyFields}
            {kindFields}
            {campaign && requirementFields}
            {sourceFields}
            {styleFields}
          </div>
        )}
      </form>
      {children}
      {!readOnly && !guided && (
        <ActionBar className="mt-auto">
          {notices}
          <div className="flex flex-wrap justify-end gap-3">{generationActions}</div>
        </ActionBar>
      )}
    </>
  )
}
