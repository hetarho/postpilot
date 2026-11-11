import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import {
  isPublished,
  postReturnDestination,
  rememberPostEntry,
  readPostReturnContext,
  retainMintedPostEntry,
  markPostHistoryReturn,
  type PostDraft,
  type PostTemplateAnswer,
} from '@/entities/post'
import { useSession } from '@/entities/session'
import { noTemplateLabel, useTemplates } from '@/entities/template'
import type { Voice } from '@/entities/voice'
import {
  discardContentQueue,
  flushContentQueue,
  useCaretHandoff,
} from '@/features/edit-post-content'
import { DeletePostButton } from '@/features/delete-post'
import { InspectWritingRequestAction } from '@/features/inspect-writing-request'
import { discardDraftQueue, useAutosave } from '@/features/save-draft'
import { useBriefMirror, type GenerationMode } from '@/features/generate-post'
import {
  TemplateAnswerFields,
  answerFields,
  firstEnabledAnswer,
  missingRequiredAnswers,
  toAnswerPatch,
  withAnswer,
} from '@/features/fill-template-answers'
import {
  SegmentedControl,
  buttonStyles,
  ChoiceButton,
  type PopoverHandle,
  pageStyles,
  Typography,
} from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { editorNavigationCopy } from '../config/i18n'
import { initialGuideStep, stashGuideStep, takeGuideStep, type GuideStep } from '../model/guide'
import { editorSteps } from '../model/steps'
import { useDraftAssignments } from '../model/useDraftAssignments'
import { useDraftSteps } from '../model/useDraftSteps'
import { useEditorJob } from '../model/useEditorJob'
import { EditorPhotos } from './EditorPhotos'
import { EditorProgressBar, EditorStatusLine } from './EditorStatus'
import { useEditorChoices } from './EditorDockHeader'
import { EditorGuide } from './EditorGuide'
import { EditorReview } from './EditorReview'
import { EditorVoiceWarning } from './EditorVoiceWarning'
import { LifecycleSteps } from './LifecycleSteps'
import { MemoField } from './MemoField'
import { TitleField } from './TitleField'
import { CampaignMaterial } from './CampaignMaterial'
import { CampaignTracker } from './CampaignTracker'

const STEP_PANEL_ID = 'editor-step-panel'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
  /** The account's 기본, which a draft with no post yet starts in — resolved by the route before
   *  this mounts, so the picker is seeded once with the right answer; undefined when there is
   *  none, which starts the draft on 말투 없음 (POST-101). Ignored for an existing post, whose
   *  voice is its own. */
  defaultVoice?: Pick<Voice, 'id' | 'made'>
  /** A template the wizard opened from ①'s 템플릿 picker just saved (TMPL-32): assigned once
   *  through the ordinary assignment, then dropped from the address. */
  handedTemplate?: string
}

/** Title + memo, autosaved, plus the post's lifecycle presented as three steps. The screen the
 *  whole input side of the product hangs off (PRD F-2).
 *
 *  The steps are PANELS, not routes: one mounted editor per slug, so a step change cannot remount
 *  the component and strand a queued save (tech/draft-autosave). Title, memo, photos and the mint
 *  plumbing therefore stay outside the panels — they are the post's identity, not one step's work. */
