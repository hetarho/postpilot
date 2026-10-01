import { forwardRef, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft, PostStorylineParagraph } from '@/entities/post'
import {
  StorylineAttachmentViewer,
  StorylineParagraphEditor,
  TakenOutFiles,
  storylineAttachments,
  storylineViewOrder,
  takenOutFiles,
} from '@/features/edit-storyline'
import {
  StorylineActionBlocker,
  StorylineActionButtons,
  StorylineActionsProvider,
  StorylineRequestComposer,
  type GenerationMode,
  type StorylineActionsHandle,
} from '@/features/generate-post'
import { Disclosure, Notice } from '@/shared/ui'

type SpacePost = Pick<
  PostDraft,
  | 'slug'
  | 'status'
  | 'images'
  | 'videos'
  | 'observations'
  | 'pendingExperimentId'
  | 'voice'
  | 'storyline'
  | 'content'
  | 'contentRevision'
  | 'machineBaselineRevision'
>

/** ②'s storyline space (POST-95 – POST-98): the post's storyline in a space that opens and closes
 *  above the draft — its paragraphs with their photos and clips, edited by hand, the ones taken
 *  out to put back — under a heading row that remakes it, writes from it and asks for a change.
 *
 *  It is open while the post has no content, since the storyline is then all ② has, and it closes
 *  once when content first appears while it is mounted — the draft is what the owner came back for.
 *  Opening or closing changes nothing else. */
export const StorylineSpace = forwardRef<
  StorylineActionsHandle,
  {
    post: SpacePost
    /** The paragraphs as ② shows them: the owner's unsaved edit, or the server's. */
    paragraphs: readonly PostStorylineParagraph[]
    onChange: (paragraphs: PostStorylineParagraph[]) => void
    /** While a job targets the post, on a published post and while a save is being refused
     *  (POST-86): no field and no move control. */
    readOnly: boolean
    hasContent: boolean
    /** What the space's actions start with and report to. */
    actions: {
      targetLength?: number
      activeJob?: GenerationJob
      jobPending?: boolean
      onStarted: (jobId: string) => void
      beforeStart: () => Promise<void>
      checkRequiredAnswers?: () => boolean
      flushContent: () => Promise<unknown>
      onOpenBrief: (mode: GenerationMode) => void
    }
  }
>(function StorylineSpace({ post, paragraphs, onChange, readOnly, hasContent, actions }, ref) {
  const { t } = useTranslation('posts')
  const [open, setOpen] = useState(!hasContent)
  const closedForContent = useRef(hasContent)
  useEffect(() => {
    if (!hasContent || closedForContent.current) return
    closedForContent.current = true
    setOpen(false)
  }, [hasContent])
  const attachments = useMemo(() => storylineAttachments(post), [post])
  // One large view for the whole space, because its previous and next walk every paragraph and
  // the taken-out row (POST-100).
  const [viewing, setViewing] = useState<string | null>(null)
  const storyline = post.storyline
  if (!storyline) return null
  const takenOut = takenOutFiles(storyline, paragraphs)
  const viewOrder = storylineViewOrder(
    paragraphs,
    takenOut,
    (file) => !!attachments.get(file)?.viewUrl,
  )

  return (
    <StorylineActionsProvider ref={ref} post={post} {...actions}>
      <div className="mb-6">
        <StorylineActionBlocker />
        <Disclosure
          title={t('storylineSpace.heading')}
          open={open}
          onOpenChange={setOpen}
          aside={<StorylineActionButtons />}
          lead={<StorylineRequestComposer />}
        >
          {storyline.addedFiles.length > 0 && (
            <Notice tone="info" role="status" className="mt-2">
              {t('storylineSpace.added')}
            </Notice>
          )}
          <ol className="divide-divider mt-2 divide-y">
            {paragraphs.map((_, index) => (
              <StorylineParagraphEditor
                key={index}
                paragraphs={paragraphs}
                index={index}
                attachments={attachments}
                readOnly={readOnly}
                onChange={onChange}
                onView={setViewing}
              />
            ))}
          </ol>
          <TakenOutFiles
            files={takenOut}
            paragraphs={paragraphs}
            attachments={attachments}
            readOnly={readOnly}
            onChange={onChange}
            onView={setViewing}
          />
        </Disclosure>
        <StorylineAttachmentViewer
          files={viewOrder}
          attachments={attachments}
          viewing={viewing}
          onView={setViewing}
          onClose={() => setViewing(null)}
        />
      </div>
    </StorylineActionsProvider>
  )
})
