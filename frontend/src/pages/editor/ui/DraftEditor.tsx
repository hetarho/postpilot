import { useCallback, useEffect, useRef, useState, type ReactNode, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from '@tanstack/react-router'
import { FailureNotice, isTerminal } from '@/entities/generation-job'
import { hasContent, useRefreshPostImages, type PostDraft } from '@/entities/post'
import { useSession } from '@/entities/session'
import {
  useVoiceProfile,
  voiceContentLanguageMismatch,
  voiceContentLanguageMismatchReason,
  type VoiceRef,
} from '@/entities/voice'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
import { GenerationActions, type GenerationActionsHandle } from '@/features/generate-post'
import {
  BlockEditor,
  discardContentQueue,
  flushContentQueue,
  type BlockEditorHandle,
} from '@/features/edit-post-content'
import { DeletePostButton } from '@/features/delete-post'
import { VoiceLearningPanel, useVoiceLearning } from '@/features/finalize-post'
import { SentenceFeedback } from '@/features/give-voice-feedback'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { discardDraftQueue, peekPendingDraft, useAutosave } from '@/features/save-draft'
import { PostTemplateSelect } from '@/features/select-post-template'
import { PostVoiceSelect, reassignmentBlocker } from '@/features/select-post-voice'
import {
  ActionBar,
  Button,
  FieldLabel,
  Notice,
  SegmentedControl,
  Textarea,
  Typography,
  typographyStyles,
  type PopoverHandle,
  pageStyles,
} from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { ExportPanel } from '@/widgets/export-panel'
import { GenerationBrief } from '@/widgets/generation-brief'
import { PublishPanel } from '@/widgets/publish-panel'
import { RefineDock } from '@/widgets/refine-dock'
import { DeletedVoiceWarning, VoiceWarning } from '@/widgets/voice-warning'
import { clearCaret, peekCaret, stashCaret } from '../model/editor-handoff'
import { activeLocale } from '@/shared/lib'
import { editorStepLabel, editorSteps, stepForStatus, type EditorStep } from '../model/steps'
import { useEditorJob, type EditorJobView } from '../model/useEditorJob'
import { EditorPhotos } from './EditorPhotos'
import { EditorProgressBar, EditorStatusLine } from './EditorStatus'

const STEP_PANEL_ID = 'editor-step-panel'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
  /** The voice a draft with no post yet starts in — the account's default, resolved by the
   *  route before this mounts, so the first save always carries a concrete id
   *  (spec/policy/posts.md). Ignored for an existing post, whose voice is its own. */
  defaultVoiceId?: string
}

/** Title + memo, autosaved, plus the post's lifecycle presented as three steps. The screen the
 *  whole input side of the product hangs off (PRD F-2).
 *
 *  The steps are PANELS, not routes: one mounted editor per slug, so a step change cannot remount
 *  the component and strand a queued save (tech/draft-autosave). Title, memo, photos and the mint
 *  plumbing therefore stay outside the panels — they are the post's identity, not one step's work. */
