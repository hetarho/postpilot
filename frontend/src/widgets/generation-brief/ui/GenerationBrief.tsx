import { forwardRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { Settings } from 'lucide-react'
import type { PurposeRef } from '@/entities/purpose'
import type { ContentLanguage } from '@/shared/api'
import { GenerationOptions } from '@/features/generate-post'
import { StageModelSelect } from '@/features/select-model'
import { PostLanguageSelect } from '@/features/select-post-language'
import { PostPurposeSelect } from '@/features/select-post-purpose'
import { Popover, Typography, typographyStyles, type PopoverHandle } from '@/shared/ui'

interface GenerationBriefProps {
  ownerId: string
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

/** Everything the next AI run is given, in one surface: 관찰 모델 · 작성 모델 · 용도 · 목표 언어 ·
 *  목표 분량, plus the way to the A/B candidate pair.
 *
 *  It is a WIDGET because it composes four different `features/*` slices and a feature may not
 *  import a sibling feature (ARCHITECTURE §3). Every callback is supplied by `pages/editor`, so
 *  each assignment still rides the draft autosave queue that lives above the step panels: an
 *  assignment made here cannot be lost to a step change any more than a title edit can.
 *
 *  The 말투 is NOT in here. A wrong voice silently ruins a draft, so it is the one field of the
 *  brief that stays on the dock's own surface, beside this trigger — visible without opening
 *  anything. Everything else is a setting you check once and forget. */
export const GenerationBrief = forwardRef<PopoverHandle, GenerationBriefProps>(
  function GenerationBrief(
    {
      ownerId,
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
    },
    ref,
  ) {
    const { t } = useTranslation(['posts', 'common'])
    const label = t('generation.brief.title', { ns: 'posts' })

    return (
      <Popover
        ref={ref}
        label={label}
        // A 44px glyph pinned to the dock's top-right instead of a full-width labelled bar: the
        // brief is what you set BEFORE writing, and on the phone it was standing over the draft it
        // exists to produce (owner decision, 2026-09-01). The 말투 it used to name now sits beside
        // this glyph as its own dropdown, so the expensive-to-get-wrong field is visible WITHOUT
        // opening anything, which is what naming it in the trigger was for.
        triggerSize="icon"
        triggerLabel={<Settings aria-hidden="true" className="size-5" />}
        align="end"
        phone="sheet"
        className="shrink-0"
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
  },
)
