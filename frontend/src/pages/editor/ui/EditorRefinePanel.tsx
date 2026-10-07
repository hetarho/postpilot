import type { ReactNode, RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { BlockList, isPublished, type PostDraft } from '@/entities/post'
import { PostMeasurementRow } from '@/entities/quality'
import { BlockEditor, type BlockEditorHandle } from '@/features/edit-post-content'
import { WritingOriginReview } from '@/features/review-writing-origins'
import type { PostContent } from '@/shared/api'
import { Notice, Typography } from '@/shared/ui'
import { PostFingerprintRow } from '@/widgets/voice-fingerprint'
import { EmptyStep } from './EmptyStep'

/** ②'s panel: the block editor over the generated draft, or the way back to ① when there is
 *  nothing generated yet. A published post reads its prose instead, under the one sentence that
 *  says why and how to reopen it (POST-86): no editor is mounted, so no content save can start. */
export function EditorRefinePanel({
  post,
  ownerId,
  storylineSpace,
  result,
  editorRef,
  onContentChange,
  onGoGenerate,
}: {
  post: PostDraft
  /** The signed-in account, whose readings the measurement row is keyed by. */
  ownerId: string
  /** The storyline space, above the draft, when the post holds a storyline (POST-95). */
  storylineSpace?: ReactNode
  /** The generated content this step edits; absent until a run has produced one. */
  result?: PostContent
  editorRef: RefObject<BlockEditorHandle | null>
  onContentChange: (content: PostContent) => void
  onGoGenerate: () => void
}) {
  const { t } = useTranslation('posts')
  // A storyline and no post yet: the draft area waits for 이 스토리로 글 쓰기.
  if (!result && storylineSpace)
    return (
      <>
        {storylineSpace}
        <Typography variant="body" as="p" className="text-content-secondary">
          {t('storylineSpace.waiting')}
        </Typography>
      </>
    )
  if (!result)
    return (
      <EmptyStep goTo={onGoGenerate} goToLabel={t('editor.goGenerate')}>
        {t('editor.refineEmpty')}
      </EmptyStep>
    )
  return (
    <>
      {storylineSpace}
      {isPublished(post) && (
        <Notice tone="info" role="status" className="mb-4">
          {t('published.locked')}
        </Notice>
      )}
      {isPublished(post) ? (
        <WritingOriginReview ownerId={ownerId} post={post} content={result}>
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
            <WritingOriginReview ownerId={ownerId} post={post} content={content} pending={pending}>
              {children}
            </WritingOriginReview>
          )}
          // This post's own M2, M3 and M4 (QUAL-36), then its fingerprint beside its voice's
          // (POST-102), both read at the revision on screen; a post with 말투 없음 has none.
          beforeArticle={
            <>
              <PostMeasurementRow
                ownerId={ownerId}
                slug={post.slug}
                revision={post.contentRevision}
                contentLanguage={post.contentLanguage}
                className="mt-4"
              />
              {post.voice?.made && !post.voice.deleted && (
                <PostFingerprintRow
                  ownerId={ownerId}
                  slug={post.slug}
                  revision={post.contentRevision}
                  className="mt-4"
                />
              )}
            </>
          }
        />
      )}
    </>
  )
}