export function DraftEditor({ post, defaultVoice, handedTemplate }: DraftEditorProps) {
  const { t, i18n } = useTranslation('posts')
  const navigate = useNavigate()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const copy = editorNavigationCopy[i18n.resolvedLanguage?.startsWith('ko') ? 'ko' : 'en']
  const returnTo = useMemo(() => postReturnDestination(ownerId, post?.slug), [ownerId, post?.slug])
  useLayoutEffect(() => {
    if (!post && !readPostReturnContext(ownerId))
      rememberPostEntry(ownerId, {
        path: '/naver-blog',
        section: 'naverBlog',
        filters: {},
        scrollY: 0,
        targetId: 'new',
      })
  }, [ownerId, post])
  // Both fields are textareas: a Korean title fits ~14 characters across a 360px screen at the
  // display size, and a single-line input would scroll the rest of it out of a field that has no
  // well to show it scrolled (THEME-8 — the title is one of the largest things on the
  // screen, so it wraps instead).
  const titleRef = useRef<HTMLTextAreaElement>(null)
  const memoRef = useRef<HTMLTextAreaElement>(null)
  const firstAnswerRef = useRef<HTMLTextAreaElement>(null)
  const caretFields = useMemo(() => ({ title: titleRef, memo: memoRef }), [])
  const caret = useCaretHandoff(post?.slug, caretFields)

  const assignments = useDraftAssignments(post, defaultVoice, post ? '' : handedTemplate)
  const { voiceId, templateId, targetLanguage } = assignments

  // Which writing screen is up. It crosses the mint's remount through a stash,
  // the way the caret does, so the first saved input never throws the owner elsewhere.
  const [guide, setGuide] = useState<GuideStep>(
    () => takeGuideStep(post?.slug) ?? initialGuideStep(post),
  )
  const guideRef = useRef(guide)
  useEffect(() => {
    guideRef.current = guide
  }, [guide])

  // ①'s text and its lock are the autosave's (POST-86); the page only lays the SELECTED template's
  // fields over it (POST-62).
  const { templates } = useTemplates(ownerId)
  const selectedTemplate = templates.find((candidate) => candidate.id === templateId)
  const answerPatch = useCallback(
    (answers: readonly PostTemplateAnswer[]) =>
      toAnswerPatch(answerFields(selectedTemplate, answers)),
    [selectedTemplate],
  )
  const autosave = useAutosave({
    post,
    answerPatch,
    voiceId,
    templateId,
    targetLanguage,
    onMinted: (slug) => {
      caret.stash(slug)
      stashGuideStep(slug, guideRef.current)
      retainMintedPostEntry(ownerId, slug)
      // Replace the unminted address without changing its creation/history context. The scroll
      // stays: the mint is an edit’s side effect, and a router reset would move the input
      // away from the owner.
      void navigate({ to: '/posts/$slug', params: { slug }, replace: true, resetScroll: false })
    },
  })
  // A template the wizard handed back is assigned once, through the same assignment the picker
  // uses, and then dropped from the address so a reload or Back cannot assign it again (TMPL-32).
  // An unsaved draft already started on it (useDraftAssignments).
  const handled = useRef(false)
  const { assignTemplate } = autosave
  const postSlug = post?.slug
  const postTemplateId = post?.template.id
  useEffect(() => {
    if (!handedTemplate || handled.current) return
    handled.current = true
    if (postSlug) {
      if (postTemplateId !== handedTemplate)
        void assignTemplate(handedTemplate).catch(() => undefined)
      void navigate({
        to: '/posts/$slug',
        params: { slug: postSlug },
        search: {},
        replace: true,
        resetScroll: false,
      })
    } else void navigate({ to: '/posts/new', search: {}, replace: true, resetScroll: false })
  }, [handedTemplate, postSlug, postTemplateId, assignTemplate, navigate])
  // The patch carries every field the template declares, so reading it back gives the same fields.
  const fields = useMemo(
    () => answerFields(selectedTemplate, autosave.answers),
    [selectedTemplate, autosave.answers],
  )
  const published = post ? isPublished(post) : false

  // The durable job is watched HERE, not inside the step panels: the page-top status region
  // reports it and the panels act on it, and one poll has to serve both ([I5]).
  const jobView = useEditorJob(post)
  // 확인's writing card has a bar of its own; while it is in view the page-top one fades out.
  const [writingCardShown, setWritingCardShown] = useState(false)

  // The step lives here, above the fields, because the bar that switches it is the first thing
  // on the screen — the post's lifecycle is what you navigate before you read anything else.
  const { step, select: setStep } = useDraftSteps(post?.status ?? '', Boolean(post?.storyline))
  const previousStep = useRef(step)
  useLayoutEffect(() => {
    if (previousStep.current !== 'finish' && step === 'finish') window.scrollTo(0, 0)
    previousStep.current = step
  }, [step])
  const [showRequiredErrors, setShowRequiredErrors] = useState(false)
  const checkRequiredAnswers = () => {
    if (missingRequiredAnswers(fields).length === 0) return true
    setShowRequiredErrors(true)
    setStep('generate')
    setGuide('details')
    // The field lives on ①'s 내용 screen, which may have to mount again first.
    window.setTimeout(() => {
      document.querySelector<HTMLTextAreaElement>('[data-required-answer-missing="true"]')?.focus()
    }, 0)
    return false
  }

  // The brief widget SETS the target length and the tag count while the generate action SENDS
  // them from another layer, so the screen they both hang off owns the values.
  const brief = useBriefMirror(post)
  const inspectionContext = JSON.stringify([
    autosave.title,
    autosave.memo,
    autosave.answers,
    autosave.storyline,
    voiceId,
    templateId,
    targetLanguage,
    brief.targetLength,
    brief.tagCount,
  ])
  const inspectionSourcePending = Boolean(
    post &&
    (autosave.title !== post.title ||
      autosave.memo !== post.memo ||
      JSON.stringify(toAnswerPatch(fields)) !==
        JSON.stringify(toAnswerPatch(answerFields(selectedTemplate, post.templateAnswers))) ||
      JSON.stringify(autosave.storyline) !== JSON.stringify(post.storyline?.paragraphs) ||
      voiceId !== (post.voice?.id ?? '') ||
      templateId !== post.template.id ||
      targetLanguage !== post.targetLanguage ||
      brief.targetLength !== post.targetLength ||
      brief.tagCount !== post.tagCount),
  )
  const briefRef = useRef<PopoverHandle>(null)
  // The run a press of 생성 or A/B 비교 was refused for, while the brief that press opened is up:
  // the brief marks what that run is missing. The count restarts the marked fields' shake.
  const [briefRefusal, setBriefRefusal] = useState<{ mode: GenerationMode; count: number }>()
  // The brief lives on 확인. A press refused anywhere else first brings that screen up, then
  // opens the brief once it has mounted.
  const [briefPending, setBriefPending] = useState(false)
  const openBriefFor = (mode: GenerationMode) => {
    setBriefRefusal((previous) => ({ mode, count: (previous?.count ?? 0) + 1 }))
    if (step === 'generate' && guide === 'review' && briefRef.current) briefRef.current.open()
    else {
      setStep('generate')
      setGuide('review')
      setBriefPending(true)
    }
  }
  useEffect(() => {
    if (!briefPending || !briefRef.current) return
    setBriefPending(false)
    briefRef.current.open()
  }, [briefPending, guide, step])

  const titleField = (
    <TitleField
      value={autosave.title}
      onChange={autosave.setTitle}
      fieldRef={titleRef}
      nextRef={firstEnabledAnswer(fields) ? firstAnswerRef : titleRef}
      readOnly={published}
    />
  )
  const memoField = (
    <MemoField
      value={autosave.memo}
      onChange={autosave.setMemo}
      fieldRef={memoRef}
      readOnly={published}
    />
  )
  const answerFieldsPanel = (
    <TemplateAnswerFields
      fields={fields}
      showRequiredErrors={showRequiredErrors}
      firstFieldRef={firstAnswerRef}
      disabled={published}
      onChange={(label, change) =>
        autosave.setAnswers(toAnswerPatch(withAnswer(fields, label, change)))
      }
    />
  )
  const choices = useEditorChoices({
    briefRef,
    post,
    ownerId,
    voiceId,
    templateId,
    targetLanguage,
    targetLength: brief.targetLength,
    tagCount: brief.tagCount,
    onVoiceSelect: post ? autosave.reassign : assignments.setVoiceId,
    onGrowVoice: (voiceId) => {
      void autosave
        .flush()
        .then(async () => {
          if (post) await flushContentQueue(post.slug)
          await navigate({
            to: '/voices/$voiceId/materials',
            params: { voiceId },
            search: { learn: 'chat' },
          })
        })
        .catch(() => {
          /* The draft/content save status retains and explains a refusal. */
        })
    },
    onTemplateSelect: post ? autosave.assignTemplate : assignments.setTemplateId,
    onTemplateCreate: () => {
      void autosave
        .flush()
        .then(async () => {
          const slug = await autosave.ensureSlug()
          // Back from the wizard, or saved there, the owner returns to the screen they left —
          // the template question, now with the new template to pick — not to the summary a
          // returning draft otherwise opens on.
          stashGuideStep(slug, 'template')
          await navigate({ to: '/templates/new', search: { post: slug } })
        })
        .catch(() => {
          /* The autosave status explains a save refusal. */
        })
    },
    onTargetLanguageSelect: post ? autosave.assignTargetLanguage : assignments.setTargetLanguage,
    onBriefSaved: (set) => {
      brief.setTargetLength(set.targetLength)
      brief.setTagCount(set.tagCount)
    },
    refusal: briefRefusal,
    onBriefClosed: () => setBriefRefusal(undefined),
    writingTest: {
      href: post ? `/tests?sourcePost=${encodeURIComponent(post.slug)}` : '/tests',
      onOpen: async () => {
        await autosave.flush()
        const slug = await autosave.ensureSlug()
        await flushContentQueue(slug)
        await navigate({ href: `/tests?sourcePost=${encodeURIComponent(slug)}` })
      },
    },
  })

  // A way out of the draft that keeps every typed word: the autosave and the content queue are
  // flushed before the router leaves (the same path the grow-voice link takes).
  const leaveTo = (href: string) => {
    void autosave
      .flush()
      .then(async () => {
        if (post) await flushContentQueue(post.slug)
        await navigate({ href })
      })
      .catch(() => {
        /* The draft/content save status retains and explains a refusal. */
      })
  }

  // Each writing screen holds one task; the summary links back to every input.
  const guideScreens: Record<GuideStep, ReactNode> = {
    kind: (
      <div role="group" aria-label={t('guide.questions.kind')} className="mt-6 space-y-3">
        <ChoiceButton
          title={t('guide.sponsored')}
          description={t('guide.sponsoredHelp')}
          selected={autosave.campaign.on}
          disabled={published}
          onClick={() => autosave.setCampaign({ ...autosave.campaign, on: true })}
        />
        <ChoiceButton
          title={t('guide.selfPaid')}
          description={t('guide.selfPaidHelp')}
          selected={!autosave.campaign.on}
          disabled={published}
          onClick={() => autosave.setCampaign({ ...autosave.campaign, on: false })}
        />
      </div>
    ),
    requirements: (
      <CampaignMaterial
        guide={autosave.campaign.guide}
        refusal={autosave.campaignRefusal}
        readOnly={published}
        onChange={(guide) => autosave.setCampaign({ ...autosave.campaign, guide })}
      />
    ),
    story: (
      <div className="mt-6">
        <Typography variant="body" as="p" className="mb-4">
          {t('guide.memoHelp')}
        </Typography>
        {memoField}
      </div>
    ),
    template: (
      <div className="mt-6">
        <Typography variant="body" as="p" className="mb-4">
          {t('guide.templateHelp')}
        </Typography>
        {choices.template}
      </div>
    ),
    photos: (
      <div className="mt-4">
        <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />
        {post && post.images.length > 0 && (
          <ContactSheet
            images={post.images}
            videos={post.videos}
            observations={post.observations}
            activeJob={jobView.job}
          />
        )}
      </div>
    ),
    details: (
      <div className="mt-2 flex flex-col">
        {titleField}
        <Typography variant="meta" as="p" className="mt-2">
          {t('guide.titleHelp')}
        </Typography>
        {fields.length > 0 ? (
          <>
            <Typography variant="label" as="p" className="mt-5">
              {selectedTemplate?.name}
            </Typography>
            {answerFieldsPanel}
          </>
        ) : (
          <Typography variant="body" as="p" className="mt-6">
            {t(templateId ? 'guide.noFields' : 'guide.withoutTemplateFields')}
          </Typography>
        )}
      </div>
    ),
    review: (
      <EditorReview
        post={post}
        ownerId={ownerId}
        sponsored={autosave.campaign.on}
        memo={autosave.memo}
        title={autosave.title}
        templateName={selectedTemplate?.name || post?.template.name || noTemplateLabel()}
        voice={choices.voice}
        voiceWarning={
          post ? <EditorVoiceWarning ownerId={ownerId} voice={post.voice} /> : undefined
        }
        brief={choices.brief}
        job={jobView.job}
        onWritingCardShown={setWritingCardShown}
        onGo={setGuide}
        onOpenGuidelines={() => leaveTo('/guidelines')}
      />
    ),
  }
  const renderGuide = (slots?: { actions: (back: ReactNode) => ReactNode; notice: ReactNode }) => (
    <EditorGuide
      step={guide}
      sponsored={autosave.campaign.on}
      closingHasText={Boolean(autosave.memo.trim())}
      tracker={
        autosave.campaign.on && guide !== 'kind' && guide !== 'requirements' ? (
          <CampaignTracker
            guide={autosave.campaign.guide}
            photoCount={post?.images.length ?? 0}
            title={autosave.title}
            memo={autosave.memo}
            answers={fields}
            step={guide}
            onEdit={() => setGuide('requirements')}
          />
        ) : undefined
      }
      onStepChange={setGuide}
      screens={guideScreens}
      actions={slots?.actions}
      notice={guide === 'review' ? slots?.notice : undefined}
    />
  )

  return (
    // Guide: the whole flow sits in one comfortable centred column, phone and desk alike, and the
    // guided bar meets the screen's bottom edge (no page padding under it).
    <main
      className={pageStyles({
        width: 'prose',
        className: 'flex flex-1 flex-col pt-3 pb-0 sm:pt-6 sm:pb-0',
      })}
    >
      {/* First child of the flow on template: a sticky box can only be pinned by the box it sits
          in, and this one has to hold the page's top edge while a draft thousands of pixels tall
          scrolls past it. It adds no layout height. */}
      <EditorProgressBar job={jobView.job} muted={writingCardShown} />
      {/* The 가제 is an editable field on the 내용 screen; this mirrors its value into the document
          outline on every screen, without a second field or tab stop. */}
      <Typography variant="display" className="sr-only">
        {autosave.title.trim() || t('editor.titlePlaceholder')}
      </Typography>
      {/* ONE row holding the way out, the lifecycle stations and the delete, the way the clip
          workspace's top row does (CLIP-37). `flex-wrap` so the delete refusal, which asks for the
          full width, drops to its own line rather than crushing the way out beside it. */}
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        {/* Guide: a round 44px back button — the glyph is the control, the destination its name. */}
        <a
          href={returnTo.href}
          aria-label={copy.home}
          onClick={(event) => {
            if (
              event.button !== 0 ||
              event.metaKey ||
              event.ctrlKey ||
              event.shiftKey ||
              event.altKey
            )
              return
            event.preventDefault()
            void autosave
              .flush()
              .then(async () => {
                if (post) markPostHistoryReturn(ownerId, post.slug)
                await navigate({ href: returnTo.href })
                window.requestAnimationFrame(() => window.scrollTo(0, returnTo.scrollY))
              })
              .catch(() => {
                /* The autosave's status region exposes the refusal. */
              })
          }}
          className={buttonStyles({ variant: 'secondary', size: 'icon', className: 'shadow-sm' })}
        >
          <ArrowLeft aria-hidden="true" className="size-5" />
          <span className="sr-only">{copy.home}</span>
        </a>

        {/* A post with a lifecycle navigates it first: three stations, the one the post is in an
            inverted pill (THEME-39). `/posts/new` has none and names the place instead. */}
        {post ? (
          <SegmentedControl
            value={step}
            options={editorSteps()}
            onChange={setStep}
            ariaLabel={t('editor.stepAria')}
            controls={STEP_PANEL_ID}
            variant="steps"
            className="min-w-0 flex-1"
          />
        ) : (
          <Typography variant="label" as="span" className="text-content-primary">
            {copy.location}
          </Typography>
        )}

        {post && (
          <div className="ml-auto flex shrink-0 items-center gap-2">
            {/* A queue outlives its editor, so a retry left running would keep saving a slug the
                server no longer has and report that failure for a post the user destroyed on
                template (POST-12). Discarded before the navigation unmounts the
                editor, and only for this slug. */}
            <DeletePostButton
              post={post}
              returnTo={returnTo}
              onDeleted={() => {
                discardDraftQueue(post.slug)
                discardContentQueue(post.slug)
                markPostHistoryReturn(ownerId, post.slug)
              }}
            />
          </div>
        )}

        {/* The editor's ONE state indicator. It replaced the status badge that stood here: the
            row may not carry two of them (POST-45). Mounted for `/posts/new` as well, which
            has no status of its own but does have an autosave that can fail — and no save
            button anywhere to fall back on (PRD F-2). It takes the row's last line whole, so a
            message never competes with the step names for width. */}
        <EditorStatusLine
          job={jobView.job}
          saveState={autosave.state}
          status={post?.status ?? ''}
          photoCount={post?.images.length ?? 0}
          videoCount={post?.videos.length ?? 0}
          className="order-last w-full px-1 sm:text-right"
        />
      </div>

      {post ? (
        <LifecycleSteps
          post={post}
          ownerId={ownerId}
          step={step}
          onStepChange={setStep}
          renderGuide={renderGuide}
          onOpenBrief={openBriefFor}
          targetLength={brief.targetLength}
          onTitleFinalized={autosave.setTitle}
          beforeStart={autosave.flush}
          clipMaterial={{ title: autosave.title, memo: autosave.memo }}
          checkRequiredAnswers={checkRequiredAnswers}
          jobView={jobView}
          storyline={{
            paragraphs: autosave.storyline,
            onChange: autosave.setStoryline,
            saveState: autosave.state,
          }}
          campaign={{
            on: autosave.campaign.on,
            guide: autosave.campaign.guide,
            ticks: autosave.campaign.ticks,
            readOnly: published,
            onTicksChange: (ticks) => autosave.setCampaign({ ...autosave.campaign, ticks }),
          }}
          inspection={{ contextKey: inspectionContext, sourcePending: inspectionSourcePending }}
        />
      ) : (
        <>
          {/* The saved post's entry stands here from the start, so the mint does not push ① down
              by a button's height. There is no request to read until the draft is saved. */}
          {ownerId && (
            <InspectWritingRequestAction
              target={null}
              blockedReason={t('unsavedSource', { ns: 'requestInspection' })}
              className="my-2 self-end"
            />
          )}
          {/* No lifecycle yet: the same guided screens, with no 생성 until the first save has
              made a post to start a run on. */}
          <div className="flex flex-1 flex-col">{renderGuide()}</div>
        </>
      )}
      {choices.sheet}
    </main>
  )
}