export function DraftEditor({ post, defaultVoiceId = '' }: DraftEditorProps) {
  const { t } = useTranslation('posts')
  const navigate = useNavigate()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  // Both fields are textareas: a Korean title fits ~14 characters across a 360px screen at the
  // display size, and a single-line input would scroll the rest of it out of a field that has no
  // well to show it scrolled (design-language §0 — the title is one of the largest things on the
  // screen, so it wraps instead).
  const titleRef = useRef<HTMLTextAreaElement>(null)
  const memoRef = useRef<HTMLTextAreaElement>(null)

  // Text still queued for this post outranks what the server reported: it is what the
  // previous editor was in the middle of saving when the mint moved the URL, so it is
  // newer by exactly the characters typed during that round trip.
  const opening = post
    ? (peekPendingDraft(post.slug) ?? { title: post.title, memo: post.memo })
    : { title: '', memo: '' }
  const [title, setTitle] = useState(opening.title)
  const [memo, setMemo] = useState(opening.memo)

  // Read, not consumed — a component body may run more than once per mount.
  const caret = post ? peekCaret(post.slug) : undefined

  // A draft with no post yet holds its own choice; an existing post's voice is the server's,
  // and changes only through a confirmed reassignment (see `useAutosave.reassign`).
  const [newVoiceId, setNewVoiceId] = useState(defaultVoiceId)
  const voiceId = post ? post.voice.id : newVoiceId
  const voice: VoiceRef = post?.voice ?? {
    id: newVoiceId,
    name: '',
    deleted: false,
    sourceLanguage: undefined,
  }

  // The same shape for the 템플릿, with 없음 ('') as the default the server never overrides.
  const [newTemplateId, setNewTemplateId] = useState('')
  const templateId = post ? post.template.id : newTemplateId
  // Snapshot once when /posts/new opens. Later interface-locale switches are presentation-only;
  // the explicit selector is the sole way this draft's target changes.
  const [newTargetLanguage, setNewTargetLanguage] = useState<ContentLanguage>(() => activeLocale())
  const targetLanguage = post?.targetLanguage ?? newTargetLanguage

  const autosave = useAutosave({
    post,
    title,
    memo,
    voiceId,
    templateId,
    targetLanguage,
    onMinted: (slug) => {
      // Read off the live DOM, so this is the caret as it is now rather than as it was
      // when the save left.
      const focused = document.activeElement
      const field =
        focused === titleRef.current ? 'title' : focused === memoRef.current ? 'memo' : undefined
      if (field) {
        const element = field === 'title' ? titleRef.current : memoRef.current
        stashCaret({
          slug,
          field,
          selectionStart: element?.selectionStart ?? 0,
          selectionEnd: element?.selectionEnd ?? 0,
        })
      }
      // `replace`, so the back button goes to the list rather than to /posts/new — which
      // would open a second empty draft.
      void navigate({ to: '/posts/$slug', params: { slug }, replace: true })
    },
  })

  // The durable job is watched HERE, not inside the step panels: the page-top status region
  // reports it and the panels act on it, and one poll has to serve both ([I5]).
  const jobView = useEditorJob(post)

  // The step lives here, above the fields, because the bar that switches it is the first thing
  // on the screen — the post's lifecycle is what you navigate before you read anything else.
  const [step, setStep] = useState<EditorStep>(() => stepForStatus(post?.status ?? ''))
  const followedStatus = useRef(post?.status ?? '')
  useEffect(() => {
    const status = post?.status ?? ''
    if (followedStatus.current === status) return
    followedStatus.current = status
    // Also the "generation finished" handoff: the draft lands in 글 다듬기 and the user is taken
    // there once, rather than being scrolled inside 글 생성 and then moved again.
    setStep(stepForStatus(status))
  }, [post?.status])

  useEffect(() => {
    if (!caret) return
    clearCaret()
    const element = caret.field === 'title' ? titleRef.current : memoRef.current
    if (!element) return
    element.focus()
    element.setSelectionRange(caret.selectionStart, caret.selectionEnd)
  }, [caret])

  // Lifted out of `GenerationActions`: the brief widget SETS the target length and the generate
  // action SENDS it, and the two now live on different layers, so the value has to be owned by
  // the one screen they both hang off.
  // It is adjusted DURING RENDER rather than from an effect — an effect would paint one frame
  // carrying the value the server has already moved past.
  const [targetLength, setTargetLength] = useState(post?.targetLength)
  const briefRef = useRef<PopoverHandle>(null)
  const [serverTargetLength, setServerTargetLength] = useState(post?.targetLength)
  if (serverTargetLength !== post?.targetLength) {
    setServerTargetLength(post?.targetLength)
    setTargetLength(post?.targetLength)
  }

  const titleField = (
    <TitleField value={title} onChange={setTitle} fieldRef={titleRef} nextRef={memoRef} />
  )
  const memoField = <MemoField value={memo} onChange={setMemo} fieldRef={memoRef} />

  // Everything the next AI run is given, in one surface. Every callback goes through the autosave
  // queue for an existing post, and through local state for a draft the server has not created.
  //
  // The ref is how 글 생성's empty state offers a way IN: with no active 작성 모델 chosen there is
  // nothing to press in the bar, and this surface is the answer.
  const brief = (
    <GenerationBrief
      ref={briefRef}
      targetLanguage={targetLanguage}
      contentLanguage={post?.contentLanguage}
      frozenLanguage={
        post?.activeJob && !isTerminal(post.activeJob) ? post.activeJob.targetLanguage : undefined
      }
      onTargetLanguageSelect={post ? autosave.assignTargetLanguage : setNewTargetLanguage}
      photoCount={post?.images.length ?? 0}
      targetLength={
        post
          ? {
              slug: post.slug,
              value: targetLength,
              // The length is the one field in the brief a running job has already frozen; the
              // others are still worth changing for the NEXT run, which is why only this one greys.
              disabled: Boolean(post.activeJob && !isTerminal(post.activeJob)),
              onSaved: setTargetLength,
            }
          : undefined
      }
    />
  )

  // The 말투 and the 템플릿 ride the dock's own surface beside the brief's glyph, not inside it:
  // both are chosen per draft and both silently change what comes out of a run, so they are the
  // parts of the brief that must be readable — and changeable — without opening anything
  // (policy/posts.md). Neither shows a caption; each trigger reads its own value, and the labels
  // stay `sr-only` inside the two features.
  const voiceSelect = (
    <PostVoiceSelect
      ownerId={ownerId}
      value={voiceId}
      current={post?.voice}
      blocked={post ? reassignmentBlocker(post) : ''}
      confirm={Boolean(post)}
      onSelect={post ? autosave.reassign : setNewVoiceId}
      className="min-w-0 flex-1"
    />
  )

  const templateSelect = (
    <PostTemplateSelect
      ownerId={ownerId}
      value={templateId}
      current={post?.template}
      jobRunning={Boolean(post?.activeJob && !isTerminal(post.activeJob))}
      onSelect={post ? autosave.assignTemplate : setNewTemplateId}
      className="min-w-0 flex-1"
    />
  )

  // `items-start` so the glyph stays level with the two listboxes when either field grows a hint
  // or an error underneath it; `min-w-0` on both so a long voice or template name truncates inside
  // its own trigger instead of pushing the glyph off a 320px screen (§8.5). The two fields share
  // the row evenly, which is also what took the voice trigger down from full width.
  const dockHeader = (
    <div className="flex items-start gap-2">
      {voiceSelect}
      {templateSelect}
      {brief}
    </div>
  )

  // `flex-1 flex-col` here plus `mt-auto` on the dock is what puts the bar at the BOTTOM of a
  // short draft: `sticky` can only pull an element up toward the scrollport edge, never push one
  // down, so without it a new draft renders its dock mid-page with dead space beneath.
  return (
    <main className={pageStyles({ className: 'flex flex-1 flex-col' })}>
      {/* First child of the flow on template: a sticky box can only be pinned by the box it sits
          in, and this one has to hold the page's top edge while a draft thousands of pixels tall
          scrolls past it. It adds no layout height. */}
      <EditorProgressBar job={jobView.job} />
      {/* `flex-wrap` so the delete refusal, which asks for the full width, drops to its own line
          rather than crushing the way out beside it (§8.5). The Korean refusal copy is over 40
          characters, which is more than a 360px row can hold beside anything. */}
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        {/* Underlined: `link-fg` resolves to `content-secondary`, so at rest this was pixel-identical
            to ordinary copy and the only thing marking it as the way out was a `hover:` colour no
            touchscreen ever matches (§6). */}
        <Link
          to="/posts"
          className={typographyStyles({
            variant: 'label',
            className:
              'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center underline',
          })}
        >
          {t('editor.backToList')}
        </Link>
        <div className="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-2">
          {/* The editor's ONE state indicator. It replaced the status badge that stood here: the
              row may not carry two of them (change 15). Mounted for `/posts/new` as well, which
              has no status of its own but does have an autosave that can fail — and no save
              button anywhere to fall back on (PRD F-2). */}
          <EditorStatusLine
            job={jobView.job}
            saveState={autosave.state}
            status={post?.status ?? ''}
          />
          {post && (
            /* A queue outlives its editor, so a retry left running would keep saving a slug the
               server no longer has and report that failure for a post the user destroyed on
               template (tech/draft-autosave.md). Discarded before the navigation unmounts the
               editor, and only for this slug. */
            <DeletePostButton
              post={post}
              onDeleted={() => {
                discardDraftQueue(post.slug)
                discardContentQueue(post.slug)
              }}
            />
          )}
        </div>
      </div>

      {/* A post with a lifecycle navigates it first. `/posts/new` has none, so it shows no bar. */}
      {post && (
        <SegmentedControl
          value={step}
          options={editorSteps()}
          onChange={setStep}
          ariaLabel={t('editor.stepAria')}
          controls={STEP_PANEL_ID}
          className="mt-4"
        />
      )}

      {post ? (
        <LifecycleSteps
          post={post}
          ownerId={ownerId}
          step={step}
          onStepChange={setStep}
          titleField={titleField}
          memoField={memoField}
          dockHeader={dockHeader}
          onOpenBrief={() => briefRef.current?.open()}
          targetLength={targetLength}
          onTitleFinalized={setTitle}
          beforeStart={autosave.flush}
          ensureSlug={autosave.ensureSlug}
          jobView={jobView}
        />
      ) : (
        <>
          {/* No lifecycle yet, so no step bar — just the step ① surfaces that work without a post. */}
          {titleField}
          {memoField}
          <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />
          <EditorVoiceWarning ownerId={ownerId} voice={voice} />
          {/* A draft with no post yet has no committing action, but its 말투 and the rest of the
              brief still have to be reachable before the first word is typed. */}
          <EditorDock header={dockHeader} />
        </>
      )}
    </main>
  )
}

