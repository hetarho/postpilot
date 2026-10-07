import type { ReactNode } from 'react'
import { type GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { proseStyles } from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { EditorPhotos } from './EditorPhotos'
import { EditorVoiceWarning } from './EditorVoiceWarning'

/** ①'s panel: the post's own material — 제목 · 템플릿 답변 · 메모 · 사진 · the voice caveat
 *  (POST-54). The memo follows the template's fields because it carries what they did not ask. Everything that DESCRIBES the next AI run, 분야 and 기억 사용 included, lives in the
 *  dock's brief instead (POST-89). */
export function EditorGeneratePanel({
  post,
  ownerId,
  titleField,
  memoField,
  answerFields,
  ensureSlug,
  job,
}: {
  post: PostDraft
  ownerId: string
  titleField: ReactNode
  memoField: ReactNode
  answerFields: ReactNode
  ensureSlug: () => Promise<string>
  job?: GenerationJob
}) {
  return (
    <div className={proseStyles()}>
      {titleField}
      {answerFields}
      {memoField}
      <EditorPhotos post={post} ensureSlug={ensureSlug} />
      <EditorVoiceWarning ownerId={ownerId} voice={post.voice} />

      {post.images.length > 0 && (
        <ContactSheet
          images={post.images}
          videos={post.videos}
          observations={post.observations}
          activeJob={job}
        />
      )}
    </div>
  )
}
