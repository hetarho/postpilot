import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { BlockList, isPublished, type PostDraft } from '@/entities/post'
import { PostMeasurementRow } from '@/entities/quality'
import { voiceContentLanguageMismatchReason } from '@/entities/voice'
import { BlockEditor, type BlockEditorHandle } from '@/features/edit-post-content'
import type { PostContent } from '@/shared/api'
import { Notice } from '@/shared/ui'
import { EmptyStep } from './EmptyStep'

/** ②'s panel: the block editor over the generated draft, or the way back to ① when there is
 *  nothing generated yet. A published post reads its prose instead, under the one sentence that
 *  says why and how to reopen it (POST-86): no editor is mounted, so no content save can start. */
export function EditorRefinePanel({
  post,
  ownerId,
  result,
  languageMismatch,
  editorRef,
  onContentChange,
  onGoGenerate,
}: {
  post: PostDraft
  /** The signed-in account, whose readings the measurement row is keyed by. */
  ownerId: string
  /** The generated content this step edits; absent until a run has produced one. */
  result?: PostContent
  languageMismatch: boolean
  editorRef: RefObject<BlockEditorHandle | null>
  onContentChange: (content: PostContent) => void
  onGoGenerate: () => void
}) {
  const { t } = useTranslation('posts')
  if (!result)
    return (
      <EmptyStep goTo={onGoGenerate} goToLabel={t('editor.goGenerate')}>
        {t('editor.refineEmpty')}
      </EmptyStep>
    )
  return (
    <>
      {languageMismatch && (
        <Notice tone="warning" role="status" className="mb-4">
          {voiceContentLanguageMismatchReason()}
        </Notice>
      )}
      {isPublished(post) && (
        <Notice tone="info" role="status" className="mb-4">
          {t('published.locked')}
        </Notice>
      )}
      {/* 문장 의견 is NOT here. The server requires a completed voice-learning event for the
          post before it will accept feedback, and a post on this step is in `review` — never
          finalized, never learned — so the control failed on the ordinary path every time. It
          lives on 글 완성 now, behind the same condition the server enforces (change 16). */}
      {isPublished(post) ? (
        <BlockList content={result} images={post.images} videos={post.videos} />
      ) : (
        <BlockEditor
          key={`${post.slug}:${post.machineBaselineRevision}`}
          ref={editorRef}
          post={post}
          onContentChange={onContentChange}
          // This post's own M2, M3 and M4 (QUAL-36), read at the revision on screen. Never on a
          // published post, which has no editor here anyway (POST-86).
          beforeArticle={
            !isPublished(post) && (
              <PostMeasurementRow
                ownerId={ownerId}
                slug={post.slug}
                revision={post.contentRevision}
                contentLanguage={post.contentLanguage}
                className="mt-4"
              />
            )
          }
        />
      )}
    </>
  )
}
