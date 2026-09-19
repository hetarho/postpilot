import { forwardRef } from 'react'
import { isTerminal } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { PostTemplateSelect } from '@/features/select-post-template'
import { PostVoiceSelect, reassignmentBlocker } from '@/features/select-post-voice'
import type { ContentLanguage } from '@/shared/api'
import type { PopoverHandle } from '@/shared/ui'
import { GenerationBrief } from '@/widgets/generation-brief'

/** The dock's first row: 말투, 템플릿 and the writing brief's glyph.
 *
 *  Both selects ride the dock's own surface beside the brief rather than inside it: each is
 *  chosen per draft and each silently changes what comes out of a run, so they are the parts of
 *  the brief that must be readable — and changeable — without opening anything (policy/posts.md).
 *  Neither shows a caption; each trigger reads its own value.
 *
 *  `items-start` so the glyph stays level with the two listboxes when either field grows a hint
 *  or an error underneath it; `min-w-0` on both so a long voice or template name truncates inside
 *  its own trigger instead of pushing the glyph off a 320px screen (§8.5). */
export const EditorDockHeader = forwardRef<
  PopoverHandle,
  {
    post?: PostDraft
    ownerId: string
    voiceId: string
    templateId: string
    targetLanguage: ContentLanguage
    targetLength?: number
    tagCount: number
    onVoiceSelect: (id: string) => void
    onTemplateSelect: (id: string) => void
    onTargetLanguageSelect: (language: ContentLanguage) => void
    onBriefSaved: (values: { targetLength?: number; tagCount: number }) => void
  }
>(function EditorDockHeader(
  {
    post,
    ownerId,
    voiceId,
    templateId,
    targetLanguage,
    targetLength,
    tagCount,
    onVoiceSelect,
    onTemplateSelect,
    onTargetLanguageSelect,
    onBriefSaved,
  },
  briefRef,
) {
  // Everything the next AI run is given, in one surface. Every callback goes through the autosave
  // queue for an existing post, and through local state for a draft the server has not created.
  //
  // The ref is how 글 생성's empty state offers a way IN: with no active 작성 모델 chosen there is
  // nothing to press in the bar, and this surface is the answer.
  const briefPanel = (
    <GenerationBrief
      ref={briefRef}
      targetLanguage={targetLanguage}
      contentLanguage={post?.contentLanguage}
      frozenLanguage={
        post?.activeJob && !isTerminal(post.activeJob) ? post.activeJob.targetLanguage : undefined
      }
      onTargetLanguageSelect={onTargetLanguageSelect}
      photoCount={post?.images.length ?? 0}
      options={
        post
          ? {
              slug: post.slug,
              targetLength,
              tagCount,
              // The length and the tag count are the fields in the brief a running job has already
              // frozen; the others are still worth changing for the NEXT run, which is why only
              // these grey.
              disabled: Boolean(post.activeJob && !isTerminal(post.activeJob)),
              onSaved: onBriefSaved,
            }
          : undefined
      }
    />
  )

  // The 말투 and the 템플릿 ride the dock's own surface beside the brief's glyph, not inside it:
  // both are chosen per draft and both silently change what comes out of a run, so they are the
  // parts of the brief that must be readable — and changeable — without opening anything
  // (policy/posts.md). Neither shows a caption; each trigger reads its own value, and the labels
  // stay `sr-only` inside the two features.
  const voiceSelect = (
    <PostVoiceSelect
      ownerId={ownerId}
      value={voiceId}
      current={post?.voice}
      blocked={post ? reassignmentBlocker(post) : ''}
      confirm={Boolean(post)}
      onSelect={onVoiceSelect}
      className="min-w-0 flex-1"
    />
  )

  const templateSelect = (
    <PostTemplateSelect
      ownerId={ownerId}
      value={templateId}
      current={post?.template}
      jobRunning={Boolean(post?.activeJob && !isTerminal(post.activeJob))}
      onSelect={onTemplateSelect}
      className="min-w-0 flex-1"
    />
  )

  // `items-start` so the glyph stays level with the two listboxes when either field grows a hint
  // or an error underneath it; `min-w-0` on both so a long voice or template name truncates inside
  // its own trigger instead of pushing the glyph off a 320px screen (§8.5). The two fields share
  // the row evenly, which is also what took the voice trigger down from full width.
  const dockHeader = (
    <div className="flex items-start gap-2">
      {voiceSelect}
      {templateSelect}
      {briefPanel}
    </div>
  )

  return dockHeader
})
