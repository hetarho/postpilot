import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { BlockList, isPublished, type PostDraft } from '@/entities/post'
import { PostMeasurementRow } from '@/entities/quality'
import { BlockEditor, type BlockEditorHandle } from '@/features/edit-post-content'
import { WritingOriginReview, type WritingOriginVoice } from '@/features/review-writing-origins'
import type { PostContent } from '@/shared/api'
import { Notice } from '@/shared/ui'
import { usePostFingerprint } from '@/entities/voice'
import { PostVoiceReading } from '@/widgets/voice-fingerprint'
import { EmptyStep } from './EmptyStep'

/** ④'s panel: the block editor over the generated draft, or what it waits for — 이 스토리로 글 쓰기 —
 *  with the way to ③ when there is nothing written yet (POST-95). A published post reads its prose instead, under the one sentence that
 *  says why and how to reopen it (POST-86): no editor is mounted, so no content save can start. */
export function EditorRefinePanel({
  post,
  ownerId,
  result,
  editorRef,
  onContentChange,
  onGoStoryline,
}: {
  post: PostDraft
  /** The signed-in account, whose readings the measurement row is keyed by. */
  ownerId: string
  /** The generated content this step edits; absent until a run has produced one. */
  result?: PostContent
  editorRef: RefObject<BlockEditorHandle | null>
  onContentChange: (content: PostContent) => void
  onGoStoryline: () => void
}) {
  const { t } = useTranslation('posts')
  // The post's voice reading is read once here, at the revision on screen, and heads 출처 보기's
  // 말투 view (POST-102, POST-128). A post with 말투 없음, or whose voice is deleted or not made,
  // asks for nothing and has 말투 disabled with its reason.
  const voiceReason = !post.voice
    ? 'none'
    : post.voice.deleted
      ? 'deleted'
      : !post.voice.made
        ? 'unmade'
        : undefined
  const { fingerprint, isError, isFetching, refetch } = usePostFingerprint(
    ownerId,
    post.slug,
    post.contentRevision,
    Boolean(result) && !voiceReason,
  )
  const voice: WritingOriginVoice = {
    applicable: !voiceReason && (fingerprint ? fingerprint.applicable : isError),
    reason: voiceReason,
    revision: fingerprint?.revision,
    habits: fingerprint?.habits ?? [],
    reading: (
      <PostVoiceReading
        fingerprint={fingerprint}
        isError={isError}
        isFetching={isFetching}
        onRetry={refetch}
      />
    ),
  }
  if (!result)
    return (
      <EmptyStep goTo={onGoStoryline} goToLabel={t('editor.goStoryline')}>
        {t('editor.refineEmpty')}
      </EmptyStep>
    )
  return (
    <>
      {isPublished(post) && (
        <Notice tone="info" role="status" className="mb-4">
          {t('published.locked')}
        </Notice>
      )}
      {isPublished(post) ? (
        <WritingOriginReview ownerId={ownerId} post={post} content={result} voice={voice}>
          {(fields) => (
            <BlockList content={result} images={post.images} videos={post.videos} {...fields} />
          )}
        </WritingOriginReview>
      ) : (
        <BlockEditor
          key={`${ownerId}:${post.slug}:${post.machineBaselineRevision}`}
          ref={editorRef}
          post={post}
          onContentChange={onContentChange}
          renderReview={({ content, pending, children }) => (
            <WritingOriginReview
              ownerId={ownerId}
              post={post}
              content={content}
              pending={pending}
              voice={voice}
            >
              {children}
            </WritingOriginReview>
          )}
          // This post's own M2, M3 and M4 (QUAL-36, POST-83) stand where they were; the voice
          // reading no longer has a row of its own (POST-102).
          beforeArticle={
            <PostMeasurementRow
              ownerId={ownerId}
              slug={post.slug}
              revision={post.contentRevision}
              contentLanguage={post.contentLanguage}
              className="mt-4"
            />
          }
        />
      )}
    </>
  )
}
