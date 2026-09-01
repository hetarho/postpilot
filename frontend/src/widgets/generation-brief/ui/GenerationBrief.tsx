import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import type { PurposeRef } from '@/entities/purpose'
import type { VoiceRef } from '@/entities/voice'
import type { ContentLanguage } from '@/shared/api'
import { GenerationOptions } from '@/features/generate-post'
import { StageModelSelect } from '@/features/select-model'
import { PostLanguageSelect } from '@/features/select-post-language'
import { PostPurposeSelect } from '@/features/select-post-purpose'
import { PostVoiceSelect } from '@/features/select-post-voice'
import { Popover, Typography, typographyStyles } from '@/shared/ui'

interface GenerationBriefProps {
  ownerId: string
  /** The post's voice, as the editor shows it. */
  voiceId: string
  currentVoice?: VoiceRef
  /** Why the voice cannot be reassigned right now, or ''. */
  voiceBlocked?: string
  /** Confirm before reassigning. False only for a draft the server has not created yet. */
  confirmVoiceChange: boolean
  onVoiceSelect: (voiceId: string) => Promise<void> | void
  purposeId: string
  currentPurpose?: PurposeRef
  jobRunning?: boolean
  onPurposeSelect: (purposeId: string) => Promise<void> | void
  targetLanguage: ContentLanguage
  contentLanguage?: ContentLanguage
  frozenLanguage?: ContentLanguage
  onTargetLanguageSelect: (language: ContentLanguage) => Promise<void> | void
  /** Decides whether the observe model is optional — a post with no photo never observes. */
  photoCount: number
  /** Absent for a draft with no post yet: a target length has no slug to save against. */
  targetLength?: {
    slug: string
    value?: number
    disabled: boolean
    onSaved: (value?: number) => void
  }
}

/** Everything the next AI run is given, in one surface: 관찰 모델 · 작성 모델 · 말투 · 용도 ·
 *  목표 언어 · 목표 분량, plus the way to the A/B candidate pair.
 *
 *  It is a WIDGET because it composes five different `features/*` slices and a feature may not
 *  import a sibling feature (ARCHITECTURE §3). Every callback is supplied by `pages/editor`, so
 *  each assignment still rides the draft autosave queue that lives above the step panels — a
 *  reassignment made here cannot be lost to a step change any more than a title edit can.
 *
 *  The TRIGGER NAMES THE VOICE. A wrong voice silently ruins a draft, and the one thing a closed
 *  surface must still report is the thing that is expensive to get wrong. */
export function GenerationBrief({
  ownerId,
  voiceId,
  currentVoice,
  voiceBlocked = '',
  confirmVoiceChange,
  onVoiceSelect,
  purposeId,
  currentPurpose,
  jobRunning = false,
  onPurposeSelect,
  targetLanguage,
  contentLanguage,
  frozenLanguage,
  onTargetLanguageSelect,
  photoCount,
  targetLength,
}: GenerationBriefProps) {
  const { t } = useTranslation(['posts', 'common'])
  const voiceName = currentVoice?.name?.trim() ?? ''
  const label = voiceName
    ? t('generation.brief.trigger', { ns: 'posts', voice: voiceName })
    : t('generation.brief.title', { ns: 'posts' })

  return (
    <Popover
      label={label}
      // `min-w-0` on both the button and its label: without it the flex child's automatic minimum
      // is the voice name's min-content width, and a long name pushes the dock's row past 320px
      // instead of truncating (§8.5).
      triggerClassName="min-w-0"
      triggerLabel={<span className="min-w-0 truncate">{label}</span>}
      align="end"
      phone="sheet"
      className="min-w-0 flex-1 sm:flex-none"
    >
      {(close) => (
        <div className="grid gap-4">
          <div>
            <StageModelSelect stage="observe" optional={photoCount === 0} />
            {photoCount === 0 && (
              <Typography variant="body" as="p" className="text-content-secondary mt-1">
                {t('editor.noPhotoModel', { ns: 'posts' })}
              </Typography>
            )}
          </div>
          <StageModelSelect stage="write" />
          <PostVoiceSelect
            ownerId={ownerId}
            value={voiceId}
            current={currentVoice}
            blocked={voiceBlocked}
            confirm={confirmVoiceChange}
            onSelect={onVoiceSelect}
          />
          <PostPurposeSelect
            ownerId={ownerId}
            value={purposeId}
            current={currentPurpose}
            jobRunning={jobRunning}
            onSelect={onPurposeSelect}
          />
          <PostLanguageSelect
            value={targetLanguage}
            contentLanguage={contentLanguage}
            frozenLanguage={frozenLanguage}
            onSelect={onTargetLanguageSelect}
          />
          {targetLength && (
            <div>
              <Typography variant="label" as="p">
                {t('generation.brief.length', { ns: 'posts' })}
              </Typography>
              <GenerationOptions
                key={`${targetLength.slug}-${targetLength.value ?? 'natural'}`}
                slug={targetLength.slug}
                targetLength={targetLength.value}
                disabled={targetLength.disabled}
                onSaved={targetLength.onSaved}
                onClose={close}
              />
            </div>
          )}
          <div>
            <Typography variant="label" as="p">
              {t('editor.writeCandidates', { ns: 'posts' })}
            </Typography>
            <Link
              to="/ai-models"
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover mt-1 inline-flex min-h-11 items-center underline',
              })}
            >
              {t('editor.configureCandidates', { ns: 'posts' })}
            </Link>
          </div>
        </div>
      )}
    </Popover>
  )
}
