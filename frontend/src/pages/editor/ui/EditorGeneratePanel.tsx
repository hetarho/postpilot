import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { typographyStyles } from '@/shared/ui'
import { UseMemoriesField } from '@/features/use-post-memories'
import { ContactSheet } from '@/widgets/contact-sheet'
import { EditorPhotos } from './EditorPhotos'
import { EditorVoiceWarning } from './EditorVoiceWarning'

/** ①'s panel: the post's own material — 제목 · 메모 · 템플릿 답변 · 분야 · 사진 · the voice caveat.
 *  Everything that DESCRIBES the next AI run lives in the dock's brief instead. */
export function EditorGeneratePanel({
  post,
  ownerId,
  titleField,
  memoField,
  answerFields,
  fieldPicker,
  ensureSlug,
  job,
  locked = false,
}: {
  post: PostDraft
  ownerId: string
  titleField: ReactNode
  memoField: ReactNode
  answerFields: ReactNode
  /** The post's 분야: what the post is about, so it follows the data fields (POST-54). */
  fieldPicker: ReactNode
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
      {fieldPicker}
      {/* At the foot of the memo and the data fields, which is what it is about: whether this
          draft's run may carry the account's memories (POST-71). It is NOT in the writing brief
          — it silently changes what a run may write. */}
      <UseMemoriesField
        slug={post.slug}
        useMemory={post.useMemory}
        targetLength={post.targetLength}
        disabled={locked}
        className="mt-6"
      />
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
