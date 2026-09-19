import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import type { PostDraft } from '@/entities/post'
import { voiceContentLanguageMismatchReason } from '@/entities/voice'
import { BlockEditor, type BlockEditorHandle } from '@/features/edit-post-content'
import type { PostContent } from '@/shared/api'
import { Notice } from '@/shared/ui'
import { EmptyStep } from './EmptyStep'

/** ②'s panel: the block editor over the generated draft, or the way back to ① when there is
 *  nothing generated yet. */
export function EditorRefinePanel({
  post,
  result,
  languageMismatch,
  editorRef,
  onContentChange,
  onGoGenerate,
}: {
  post: PostDraft
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
      {/* 문장 의견 is NOT here. The server requires a completed voice-learning event for the
          post before it will accept feedback, and a post on this step is in `review` — never
          finalized, never learned — so the control failed on the ordinary path every time. It
          lives on 글 완성 now, behind the same condition the server enforces (change 16). */}
      <BlockEditor
        key={`${post.slug}:${post.machineBaselineRevision}`}
        ref={editorRef}
        post={post}
        onContentChange={onContentChange}
      />
    </>
  )
}