/** The finalized text 문장 의견 chooses a sentence from.
 *
 *  It mirrors the server's own body projection (`parseAuthoredContent`) block type for block
 *  type: TEXT/HEADING/QUOTE contribute their content and LIST contributes its items, while an
 *  image block has no sentence in it at all. A projection that omitted list items would make the
 *  control disappear on a list-only post and, worse, offer a sentence the server cannot find. */
function sentenceSource(content: PostContent): string {
  return content.blocks
    .flatMap((block) =>
      block.type === BlockType.LIST
        ? block.items
        : block.type === BlockType.IMAGE
          ? []
          : [block.content],
    )
    .filter((value) => value.trim() !== '')
    .join('\n')
}

/** The 가제 belongs to 글 생성 alone (policy/posts.md). From 글 다듬기 on there is exactly one
 *  title on the screen and it is `content.title`, edited through the block editor's header — two
 *  title fields side by side is a question the user cannot answer.
 *
 *  A textarea, not an input: a Korean title fits ~14 characters across a 360px screen at the
 *  display size, and a single-line input would scroll the rest of it out of a field that has no
 *  well to show it scrolled (§0 — the title is one of the largest things on the screen, so it
 *  wraps instead). Its value and its autosave stay in `DraftEditor`, so unmounting it on another
 *  step cannot strand a queued save. */
