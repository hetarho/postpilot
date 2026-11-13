import type { ComponentProps, ReactNode, RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import type { GenerationJob } from '@/entities/generation-job'
import { isPublished, type PostDraft } from '@/entities/post'
import { flushContentQueue, type BlockEditorHandle } from '@/features/edit-post-content'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import {
  StorylineActionBlocker,
  StorylineActionRow,
  StorylineActionsProvider,
  StorylineRequestComposer,
  type StorylineActionsHandle,
} from '@/features/generate-post'
import { StepBar, StepBarActions, StepBarBack, StepBarCta } from '@/shared/ui'
import { RefineDock } from '@/widgets/refine-dock'
import type { EditorStep } from '../model/steps'
import type { EditorJobView } from '../model/useEditorJob'

type StorylineActionsProps = Omit<
  ComponentProps<typeof StorylineActionsProvider>,
  'post' | 'children' | 'ref'
> & { ref: RefObject<StorylineActionsHandle | null> }

/** The step bar of ② 관찰, ③ 스토리라인, ④ 다듬기 and ⑤ 완성 (THEME-71), on the same `StepBar` as
 *  ① 생성's guided flow (EditorGuide), so this docks only on the four later steps. There is
 *  exactly ONE bar in this scroller either way:
 *
 *  - ②: 이전 · 스토리라인 먼저 · 바로 글 쓰기, with the writing-test entry (POST-53);
 *  - ③: the storyline request field, then 이전 · 다시 만들기 · the write action (POST-97);
 *  - ④: the revision field, then 이전 back to ③ and 확정하기 (POST-56);
 *  - ⑤: only a failure that has something to say (POST-57).
 *
 *  A published post's ②, ③ and ④ hold only the road onward to 글 완성 beside the way back
 *  (POST-86). It is mounted as the LAST child of `main`: a `sticky bottom-*` box is only pinned
 *  while its containing block still extends below it. A bar holding neither a control nor a
 *  refusal is not chrome (THEME-8). */
export function EditorStepDock({
  post,
  ownerId,
  step,
  result,
  jobView,
  jobNotice,
  hasJobNotice,
  editorRef,
  reviseRef,
  observeActions,
  storylineActions,
  onTitleFinalized,
  onStepChange,
}: {
  post: PostDraft
  ownerId: string
  step: EditorStep
  /** Whether a run has produced content; ④'s dock is about committing it. */
  result: boolean
  jobView: EditorJobView
  jobNotice: ReactNode
  hasJobNotice: boolean
  editorRef: RefObject<BlockEditorHandle | null>
  reviseRef: RefObject<ReviseFormHandle | null>
  /** ②'s row, led by the way back the dock hands it. */
  observeActions: (back: ReactNode) => ReactNode
  /** What ③'s storyline actions start with and report to. */
  storylineActions: StorylineActionsProps
  onTitleFinalized: (title: string) => void
  onStepChange: (step: EditorStep) => void
}) {
  const { t } = useTranslation('posts')
  const job: GenerationJob | undefined = jobView.job
  if (step === 'generate') return null
  const backTo = (target: EditorStep) => (
    <StepBarBack
      label={t('guide.back')}
      onClick={() => {
        onStepChange(target)
        window.scrollTo(0, 0)
      }}
    />
  )
  const onward = (target: EditorStep) => (
    <StepBarActions back={backTo(target)}>
      <StepBarCta onClick={() => onStepChange('finish')}>{t('finalize.goFinish')}</StepBarCta>
    </StepBarActions>
  )
  const published = isPublished(post)

  if (step === 'observe')
    return (
      <StepBar ariaLabel={t('editor.actionAria')}>
        {jobNotice}
        {published ? onward('generate') : observeActions(backTo('generate'))}
      </StepBar>
    )

  if (step === 'storyline') {
    if (!post.storyline)
      return hasJobNotice ? <StepBar ariaLabel={t('editor.actionAria')}>{jobNotice}</StepBar> : null
    const { ref, ...actions } = storylineActions
    return (
      <StepBar ariaLabel={t('editor.actionAria')}>
        {jobNotice}
        {published ? (
          onward('observe')
        ) : (
          <StorylineActionsProvider ref={ref} post={post} {...actions}>
            <StorylineActionBlocker />
            <StorylineRequestComposer />
            <StorylineActionRow back={backTo('observe')} />
          </StorylineActionsProvider>
        )}
      </StepBar>
    )
  }

  if (!((step === 'refine' && Boolean(result)) || hasJobNotice)) return null
  return (
    <StepBar ariaLabel={t('editor.actionAria')}>
      {jobNotice}
      {step === 'refine' && result && (
        <RefineDock
          ref={reviseRef}
          back={backTo('storyline')}
          ownerId={ownerId}
          post={post}
          activeJob={job}
          jobPending={jobView.isPending}
          onRevisionStarted={(id) => jobView.onStarted(id, 'refine')}
          // The block editor is mounted in the panel above, so the flush is a live ref; the
          // queue's fallback covers the beat between a step change and its unmount. A finalize
          // may never name a revision that omits an edit the user has already made.
          beforeStart={() =>
            (editorRef.current?.flush() ?? Promise.resolve()).then(() => undefined)
          }
          beforeFinalize={() =>
            editorRef.current?.flush() ??
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
    </StepBar>
  )
}
