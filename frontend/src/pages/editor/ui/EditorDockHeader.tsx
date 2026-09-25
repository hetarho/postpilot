import { forwardRef } from 'react'
import { isTerminal } from '@/entities/generation-job'
import { isPublished, type GenerationOptionsSet, type PostDraft } from '@/entities/post'
import { usePrefetchAccountQuality } from '@/entities/quality'
import { PostTemplateSelect } from '@/features/select-post-template'
import { PostVoiceSelect, reassignmentBlocker } from '@/features/select-post-voice'
import type { GenerationMode } from '@/features/generate-post'
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
    onBriefSaved: (set: GenerationOptionsSet) => void
    /** The run a press was refused for, which the brief marks the missing fields of; `count`
     *  grows with every refused press. */
    refusal?: { mode: GenerationMode; count: number }
    onBriefClosed?: () => void
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
    refusal,
    onBriefClosed,
  },
  briefRef,
) {
  // Read from the post it holds: a published post (POST-86) takes no post write here, and none of
  // these fields names a reason — ① says it once, above its actions.
  const published = post ? isPublished(post) : false
  const jobRunning = Boolean(post?.activeJob && !isTerminal(post.activeJob))
  // Read as soon as the dock renders, so the rows are there when the brief opens; the language is
  // part of the read, since every rule text is rendered in it.
  usePrefetchAccountQuality(ownerId, post?.slug ?? '', targetLanguage)

  // Everything the next AI run is given, in one surface. Every callback goes through the autosave
  // queue for an existing post, and through local state for a draft the server has not created.
  //
  // The ref is how 글 생성 opens it for a press its setup refused, with `refusal` naming the run
  // whose missing fields it marks.
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
      videoCount={post?.videos.length ?? 0}
      refusal={refusal}
      onClose={onBriefClosed}
      locked={published}
      // The run options, one form saved by the brief's 저장 (POST-89), only for a saved post: a
      // draft has no slug to save against. The two numbers come from the brief's mirror, which is
      // also what 생성 sends; the other three from the post. A running job holds the numbers and
      // the ticks, which it has already frozen; 분야 and 기억 사용 are read at the next enqueue.
      options={
        post
          ? {
              ownerId,
              slug: post.slug,
              saved: {
                targetLength,
                tagCount,
                useMemory: post.useMemory,
                qualityRules: post.qualityRules,
                field: post.field,
              },
              jobRunning,
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
      blocked={published || !post ? '' : reassignmentBlocker(post)}
      confirm={Boolean(post)}
      onSelect={onVoiceSelect}
      disabled={published}
      className="min-w-0 flex-1"
    />
  )

  const templateSelect = (
    <PostTemplateSelect
      ownerId={ownerId}
      value={templateId}
      current={post?.template}
      jobRunning={!published && Boolean(post?.activeJob && !isTerminal(post.activeJob))}
      onSelect={onTemplateSelect}
      disabled={published}
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
