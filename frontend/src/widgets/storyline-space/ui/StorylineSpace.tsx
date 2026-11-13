import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  AttachmentViewer,
  postAttachments,
  type PostDraft,
  type PostStorylineParagraph,
} from '@/entities/post'
import {
  StorylineParagraphEditor,
  TakenOutFiles,
  storylineViewOrder,
  takenOutFiles,
} from '@/features/edit-storyline'
import { Notice, Typography } from '@/shared/ui'

type SpacePost = Pick<PostDraft, 'images' | 'videos' | 'storyline'>

/** ③ 스토리라인 as the whole step (POST-95 – POST-100): the post's storyline — its paragraphs with
 *  their photos and clips, edited by hand, the ones taken out to put back — and the notice that
 *  attachments were added since it was made. Its actions stand in ③'s step bar, not here. */
export function StorylineSpace({
  post,
  paragraphs,
  onChange,
  readOnly,
}: {
  post: SpacePost
  /** The paragraphs as ③ shows them: the owner's unsaved edit, or the server's. */
  paragraphs: readonly PostStorylineParagraph[]
  onChange: (paragraphs: PostStorylineParagraph[]) => void
  /** While a job targets the post, on a published post and while a save is being refused
   *  (POST-86): no field and no move control. */
  readOnly: boolean
}) {
  const { t } = useTranslation('posts')
  const attachments = useMemo(() => postAttachments(post), [post])
  // One large view for the whole step, because its previous and next walk every paragraph and
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
    <section aria-labelledby="storyline-space-heading" className="mb-6">
      <Typography variant="title" id="storyline-space-heading">
        {t('storylineSpace.heading')}
      </Typography>
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
      <AttachmentViewer
        files={viewOrder}
        attachments={attachments}
        viewing={viewing}
        onView={setViewing}
        onClose={() => setViewing(null)}
      />
    </section>
  )
}
