import { useTranslation } from 'react-i18next'
import type { PostStorylineParagraph } from '@/entities/post'
import { ActionMenu, Typography } from '@/shared/ui'
import { withFileIn } from '../model/storyline-edits'
import { StorylineTile, type StorylineAttachment } from './StorylineParagraphEditor'

/** 빠진 사진 (POST-96): the attachments the storyline was made with that no paragraph holds, each
 *  with 넣기 choosing the paragraph it goes back into. An attachment added after the storyline was
 *  made is never here — it was never part of it. Nothing renders when none is taken out. */
export function TakenOutFiles({
  files,
  paragraphs,
  attachments,
  readOnly,
  onChange,
}: {
  files: readonly string[]
  paragraphs: readonly PostStorylineParagraph[]
  attachments: ReadonlyMap<string, StorylineAttachment>
  readOnly: boolean
  onChange: (paragraphs: PostStorylineParagraph[]) => void
}) {
  const { t } = useTranslation('posts')
  if (files.length === 0) return null
  return (
    <section aria-labelledby="storyline-taken-out" className="mt-4">
      <Typography variant="label" as="h3" id="storyline-taken-out">
        {t('storylineEdit.takenOut')}
      </Typography>
      <ul className="mt-2 flex flex-wrap gap-3">
        {files.map((file) => (
          <li key={file} className="flex flex-col items-start gap-1">
            <StorylineTile attachment={attachments.get(file)} filename={file} />
            {!readOnly && (
              <ActionMenu
                label={t('storylineEdit.putBack', { file })}
                triggerLabel={t('storylineEdit.putBackLabel')}
                items={paragraphs.map((_, at) => ({
                  id: `p${at}`,
                  label: t('storylineEdit.paragraph', { n: at + 1 }),
                  onSelect: () => onChange(withFileIn(paragraphs, file, at)),
                }))}
              />
            )}
          </li>
        ))}
      </ul>
    </section>
  )
}
