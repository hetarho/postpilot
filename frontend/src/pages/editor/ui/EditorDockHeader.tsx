import { forwardRef } from 'react'
import { isTerminal } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { usePrefetchAccountQuality } from '@/entities/quality'
import { QualityRuleChoices } from '@/features/choose-quality-rules'
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
    onBriefSaved: (values: { targetLength?: number; tagCount: number }) => void
    /** The run a press was refused for, which the brief marks the missing fields of; `count`
     *  grows with every refused press. */
    refusal?: { mode: GenerationMode; count: number }
    onBriefClosed?: () => void
    /** A published post (POST-86): every post write here is off, and none of these fields names
     *  a reason — ① says it once, above its actions. */
    locked?: boolean
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
    locked = false,
  },
  briefRef,
) {
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
      locked={locked}
      options={
        post
          ? {
              slug: post.slug,
              targetLength,
              tagCount,
              // The length and the tag count are the fields in the brief a running job has already
              // frozen; the others are still worth changing for the NEXT run, which is why only
              // these grey.
              disabled: jobRunning,
              onSaved: onBriefSaved,
            }
          : undefined
      }
      // Only for a saved post: a draft has no slug to tick against (POST-81). A running job or the
      // publish lock holds the boxes, with no reason of their own — ① says it once.
      qualityRules={
        post ? (
          <QualityRuleChoices
            ownerId={ownerId}
            slug={post.slug}
            targetLanguage={targetLanguage}
            ticked={post.qualityRules}
            targetLength={targetLength}
            disabled={jobRunning || locked}
          />
        ) : undefined
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
      blocked={locked || !post ? '' : reassignmentBlocker(post)}
      confirm={Boolean(post)}
      onSelect={onVoiceSelect}
      disabled={locked}
      className="min-w-0 flex-1"
    />
  )

  const templateSelect = (
    <PostTemplateSelect
      ownerId={ownerId}
      value={templateId}
      current={post?.template}
      jobRunning={!locked && Boolean(post?.activeJob && !isTerminal(post.activeJob))}
      onSelect={onTemplateSelect}
      disabled={locked}
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