function TitleField({
  value,
  onChange,
  fieldRef,
  nextRef,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
  nextRef: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-title" className="sr-only">
        {t('editor.title')}
      </FieldLabel>
      {/* The visible title remains an editable field, so this mirrors its current value into the
          document outline without replacing the field or creating a second tab stop. */}
      <Typography variant="display" className="sr-only">
        {value.trim() || t('editor.titlePlaceholder')}
      </Typography>
      <Textarea
        id="post-title"
        ref={fieldRef}
        appearance="bare"
        rows={1}
        autoGrow
        value={value}
        // A pasted newline would otherwise be saved inside the title; the single-line input this
        // replaced dropped one for free.
        onChange={(event) => onChange(event.target.value.replace(/\n/g, ' '))}
        onKeyDown={(event) => {
          // The title is one line even though the field wraps, so Enter moves to the memo instead
          // of typing a newline into it — but never mid-composition, where Enter is how a Hangul
          // IME commits the syllable being written.
          if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
          event.preventDefault()
          nextRef.current?.focus()
        }}
        placeholder={t('editor.titlePlaceholder')}
        enterKeyHint="next"
        autoCapitalize="off"
        autoComplete="off"
        // The bare editor's caller owns the field's type (§7): the title wears the display role —
        // the post title is the largest thing on the screen (§0).
        className={typographyStyles({ variant: 'display', className: 'mt-4' })}
      />
    </>
  )
}

