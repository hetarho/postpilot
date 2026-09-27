import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { PostDraft, PostStorylineParagraph } from '@/entities/post'
import {
  StorylineParagraphEditor,
  TakenOutFiles,
  storylineAttachments,
  takenOutFiles,
} from '@/features/edit-storyline'
import { Disclosure, Notice } from '@/shared/ui'

/** ②'s storyline space (POST-95, POST-96): the post's storyline in a space that opens and closes
 *  above the draft — its paragraphs with their photos and clips, edited by hand, and the ones
 *  taken out, to put back.
 *
 *  It is open while the post has no content, since the storyline is then all ② has, and it closes
 *  once when content first appears while it is mounted — the draft is what the owner came back for.
 *  Opening or closing changes nothing else. */
export function StorylineSpace({
  post,
  paragraphs,
  onChange,
  readOnly,
  hasContent,
  actions,
}: {
  post: Pick<PostDraft, 'storyline' | 'images' | 'videos'>
  /** The paragraphs as ② shows them: the owner's unsaved edit, or the server's. */
  paragraphs: readonly PostStorylineParagraph[]
  onChange: (paragraphs: PostStorylineParagraph[]) => void
  /** While a job targets the post, on a published post and while a save is being refused
   *  (POST-86): no field and no move control. */
  readOnly: boolean
  hasContent: boolean
  /** The space's own actions, under the paragraphs. */
  actions?: ReactNode
}) {
  const { t } = useTranslation('posts')
  const [open, setOpen] = useState(!hasContent)
  const closedForContent = useRef(hasContent)
  useEffect(() => {
    if (!hasContent || closedForContent.current) return
    closedForContent.current = true
    setOpen(false)
  }, [hasContent])
  const attachments = useMemo(() => storylineAttachments(post), [post])
  const storyline = post.storyline
  if (!storyline) return null
  const takenOut = takenOutFiles(storyline, paragraphs)

  return (
    <Disclosure
      title={t('storylineSpace.heading')}
      open={open}
      onOpenChange={setOpen}
      className="mb-6"
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
          />
        ))}
      </ol>
      <TakenOutFiles
        files={takenOut}
        paragraphs={paragraphs}
        attachments={attachments}
        readOnly={readOnly}
        onChange={onChange}
      />
      {actions}
    </Disclosure>
  )
}
