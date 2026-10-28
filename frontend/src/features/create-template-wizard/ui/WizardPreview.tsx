import { useEffect, useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import { Sparkles } from 'lucide-react'
import {
  AnswerStandIn,
  FixedTextStandIn,
  PhotoStandIn,
  PREVIEW_REPEAT_PASSES,
  PreviewLines,
  PreviewPostCard,
  RepeatStandIn,
  WriteStandIn,
} from '@/entities/template'
import { Typography } from '@/shared/ui'
import { checkedSections, type SectionDraft } from '../model/wizard-machine'

/** The post the wizard's answers make (TMPL-75), drawn from the sections themselves rather than
 *  from the built body: the wizard knows which line is a 소제목, so the preview can show it as one
 *  and mark the section whose step is on screen. It mirrors `toWizardBody` — a repeating section
 *  repeats its content and photos under its heading — and uses the stand-ins every template
 *  preview uses (TMPL-66). The title area stays empty, so the title is the AI's (TMPL-52). */
export function WizardPreview({
  sections,
  currentId,
  titled = true,
  className,
}: {
  sections: readonly SectionDraft[]
  currentId?: string
  /** Off where a surrounding sheet already says 미리보기; the article keeps its name either way. */
  titled?: boolean
  className?: string
}) {
  const { t } = useTranslation('templates')
  const shown = checkedSections({ sections })
  const current = useRef<HTMLElement>(null)
  useEffect(() => {
    if (current.current) revealInOwnScroller(current.current)
  }, [currentId])
  return (
    <article aria-label={t('preview.heading')} className={twMerge('min-w-0', className)}>
      {titled && (
        <Typography variant="eyebrow" as="p" className="mb-1">
          {t('preview.heading')}
        </Typography>
      )}
      <Typography variant="meta" as="p">
        {t('preview.note')}
      </Typography>
      <PreviewPostCard className="mt-4">
        <div className="space-y-2">
          <Typography variant="meta" as="p" className="flex items-center gap-1.5">
            <Sparkles aria-hidden className="size-3.5" />
            {t('wizard.preview.title')}
          </Typography>
          <div aria-hidden className="bg-surface-recessed h-3.5 w-4/5 rounded-full" />
        </div>
        {shown.length === 0 ? (
          <Typography variant="body" as="p" className="text-content-secondary mt-6">
            {t('wizard.preview.empty')}
          </Typography>
        ) : (
          <div className="mt-5 space-y-3">
            {shown.map((section) => {
              const here = section.id === currentId
              return (
                <section
                  key={section.id}
                  ref={here ? current : undefined}
                  aria-current={here ? 'step' : undefined}
                  className={twMerge(
                    'duration-base ease-standard -mx-2 space-y-3 rounded-xl px-2 py-2 transition-shadow',
                    here && 'ring-stroke-accent ring-2',
                  )}
                >
                  <Typography variant="fieldTitle" as="h4" className="break-words">
                    {section.heading.trim() || section.name}
                  </Typography>
                  <SectionStandIns section={section} />
                </section>
              )
            })}
          </div>
        )}
      </PreviewPostCard>
    </article>
  )
}

function SectionStandIns({ section }: { section: SectionDraft }) {
  const { t } = useTranslation('templates')
  const content = ((): ReactNode => {
    switch (section.content) {
      case 'write':
        return <WriteStandIn topic={section.help.trim() || section.name} />
      case 'ask':
        return <AnswerStandIn label={section.question.trim() || section.name} topic="" />
      case 'literal':
        return section.literal.trim() !== '' ? (
          <FixedTextStandIn text={section.literal.trim()} />
        ) : (
          <div className="space-y-1.5">
            <Typography variant="meta" as="p">
              {t('wizard.preview.literalEmpty')}
            </Typography>
            <PreviewLines lines={1} />
          </div>
        )
    }
  })()
  const photos = section.photos > 0 ? <PhotoStandIn count={section.photos} /> : null
  // A 데이터 받기 field never repeats: the grammar refuses one inside a repeat (to-body.ts).
  if (section.repeat && photos && section.content !== 'ask')
    return (
      <RepeatStandIn>
        {Array.from({ length: PREVIEW_REPEAT_PASSES }, (_, pass) => (
          <div key={pass} className="space-y-3">
            {content}
            {photos}
          </div>
        ))}
      </RepeatStandIn>
    )
  return (
    <>
      {content}
      {photos}
    </>
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