/** The memo is the post's own words, and it is what 글 생성 works from — so it is rendered by
 *  that step while its value and its autosave stay in `DraftEditor`. Unmounting the field on
 *  another step cannot strand a queued save: the queue is keyed by slug and lives above it. */
function MemoField({
  value,
  onChange,
  fieldRef,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-memo" className="sr-only">
        {t('editor.memo')}
      </FieldLabel>
      {/* `autoGrow` with a small `rows`: at 16 rows the memo was a 364px box scrolling inside
          itself, which swallowed every vertical swipe that landed on it and left the 16px gutters
          as the only place to scroll the page (§4.4). The well appearance owns the §3.1 input
          size, so the focused field remains at least 16px on a phone. */}
      <Textarea
        id="post-memo"
        ref={fieldRef}
        appearance="well"
        rows={6}
        autoGrow
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t('editor.memoPlaceholder')}
        enterKeyHint="enter"
        className="mt-5"
      />
    </>
  )
}

/** Below the memo, not above it: three wrapped lines of undismissable warning at the top of the
 *  editor pushed the writing field a fifth of a 640px screen down, for every user who has not
 *  trained a profile yet — and chrome is small, quiet and at the edges (§0). It still sits above
 *  글 생성, which is what it is a caveat about.
 *
 *  A deleted voice is the other caveat, and the louder one: nothing AI will run until it is
 *  restored or the post is moved. Its profile is not read — a tombstone's emptiness is not the
 *  point. */
function EditorVoiceWarning({ ownerId, voice }: { ownerId: string; voice: VoiceRef }) {
  if (voice.deleted) {
    return (
      <div className="mt-6">
        <DeletedVoiceWarning ownerId={ownerId} voice={voice} />
      </div>
    )
  }
  if (!voice.id) return null
  return <EmptyProfileWarning ownerId={ownerId} voiceId={voice.id} />
}

