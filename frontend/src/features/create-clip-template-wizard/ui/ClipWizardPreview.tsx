import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import { CLIP_CAPTION_STYLES } from '@/entities/clip-design'
import { CompositionPreview, type ClipComposition, type ClipRecipe } from '@/entities/clip-template'
import { Checkbox, Typography } from '@/shared/ui'
import { checkedStages, type StageDraft } from '../model/wizard-machine'

type Design = Pick<ClipRecipe, 'introPreset' | 'outroPreset' | 'allowedCaptionStyles'>

/** The clip the wizard's answers make (CLIP-221). Stages are not drawn on the clip itself
 *  (CLIP-141), so the checked stages stand in order above the template preview, the one whose
 *  step is on screen marked. The preview draws the draft's design selection; only the design
 *  step hands in `onDesignChange`, which offers the presets and caption styles on it. */
export function ClipWizardPreview({
  stages,
  currentId,
  document,
  design,
  onDesignChange,
}: {
  stages: readonly StageDraft[]
  currentId?: string
  document: ClipComposition
  design: Design
  onDesignChange?: (patch: Partial<Design>) => void
}) {
  const { t } = useTranslation(['clipWizard', 'clips'])
  const shown = checkedStages({ stages })
  const current = useRef<HTMLLIElement>(null)
  useEffect(() => {
    if (current.current) revealInOwnScroller(current.current)
  }, [currentId])
  return (
    <div className="min-w-0 space-y-6">
      <section aria-label={t('preview.stages', { ns: 'clipWizard' })}>
        <Typography variant="eyebrow" as="h3" className="mb-2">
          {t('preview.stages', { ns: 'clipWizard' })}
        </Typography>
        {shown.length === 0 ? (
          <Typography variant="body" as="p" className="text-content-secondary">
            {t('preview.empty', { ns: 'clipWizard' })}
          </Typography>
        ) : (
          <ol className="space-y-1">
            {shown.map((stage, index) => {
              const here = stage.id === currentId
              return (
                <li
                  key={stage.id}
                  ref={here ? current : undefined}
                  aria-current={here ? 'step' : undefined}
                  className={twMerge(
                    'duration-base ease-standard flex items-baseline gap-2 rounded-xl px-2 py-1.5 transition-shadow',
                    here && 'ring-stroke-accent ring-2',
                  )}
                >
                  <Typography variant="meta" as="span" className="tabular-nums">
                    {index + 1}
                  </Typography>
                  <Typography variant="body" as="span" className="min-w-0 break-words">
                    {stage.heading.trim() || stage.name}
                  </Typography>
                </li>
              )
            })}
          </ol>
        )}
      </section>
      <CompositionPreview
        document={document}
        presets={{ intro: design.introPreset, outro: design.outroPreset }}
        captionStyles={design.allowedCaptionStyles}
        onPresetsChange={
          onDesignChange &&
          (({ intro, outro }) => onDesignChange({ introPreset: intro, outroPreset: outro }))
        }
      >
        {onDesignChange && (
          <div
            role="group"
            aria-label={t('project.captionStyles', { ns: 'clips' })}
            className="grid gap-2 sm:grid-cols-2"
          >
            {CLIP_CAPTION_STYLES.map((style) => (
              <label key={style} className="flex min-h-11 items-center gap-3">
                <Checkbox
                  checked={design.allowedCaptionStyles.includes(style)}
                  onChange={(event) =>
                    onDesignChange({
                      allowedCaptionStyles: event.target.checked
                        ? CLIP_CAPTION_STYLES.filter(
                            (entry) =>
                              entry === style || design.allowedCaptionStyles.includes(entry),
                          )
                        : design.allowedCaptionStyles.filter((entry) => entry !== style),
                    })
                  }
                />
                <Typography variant="body" as="span">
                  {t(`captionStyles.${style}`, { ns: 'clips' })}
                </Typography>
              </label>
            ))}
          </div>
        )}
      </CompositionPreview>
    </div>
  )
}

/** Scrolls the nearest scrolling ancestor — the side column or the sheet's body — just enough to
 *  show `element`. Not `scrollIntoView`: that also scrolls the window, which would move the step
 *  the owner is answering out from under them. */
function revealInOwnScroller(element: HTMLElement) {
  let scroller = element.parentElement
  while (scroller && scroller !== document.body) {
    const overflow = getComputedStyle(scroller).overflowY
    if (
      (overflow === 'auto' || overflow === 'scroll') &&
      scroller.scrollHeight > scroller.clientHeight
    )
      break
    scroller = scroller.parentElement
  }
  if (!scroller || scroller === document.body) return
  const box = scroller.getBoundingClientRect()
  const target = element.getBoundingClientRect()
  if (target.top < box.top) scroller.scrollTop -= box.top - target.top
  else if (target.bottom > box.bottom)
    scroller.scrollTop += Math.min(target.bottom - box.bottom, target.top - box.top)
}
