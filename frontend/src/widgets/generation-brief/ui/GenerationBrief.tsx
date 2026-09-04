import { forwardRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Settings } from 'lucide-react'
import type { ContentLanguage } from '@/shared/api'
import { CandidatePairSelect } from '@/features/configure-model-pair'
import { GenerationOptions } from '@/features/generate-post'
import { StageModelSelect } from '@/features/select-model'
import { PostLanguageSelect } from '@/features/select-post-language'
import { Popover, Typography, type PopoverHandle } from '@/shared/ui'

interface GenerationBriefProps {
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

/** Everything the next AI run is given that is a SETTING rather than a per-draft decision:
 *  관찰 모델 · 작성 모델 · 작성 A/B 후보 · 글 언어 · 목표 분량.
 *
 *  It is a WIDGET because it composes four different `features/*` slices and a feature may not
 *  import a sibling feature (ARCHITECTURE §3). Every callback is supplied by `pages/editor`, so
 *  each assignment still rides the draft autosave queue that lives above the step panels: an
 *  assignment made here cannot be lost to a step change any more than a title edit can.
 *
 *  The 말투 and the 용도 are NOT in here. Both are chosen per draft and both silently change what
 *  comes out of a run, so they ride the dock's own surface beside this trigger — visible without
 *  opening anything. The 글 언어 stays behind it: it is set once and then followed by every run,
 *  and a third dropdown on the dock row would leave none of them readable (owner decision
 *  2026-09-02). */
export const GenerationBrief = forwardRef<PopoverHandle, GenerationBriefProps>(
  function GenerationBrief(
    {
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
        // exists to produce (owner decision, 2026-09-01). The 말투 and 용도 it used to name now sit
        // beside this glyph as their own dropdowns, so the two expensive-to-get-wrong fields are
        // visible WITHOUT opening anything, which is what naming them in the trigger was for.
        triggerSize="icon"
        triggerLabel={<Settings aria-hidden="true" className="size-5" />}
        align="end"
        phone="sheet"
        className="shrink-0"
      >
        {(close) => (
          // `grid-cols-1` and not a bare `grid`: an IMPLICIT column is `auto`, which is floored at
          // the widest child's min-content and therefore grows the brief past the 288px panel it
          // opens in — that is what put a horizontal scrollbar under the surface. An explicit
          // `minmax(0, 1fr)` track plus `min-w-0` on each row lets every field shrink to the panel
          // instead, which is what the fields are already built to do: each one truncates or wraps
          // inside its own well.
          <div className="grid grid-cols-1 gap-4 *:min-w-0">
            <div>
              <StageModelSelect stage="observe" optional={photoCount === 0} />
              {photoCount === 0 && (
                <Typography variant="body" as="p" className="text-content-secondary mt-1">
                  {t('editor.noPhotoModel', { ns: 'posts' })}
                </Typography>
              )}
            </div>
            <StageModelSelect stage="write" />
            {/* Directly under the model the ordinary run uses, because that is the comparison the
                A/B pair is: the same step, run twice. The link to the AI 모델 page this replaced
                asked the user to leave the draft to make a two-dropdown choice. */}
            <CandidatePairSelect stage="write" />
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
          </div>
        )}
      </Popover>
    )
  },
)