function EmptyProfileWarning({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { profile } = useVoiceProfile(ownerId, voiceId)
  const warning = <VoiceWarning profile={profile} voiceId={voiceId} />
  if (!profile) return null
  return <div className="mt-6 empty:hidden">{warning}</div>
}

/** The editor's docked bar. It holds the one thing a phone could not otherwise reach — the step's
 *  committing action, which sat in normal flow roughly 1,000px down a 4,000px page (§4.3) — the
 *  brief and the assignments that decide what that action does, and the reason the action is
 *  refused. Nothing that is merely TRUE: the save state, the job's progress and the post's status
 *  all moved to the page-top status region (`EditorStatus`, change 15).
 *
 *  It is mounted as the LAST child of `main` on template: a `sticky bottom-*` box is only pinned
 *  while its containing block still extends below it, so anywhere earlier in the flow it would
 *  scroll away with the section it sat in. */
function EditorDock({
  header,
  children,
}: {
  /** The bar's first row: 글 생성 puts 말투 and the writing brief's glyph here, above whatever the
   *  step's own actions are. */
  header?: ReactNode
  children?: ReactNode
}) {
  const { t } = useTranslation('posts')
  // A bar holding neither a control nor a refusal is not chrome (§0). Now that no status line
  // lives here, that is the whole test.
  if (!children && !header) return null
  return (
    <>
      {/* `mt-auto` alone can resolve to zero when the page is taller than the viewport. Keep a
          real 24px spacer as well, so the step panel never touches the dock card. */}
      <div aria-hidden className="mt-auto h-6 shrink-0" />
      <ActionBar ariaLabel={t('editor.actionAria')} className="mt-0">
        <div className="flex flex-col gap-2">
          {header}
          {children}
        </div>
      </ActionBar>
    </>
  )
}

/** A step the post has not reached yet. Never a disabled tab: the point of showing all three from
 *  the first screen is that the shape of the flow is visible, so an empty step says what it is
 *  waiting for and offers the way to the step that produces it. */
function EmptyStep({
  children,
  goTo,
  goToLabel,
}: {
  children: ReactNode
  goTo: () => void
  goToLabel: string
}) {
  return (
    <div className="mt-10">
      <Typography variant="body" className="text-content-tertiary">
        {children}
      </Typography>
      <Button variant="secondary" className="mt-4" onClick={goTo}>
        {goToLabel}
      </Button>
    </div>
  )
}

function LifecycleSteps({
  post,
  ownerId,
  step,
  onStepChange,
  titleField,
  memoField,
  dockHeader,
  onOpenBrief,
  targetLength,
  onTitleFinalized,
  beforeStart,
  ensureSlug,
  jobView,
}: {
  post: PostDraft
  ownerId: string
  step: EditorStep
  onStepChange: (step: EditorStep) => void
  titleField: ReactNode
  memoField: ReactNode
  dockHeader: ReactNode
  onOpenBrief: () => void
  targetLength?: number
  /** Re-seeds the editor's local 가제 with what 확정 wrote into `posts.title`. */
  onTitleFinalized: (title: string) => void
  beforeStart: () => Promise<void>
  ensureSlug: () => Promise<string>
  /** The durable job, resolved by the page so the status region and these panels read one poll. */
  jobView: EditorJobView
}) {
  const { t } = useTranslation('posts')
  const navigate = useNavigate()
  const generateRef = useRef<GenerationActionsHandle>(null)
  const reviseRef = useRef<ReviseFormHandle>(null)
  const contentEditorRef = useRef<BlockEditorHandle>(null)
  // A view URL is presigned and short-lived, and this screen outlives one: the post query is
  // refetched on mount and never again while the editor sits open, and a draft save deliberately
  // patches the cached image list in place rather than invalidating it. The export panel asks for
  // a fresh set when one of its photos fails to load — the only moment a dead URL costs anything,
  // because a photo that HAS painted is copied from its own pixels.
  const refreshPhotoUrls = useRefreshPostImages(post.slug)
  // Above both panels on template: 확정하고 말투 학습 lives at the end of 글 다듬기 and every
  // learning outcome is reported on 글 완성, so the run has to outlive the step change that the
  // finalize itself causes.
  const learning = useVoiceLearning(ownerId, post)
  const { user } = useSession()
  const languageMismatch = voiceContentLanguageMismatch(
    post.contentLanguage,
    post.voice.sourceLanguage,
  )
  const { job, jobId, jobStep } = jobView
  const result = hasContent(post) ? post.content : undefined
  // What export renders. The block editor's unsaved edits are newer than the server's copy, but only
  // until the server's revision moves past them — a completed revision or a landed save makes the
  // server authoritative again. Tagging the reported content with the revision it was edited from is
  // what expires it, so leaving 글 다듬기 cannot freeze export on content the post has since passed.
  const [edited, setEdited] = useState<{ revision: bigint; content: PostContent }>()
  const liveContent = edited?.revision === post.contentRevision ? edited.content : result
  // Stable except when the revision moves: `BlockEditor` reports its content from an effect that
  // depends on this callback, so a new identity every render would loop.
  const reportEdited = useCallback(
    (content: PostContent) => setEdited({ revision: post.contentRevision, content }),
    [post.contentRevision],
  )
  // 제목 · 메모 · 사진 · the voice caveat · the contact sheet. Everything that DESCRIBES the next
  // AI run left this panel for the one brief surface in the dock, so what is left is the post's
  // own material (change 12).
  const generatePanel = (
    <>
      {titleField}
      {memoField}
      <EditorPhotos post={post} ensureSlug={ensureSlug} />
      <EditorVoiceWarning ownerId={ownerId} voice={post.voice} />

      {post.pendingExperimentId && (!job || isTerminal(job)) && (
        <Link
          to="/ai-models/experiments/$id"
          params={{ id: post.pendingExperimentId }}
          className={typographyStyles({
            variant: 'label',
            className:
              'bg-notice-info-bg text-notice-info-fg mt-6 flex min-h-11 items-center rounded-md px-3 py-2',
          })}
        >
          {t('editor.reviewAiResult')}
        </Link>
      )}

      {post.images.length > 0 && (
        <ContactSheet images={post.images} observations={post.observations} activeJob={job} />
      )}
    </>
  )

  const refinePanel = result ? (
    <>
      {languageMismatch && (
        <Notice tone="warning" role="status" className="mb-4">
          {voiceContentLanguageMismatchReason()}
        </Notice>
      )}
      {/* 문장 의견 is NOT here. The server requires a completed voice-learning event for the
          post before it will accept feedback, and a post on this step is in `review` — never
          finalized, never learned — so the control failed on the ordinary path every time. It
          lives on 글 완성 now, behind the same condition the server enforces (change 16). */}
      <BlockEditor
        key={`${post.slug}:${post.machineBaselineRevision}`}
        ref={contentEditorRef}
        post={post}
        onContentChange={reportEdited}
      />
    </>
  ) : (
    <EmptyStep goTo={() => onStepChange('generate')} goToLabel={t('editor.goGenerate')}>
      {t('editor.refineEmpty')}
    </EmptyStep>
  )

  const finishPanel = result ? (
    <>
      {ownerId && (
        <VoiceLearningPanel learning={learning} onBackToRefine={() => onStepChange('refine')} />
      )}
      {/* Offered exactly where it works: `learning.learned` is "this post's learning run
          completed", which is the precondition the server checks. Feedback is evidence for the
          voice, and a deleted voice cannot take any, nor can a post whose language differs from
          the voice's source — both are server refusals, so the control is not offered rather
          than offered to fail. The text is the FINALIZED text the user is looking at, and the
          flush goes through the slug's content queue because the block editor that owned the
          live ref is mounted on the previous step. */}
      {learning.learned && !post.voice.deleted && !languageMismatch && (
        <SentenceFeedback
          postSlug={post.slug}
          text={sentenceSource(liveContent ?? result)}
          beforeSubmit={() =>
            (flushContentQueue(post.slug) ?? Promise.resolve(0n)).then(() => undefined)
          }
        />
      )}
      {post.contentLanguage ? (
        <ExportPanel
          content={liveContent ?? result}
          images={post.images}
          createdAt={post.createdAt}
          contentLanguage={post.contentLanguage}
          onPhotoUrlsStale={refreshPhotoUrls}
        />
      ) : (
        <Notice tone="danger" role="alert" className="mt-10">
          {t('export.languageMissing')}
        </Notice>
      )}
      {/* Publishing is the operator's surface (plan 17): every PublishingService procedure is
          refused to another tier, so the panel would show a pair-and-publish flow that cannot
          complete. The server stays authoritative — this only keeps the promise off the screen. */}
      {user?.plan === 'master' && (
        <PublishPanel
          ownerId={ownerId}
          postSlug={post.slug}
          contentRevision={post.contentRevision}
          finalizedRevision={post.finalizedRevision}
          status={post.status}
          beforePublish={() =>
            contentEditorRef.current?.flush() ??
            flushContentQueue(post.slug) ??
            Promise.resolve(post.contentRevision)
          }
        />
      )}
    </>
  ) : (
    <EmptyStep goTo={() => onStepChange('generate')} goToLabel={t('editor.goGenerate')}>
      {t('editor.finishEmpty')}
    </EmptyStep>
  )

  // A FAILURE, and only a failure. It stays in the dock while the job's progress moved to the
  // page-top bar, because a failure carries a retry: something the user can act on is a control,
  // not a status (change 15). It is computed up here because it is also what decides whether the
  // bar exists at all on 글 완성 — a bar holding nothing is chrome with nothing to say (§0).
  //
  // The RETRY is offered only on the step that owns the job, because that is where the control it
  // calls is mounted.
  const jobNotice = !jobId ? null : jobView.isError ? (
    <FailureNotice message={t('editor.jobLoadFailed')} onRetry={jobView.refetch} />
  ) : job?.status === 'failed' ? (
    <FailureNotice
      failure={job.failure}
      onRetry={
        step !== jobStep || (job.kind === 'revise' && jobView.startedStep === undefined)
          ? undefined
          : () =>
              job.kind === 'revise'
                ? reviseRef.current?.start()
                : job.kind === 'model_experiment'
                  ? post.pendingExperimentId
                    ? void navigate({
                        to: '/ai-models/experiments/$id',
                        params: { id: post.pendingExperimentId },
                      })
                    : undefined
                  : generateRef.current?.startGeneration()
      }
    />
  ) : null

  return (
    <>
      <div id={STEP_PANEL_ID} role="tabpanel" aria-label={editorStepLabel(step)}>
        {step === 'generate' ? generatePanel : step === 'refine' ? refinePanel : finishPanel}
      </div>

      {/* A finished job announces itself and takes no space. The visible success banner this
          replaced stood on EVERY step, on the bar the draft is read past, and nothing ever took
          it down: `done` is a standing STATE, not an event, so it came back on the next render
          for as long as the post's last job was that one, and its 결과 보기 only changed the step
          it was already sitting on (owner decision 2026-09-02). What it announced is not lost —
          the status change carries the user to 글 다듬기 with the draft in front of them — so what
          is kept is the part a screen reader has no other way to get.

          Mounted at all times and outside the dock's own existence test, so a bar with nothing
          visible to say is still not rendered (§0). A live region inserted with its text already
          inside announces nothing, which is exactly right: this speaks on the transition to
          `done` and stays silent for a job that was already finished when the editor mounted. */}
      <p className="sr-only" role="status">
        {job?.status === 'done' ? t('editor.generationComplete') : ''}
      </p>

      {/* ① and ② both always dock: 생성 ends the first step and 확정 ends the second, and the
          draft between them is routinely thousands of pixels tall (§4.3). ③ still docks only when
          the job has something to report. There is exactly ONE ActionBar in this scroller either
          way — the revision and finalize sections no longer exist in the panel. */}
      {(step === 'generate' || (step === 'refine' && Boolean(result)) || jobNotice) && (
        <EditorDock header={step === 'generate' ? dockHeader : undefined}>
          {jobNotice}
          {step === 'generate' && (
            <GenerationActions
              ref={generateRef}
              post={post}
              targetLength={targetLength}
              activeJob={job}
              jobPending={jobView.isPending}
              onStarted={(id) => jobView.onStarted(id, 'generate')}
              beforeStart={beforeStart}
              onOpenBrief={onOpenBrief}
            />
          )}
          {step === 'refine' && result && (
            <RefineDock
              ref={reviseRef}
              ownerId={ownerId}
              post={post}
              ruleLanguageMismatch={languageMismatch}
              learning={learning}
              activeJob={job}
              jobPending={jobView.isPending}
              onRevisionStarted={(id) => jobView.onStarted(id, 'refine')}
              // The block editor is mounted in the panel above, so the flush is a live ref; the
              // queue's fallback covers the beat between a step change and its unmount. A finalize
              // may never name a revision that omits an edit the user has already made.
              beforeStart={() =>
                (contentEditorRef.current?.flush() ?? Promise.resolve()).then(() => undefined)
              }
              beforeFinalize={() =>
                contentEditorRef.current?.flush() ??
                flushContentQueue(post.slug) ??
                Promise.resolve(post.contentRevision)
              }
              onFinalized={(finalizedTitle) => {
                // 확정 copies the AI title into `posts.title` on the server. The editor still holds
                // the 가제 in state and `useAutosave` sends it on every save, so without this the
                // next keystroke would write the placeholder straight back over it.
                onTitleFinalized(finalizedTitle)
                onStepChange('finish')
              }}
            />
          )}
        </EditorDock>
      )}
    </>
  )
}
