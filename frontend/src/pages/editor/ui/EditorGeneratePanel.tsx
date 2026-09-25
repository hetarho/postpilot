import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { typographyStyles } from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { EditorPhotos } from './EditorPhotos'
import { EditorVoiceWarning } from './EditorVoiceWarning'

/** ①'s panel: the post's own material — 제목 · 메모 · 템플릿 답변 · 사진 · the voice caveat
 *  (POST-54). Everything that DESCRIBES the next AI run, 분야 and 기억 사용 included, lives in the
 *  dock's brief instead (POST-89). */
export function EditorGeneratePanel({
  post,
  ownerId,
  titleField,
  memoField,
  answerFields,
  ensureSlug,
  job,
  locked = false,
}: {
  post: PostDraft
  ownerId: string
  titleField: ReactNode
  memoField: ReactNode
  answerFields: ReactNode
  ensureSlug: () => Promise<string>
  job?: GenerationJob
  /** A published post: its material is shown and nothing in it is changed (POST-86). */
  locked?: boolean
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      {titleField}
      {memoField}
      {answerFields}
      <EditorPhotos post={post} ensureSlug={ensureSlug} locked={locked} />
      <EditorVoiceWarning ownerId={ownerId} voice={post.voice} />

      {post.pendingExperimentId && (!job || isTerminal(job)) && (
        <Link
          to="/posts/experiments/$id"
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
        <ContactSheet
          images={post.images}
          videos={post.videos}
          observations={post.observations}
          activeJob={job}
        />
      )}
    </>
  )
}
