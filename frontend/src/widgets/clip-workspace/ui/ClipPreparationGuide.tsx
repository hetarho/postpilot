import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipPreparationRow, ClipPreparationScreen } from '@/features/edit-clip-project'
import {
  Button,
  GuideProgress,
  StepBar,
  StepBarActions,
  StepBarBack,
  StepBarCta,
  StepBarReason,
  StepScreen,
  Typography,
  cardStyles,
} from '@/shared/ui'

const SCREENS = ['sources', 'story', 'style', 'review'] as const

/** Retained questions and a single shared dock. The keyed workspace owns the cursor. */
export function ClipPreparationGuide({
  screen,
  onSelect,
  screens,
  rows,
  models,
  actions,
  refusal,
  correction,
  reasonId,
}: {
  screen: ClipPreparationScreen
  onSelect: (screen: ClipPreparationScreen) => void
  screens: Record<Exclude<ClipPreparationScreen, 'review'>, ReactNode>
  rows: readonly ClipPreparationRow[]
  models: ReactNode
  actions: ReactNode
  refusal?: ReactNode
  /** Why review's actions are held: with a route or retry when the owner can fix it, or only the
   *  reason while other work finishes. */
  correction?:
    | { screen: ClipPreparationScreen; label: string }
    | { retry: () => void; label: string }
    | { label: string }
  reasonId?: string
}) {
  const { t } = useTranslation('clips')
  const ownReasonId = useId()
  const id = reasonId ?? ownReasonId
  const [shown, setShown] = useState(screen)
  const [direction, setDirection] = useState<'forward' | 'back'>('forward')
  if (shown !== screen) {
    setDirection(SCREENS.indexOf(screen) < SCREENS.indexOf(shown) ? 'back' : 'forward')
    setShown(screen)
  }
  const index = SCREENS.indexOf(screen)
  const go = (next: ClipPreparationScreen) => {
    if (next === screen) return
    onSelect(next)
    window.scrollTo(0, 0)
  }
  const back = (target: ClipPreparationScreen) => (
    <StepBarBack
      label={t('guide.backTo', { screen: t(`guide.${target}`) })}
      onClick={() => go(target)}
    />
  )
  return (
    <>
      <GuideProgress
        steps={SCREENS.map((value) => ({ value, label: t(`guide.${value}`) }))}
        current={screen}
        onSelect={go}
        ariaLabel={t('guide.aria')}
        className="mt-2"
      />
      {SCREENS.map((value, position) => (
        <StepScreen
          key={value}
          hidden={screen !== value}
          direction={direction}
          eyebrow={t('guide.position', { current: position + 1, total: SCREENS.length })}
          title={t(`guide.questions.${value}`)}
          className="mt-6 mb-8"
        >
          {value !== 'review' ? (
            <div className="mt-6 min-w-0 space-y-6">{screens[value]}</div>
          ) : (
            <>
              <dl className={cardStyles({ className: 'mt-6 min-w-0 space-y-6' })}>
                {screen === 'review' &&
                  rows.map((row) => (
                    // A group holds only its term and definitions; the grid puts the change
                    // route beside the term while it is read after the value.
                    <div
                      key={row.id}
                      className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3"
                    >
                      <Typography variant="fieldTitle" as="dt" className="min-w-0">
                        {row.label}
                      </Typography>
                      <Typography
                        variant="body"
                        as="dd"
                        className="col-span-2 mt-2 break-words whitespace-pre-wrap"
                      >
                        {row.value}
                      </Typography>
                      <dd className="col-start-2 row-start-1">
                        <Button
                          type="button"
                          variant="ghost"
                          aria-label={t('guide.change', { row: row.label })}
                          onClick={() => go(row.screen)}
                        >
                          {t('guide.changeShort')}
                        </Button>
                      </dd>
                    </div>
                  ))}
              </dl>
              {models}
            </>
          )}
        </StepScreen>
      ))}
      <StepBar ariaLabel={t('guide.actions')}>
        {screen !== 'review' && (
          <StepBarActions back={index > 0 ? back(SCREENS[index - 1]!) : undefined}>
            <StepBarCta type="button" forward onClick={() => go(SCREENS[index + 1]!)}>
              {t('guide.next')}
            </StepBarCta>
          </StepBarActions>
        )}
        {/* Approval/quote components keep their identity while another question is visible. */}
        <div hidden={screen !== 'review'}>
          {refusal}
          {correction && (
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <StepBarReason id={id}>{correction.label}</StepBarReason>
              {('screen' in correction || 'retry' in correction) && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() =>
                    'screen' in correction ? go(correction.screen) : correction.retry()
                  }
                >
                  {t('guide.fix')}
                </Button>
              )}
            </div>
          )}
          {/* Two quoted actions beside the way back can outgrow a 320px row: wrapping gives
              the committing one a row of its own rather than a label broken per syllable. */}
          <StepBarActions back={back('style')} className="flex-wrap">
            {actions}
          </StepBarActions>
        </div>
      </StepBar>
    </>
  )
}
