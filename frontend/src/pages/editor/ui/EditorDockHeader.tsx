import { useState, type ReactNode, type Ref } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { isTerminal } from '@/entities/generation-job'
import { isPublished, type GenerationOptionsSet, type PostDraft } from '@/entities/post'
import { usePrefetchAccountQuality } from '@/entities/quality'
import { CreateVoiceSheet } from '@/features/create-voice'
import { PostTemplateSelect } from '@/features/select-post-template'
import { PostVoiceSelect, reassignmentBlocker } from '@/features/select-post-voice'
import type { GenerationMode } from '@/features/generate-post'
import type { ContentLanguage } from '@/shared/api'
import type { PopoverHandle } from '@/shared/ui'
import { GenerationBrief } from '@/widgets/generation-brief'

export interface EditorChoicesProps {
  post?: PostDraft
  ownerId: string
  voiceId: string
  templateId: string
  targetLanguage: ContentLanguage
  targetLength?: number
  tagCount: number
  onVoiceSelect: (id: string) => void
  onGrowVoice: (id: string) => void
  onTemplateSelect: (id: string) => void
  onTargetLanguageSelect: (language: ContentLanguage) => void
  onBriefSaved: (set: GenerationOptionsSet) => void
  /** The run a press was refused for, which the brief marks the missing fields of; `count`
   *  grows with every refused press. */
  refusal?: { mode: GenerationMode; count: number }
  onBriefClosed?: () => void
  writingTest?: { href: string; onOpen: () => Promise<void> | void; disabled?: boolean }
  briefRef: Ref<PopoverHandle>
}

/** The three choices that decide what a run writes — 말투, 템플릿 and the writing brief — as
 *  separately placeable pieces.
 *
 *  Guide: they no longer share one dock row. The 템플릿 is chosen on the screen that asks for its
 *  answers (내용), and the 말투 and the brief are rows of the 확인 summary, each reading its own
 *  value with its own way to change it (POST-51: both stay readable without opening anything).
 *  `sheet` is the 새 말투 만들기 sheet the voice picker opens; mount it once, anywhere. */
export function useEditorChoices({
  post,
  ownerId,
  voiceId,
  templateId,
  targetLanguage,
  targetLength,
  tagCount,
  onVoiceSelect,
  onGrowVoice,
  onTemplateSelect,
  onTargetLanguageSelect,
  onBriefSaved,
  refusal,
  onBriefClosed,
  writingTest,
  briefRef,
}: EditorChoicesProps): {
  voice: ReactNode
  template: ReactNode
  brief: ReactNode
  sheet: ReactNode
} {
  // Read from the post it holds: a published post (POST-86) takes no post write here, and none of
  // these fields names a reason — ① says it once, above its actions.
  const published = post ? isPublished(post) : false
  const jobRunning = Boolean(post?.activeJob && !isTerminal(post.activeJob))
  // The picker's `새 말투 만들기` opens the directory's own create sheet (VOICE-53). Its open state
  // lives here because the picker, a feature, cannot render another feature (ARCH-13).
  const [creatingVoice, setCreatingVoice] = useState(false)
  const navigate = useNavigate()
  // Read as soon as the editor renders, so the rows are there when the brief opens; the language
  // is part of the read, since every rule text is rendered in it.
  usePrefetchAccountQuality(ownerId, post?.slug ?? '', targetLanguage)

  const brief = (
    <GenerationBrief
      ref={briefRef}
      trigger="change"
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
      writingTest={writingTest}
      // The run options, one form saved by the brief's 저장 (POST-89), only for a saved post: a
      // draft has no slug to save against.
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

  const voice = (
    <PostVoiceSelect
      ownerId={ownerId}
      value={voiceId}
      current={post?.voice}
      blocked={published || !post ? '' : reassignmentBlocker(post)}
      confirm={Boolean(post)}
      onSelect={onVoiceSelect}
      onCreateVoice={() => setCreatingVoice(true)}
      onGrowVoice={onGrowVoice}
      disabled={published}
      labelVisible
      className="min-w-0"
    />
  )

  const template = (
    <PostTemplateSelect
      ownerId={ownerId}
      value={templateId}
      current={post?.template}
      jobRunning={!published && jobRunning}
      onSelect={onTemplateSelect}
      // The wizard returns here on 저장 with the new template assigned (TMPL-32); an unsaved
      // draft has nothing typed to lose, since the first edit mints the post.
      onCreateTemplate={() =>
        void navigate({ to: '/templates/new', search: { post: post?.slug ?? 'new' } })
      }
      disabled={published}
      labelVisible
      className="min-w-0"
    />
  )

  const sheet = (
    <CreateVoiceSheet ownerId={ownerId} open={creatingVoice} onOpenChange={setCreatingVoice} />
  )

  return { voice, template, brief, sheet }
}
