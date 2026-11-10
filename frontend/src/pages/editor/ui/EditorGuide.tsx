import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { ActionBar, Button, GuideProgress, StepScreen } from '@/shared/ui'
import {
  GUIDE_STEPS,
  guideIndex,
  guideSteps,
  nextGuideStep,
  previousGuideStep,
  type GuideStep,
} from '../model/guide'

/** Guide: 글 생성 as a guided sequence — a slim progress at the top, ONE question per screen, and
 *  the bottom bar in the thumb zone holding 이전 and the step's one big action: 다음 on the first
 *  three screens, 생성 on the summary.
 *
 *  The screens are PANELS, not routes, for the same reason the lifecycle steps are: the page owns
 *  the post's fields and their autosave, so a screen change can never strand a queued save. */
export function EditorGuide({
  step,
  onStepChange,
  screens,
  actions,
  notice,
}: {
  step: GuideStep
  onStepChange: (step: GuideStep) => void
  /** Each screen's content, under the question the screen asks. */
  screens: Record<GuideStep, ReactNode>
  /** The summary's committing row — 생성 — given the way back to lead it. Absent while the draft
   *  has no post to start a run on, in which case the summary's bar holds 이전 alone. */
  actions?: (back: ReactNode) => ReactNode
  /** What the last run has to say (a failure and its retry), above the bar's buttons. */
  notice?: ReactNode
}) {
  const { t } = useTranslation('posts')
  // Which way the owner is moving, so a step back slides in from the left — derived from the step
  // last drawn, adjusted during render (React's "state from a changed prop" pattern).
  const [shown, setShown] = useState(step)
  const [direction, setDirection] = useState<'forward' | 'back'>('forward')
  if (shown !== step) {
    setDirection(guideIndex(step) < guideIndex(shown) ? 'back' : 'forward')
    setShown(step)
  }
  const previous = previousGuideStep(step)
  const next = nextGuideStep(step)
  const go = (target: GuideStep) => {
    onStepChange(target)
    window.scrollTo(0, 0)
  }
  const backTo = (target: GuideStep | undefined) =>
    target ? (
      <Button
        variant="secondary"
        size="icon"
        aria-label={t('guide.back')}
        onClick={() => go(target)}
      >
        <ArrowLeft aria-hidden="true" className="size-5" />
      </Button>
    ) : undefined
  const back = backTo(previous)
  const summary = GUIDE_STEPS[GUIDE_STEPS.length - 1]
  return (
    <>
      <GuideProgress
        steps={guideSteps()}
        current={step}
        onSelect={go}
        ariaLabel={t('guide.aria')}
        className="mt-2"
      />
      {/* Every screen stays mounted and only the current one is shown: an upload started on 사진
          keeps reporting, and an answer half-typed on 내용 keeps its caret, while the owner looks
          at another screen. Showing a screen again replays its entrance. */}
      {GUIDE_STEPS.map((value) => (
        <StepScreen
          key={value}
          hidden={value !== step}
          direction={direction}
          eyebrow={t('guide.position', {
            current: guideIndex(value) + 1,
            total: GUIDE_STEPS.length,
          })}
          title={t(`guide.questions.${value}`)}
          className="mt-6"
        >
          {screens[value]}
        </StepScreen>
      ))}
      <ActionBar dock="step" ariaLabel={t('editor.actionAria')}>
        <div className="flex min-w-0 flex-col gap-3">
          {notice}
          {next && (
            <div className="flex items-center gap-2">
              {back}
              <Button variant="cta" size="hero" className="min-w-0 flex-1" onClick={() => go(next)}>
                {t('guide.next')}
                <ArrowRight aria-hidden="true" className="size-5" />
              </Button>
            </div>
          )}
          {/* The summary's row stays MOUNTED on the other screens, hidden: a 생성 that is flushing
              the draft or asking about re-observation keeps its state while the owner steps back
              to fix an answer. */}
          {actions ? (
            <div hidden={step !== summary}>{actions(backTo(previousGuideStep(summary)))}</div>
          ) : (
            !next && back && <div className="flex items-center gap-2">{back}</div>
          )}
        </div>
      </ActionBar>
    </>
  )
}
