import { useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from '@tanstack/react-router'
import { type PostDraft, type PostTemplateAnswer } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useTemplates } from '@/entities/template'
import { discardContentQueue, useCaretHandoff } from '@/features/edit-post-content'
import { DeletePostButton } from '@/features/delete-post'
import { discardDraftQueue, peekPendingDraft, useAutosave } from '@/features/save-draft'
import { useBriefMirror } from '@/features/generate-post'
import {
  TemplateAnswerFields,
  answerFields,
  toAnswerPatch,
  withAnswer,
} from '@/features/fill-template-answers'
import { SegmentedControl, typographyStyles, type PopoverHandle, pageStyles } from '@/shared/ui'
import { editorSteps } from '../model/steps'
import { useDraftAssignments } from '../model/useDraftAssignments'
import { useDraftSteps } from '../model/useDraftSteps'
import { useEditorJob } from '../model/useEditorJob'
import { EditorPhotos } from './EditorPhotos'
import { EditorProgressBar, EditorStatusLine } from './EditorStatus'
import { EditorDock } from './EditorDock'
import { EditorDockHeader } from './EditorDockHeader'
import { EditorVoiceWarning } from './EditorVoiceWarning'
import { LifecycleSteps } from './LifecycleSteps'
import { MemoField } from './MemoField'
import { TitleField } from './TitleField'

const STEP_PANEL_ID = 'editor-step-panel'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
  /** The voice a draft with no post yet starts in — the account's default, resolved by the
   *  route before this mounts, so the first save always carries a concrete id
   *  (spec/legacy/policy/posts.md). Ignored for an existing post, whose voice is its own. */
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
  const caretFields = useMemo(() => ({ title: titleRef, memo: memoRef }), [])
  const caret = useCaretHandoff(post?.slug, caretFields)

  // Text still queued for this post outranks what the server reported: it is what the
  // previous editor was in the middle of saving when the mint moved the URL, so it is
  // newer by exactly the characters typed during that round trip.
  const opening = post
    ? (peekPendingDraft(post.slug) ?? { title: post.title, memo: post.memo })
    : { title: '', memo: '' }
  const [title, setTitle] = useState(opening.title)
  const [memo, setMemo] = useState(opening.memo)

  const assignments = useDraftAssignments(post, defaultVoiceId)
  const { voiceId, templateId, targetLanguage } = assignments

  // The template's data fields, as ① renders them. They are derived from the SELECTED template's
  // body and the post's stored answers, with the local edits laid over the top — the queue owns
  // durability, so this state only has to survive between keystrokes (POST-62).
  const { templates } = useTemplates(ownerId)
  const selectedTemplate = templates.find((candidate) => candidate.id === templateId)
  const [answerEdits, setAnswerEdits] = useState<PostTemplateAnswer[]>([])
  const templateAnswers = post?.templateAnswers
  const fields = useMemo(() => {
    const merged = [...(templateAnswers ?? [])]
    for (const edit of answerEdits) {
      const at = merged.findIndex((answer) => answer.label === edit.label)
      if (at === -1) merged.push(edit)
      else merged[at] = edit
    }
    return answerFields(selectedTemplate, merged)
  }, [selectedTemplate, templateAnswers, answerEdits])
  // A stable identity per content, because the queue compares drafts by value and `useAutosave`
  // holds this in a dependency array.
  const answers = useMemo(() => toAnswerPatch(fields), [fields])

  const autosave = useAutosave({
    post,
    title,
    memo,
    answers,
    voiceId,
    templateId,
    targetLanguage,
    onMinted: (slug) => {
      caret.stash(slug)
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
  const { step, select: setStep } = useDraftSteps(post?.status ?? '')

  // The brief widget SETS the target length and the tag count while the generate action SENDS
  // them from another layer, so the screen they both hang off owns the values.
  const brief = useBriefMirror(post)
  const briefRef = useRef<PopoverHandle>(null)

  const titleField = (
    <TitleField value={title} onChange={setTitle} fieldRef={titleRef} nextRef={memoRef} />
  )
  const memoField = <MemoField value={memo} onChange={setMemo} fieldRef={memoRef} />
  const answerFieldsPanel = (
    <TemplateAnswerFields
      fields={fields}
      onChange={(label, change) => setAnswerEdits(toAnswerPatch(withAnswer(fields, label, change)))}
    />
  )

  const dockHeader = (
    <EditorDockHeader
      ref={briefRef}
      post={post}
      ownerId={ownerId}
      voiceId={voiceId}
      templateId={templateId}
      targetLanguage={targetLanguage}
      targetLength={brief.targetLength}
      tagCount={brief.tagCount}
      onVoiceSelect={post ? autosave.reassign : assignments.setVoiceId}
      onTemplateSelect={post ? autosave.assignTemplate : assignments.setTemplateId}
      onTargetLanguageSelect={post ? autosave.assignTargetLanguage : assignments.setTargetLanguage}
      onBriefSaved={(values) => {
        brief.setTargetLength(values.targetLength)
        brief.setTagCount(values.tagCount)
      }}
    />
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
            photoCount={post?.images.length ?? 0}
            videoCount={post?.videos.length ?? 0}
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
          answerFields={answerFieldsPanel}
          dockHeader={dockHeader}
          onOpenBrief={() => briefRef.current?.open()}
          targetLength={brief.targetLength}
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
          {answerFieldsPanel}
          <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />
          <EditorVoiceWarning ownerId={ownerId} voice={assignments.voice} />
          {/* A draft with no post yet has no committing action, but its 말투 and the rest of the
              brief still have to be reachable before the first word is typed. */}
          <EditorDock header={dockHeader} />
        </>
      )}
    </main>
  )
}
