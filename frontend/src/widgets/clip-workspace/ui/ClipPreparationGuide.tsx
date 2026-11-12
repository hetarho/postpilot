import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { RequirementsTracker, type RequirementsCoverage } from '@/entities/advertiser-guide'
import {
  clipPreparationScreens,
  type ClipPreparationRow,
  type ClipPreparationScreen,
} from '@/features/edit-clip-project'
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
  campaign,
  requirements,
  noteEmpty,
}: {
  screen: ClipPreparationScreen
  onSelect: (screen: ClipPreparationScreen) => void
  screens: Record<Exclude<ClipPreparationScreen, 'review'>, ReactNode>
  /** 체험단으로 다녀왔어요 adds 요청 and the tracker on every screen after it (CLIP-228). */
  campaign: boolean
  requirements?: { coverage: RequirementsCoverage; hashtags: readonly string[] }
  /** 덧붙임's 다음 reads 건너뛰고 확인 while the instruction is empty (CLIP-225). */
  noteEmpty: boolean
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
  const SCREENS = clipPreparationScreens(campaign)
  // 요청 leaves with 체험단 글: a cursor still on it reads as the screen that took its place.
  const current = SCREENS.includes(screen) ? screen : 'sources'
  const [shown, setShown] = useState(current)
  const [direction, setDirection] = useState<'forward' | 'back'>('forward')
  if (shown !== current) {
    setDirection(SCREENS.indexOf(current) < SCREENS.indexOf(shown) ? 'back' : 'forward')
    setShown(current)
  }
  const index = SCREENS.indexOf(current)
  const go = (next: ClipPreparationScreen, focusInvalid = false) => {
    if (next === current) return focusInvalid ? focusField(next) : undefined
    onSelect(next)
    window.scrollTo(0, 0)
    if (focusInvalid) focusField(next)
  }
  // A required answer missing at run start brings 내용 back with that field focused (CLIP-225).
  const focusField = (target: ClipPreparationScreen) =>
    window.requestAnimationFrame(() =>
      document
        .querySelector<HTMLElement>(
          `[data-clip-screen="${target}"] [aria-invalid="true"]:not([disabled])`,
        )
        ?.focus(),
    )
  const tracked = campaign && requirements && index > SCREENS.indexOf('requirements')
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
        current={current}
        onSelect={(next) => go(next)}
        ariaLabel={t('guide.aria')}
        className="mt-2"
      />
      {tracked && (
        <RequirementsTracker
          coverage={requirements.coverage}
          list={current === 'content' || current === 'note'}
          onEdit={() => go('requirements')}
          notes={current === 'review' && <TrackerNotes requirements={requirements} />}
        />
      )}
      {SCREENS.map((value, position) => (
        <StepScreen
          key={value}
          hidden={current !== value}
          direction={direction}
          eyebrow={t('guide.position', { current: position + 1, total: SCREENS.length })}
          title={t(`guide.questions.${value}`)}
          className="mt-6 mb-8"
        >
          {value !== 'review' ? (
            <div className="mt-6 min-w-0 space-y-6" data-clip-screen={value}>
              {screens[value]}
            </div>
          ) : (
            <>
              <dl className={cardStyles({ className: 'mt-6 min-w-0 space-y-6' })}>
                {current === 'review' &&
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
        {current !== 'review' && (
          <StepBarActions back={index > 0 ? back(SCREENS[index - 1]!) : undefined}>
            <StepBarCta type="button" forward onClick={() => go(SCREENS[index + 1]!)}>
              {current !== 'note'
                ? t('guide.next')
                : noteEmpty
                  ? t('guide.skipNote')
                  : t('guide.toReview')}
            </StepBarCta>
          </StepBarActions>
        )}
        {/* Approval/quote components keep their identity while another question is visible. */}
        <div hidden={current !== 'review'}>
          {refusal}
          {correction && (
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <StepBarReason id={id}>{correction.label}</StepBarReason>
              {('screen' in correction || 'retry' in correction) && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() =>
                    'screen' in correction ? go(correction.screen, true) : correction.retry()
                  }
                >
                  {t('guide.fix')}
                </Button>
              )}
            </div>
          )}
          {/* Two quoted actions beside the way back can outgrow a 320px row: wrapping gives
              the committing one a row of its own rather than a label broken per syllable. */}
          <StepBarActions back={back('note')} className="flex-wrap">
            {actions}
          </StepBarActions>
        </div>
      </StepBar>
    </>
  )
}

/** 확인's own lines under the tracker: what is still missing, and what ③ checks (CLIP-228). */
function TrackerNotes({
  requirements,
}: {
  requirements: { coverage: RequirementsCoverage; hashtags: readonly string[] }
}) {
  const { t } = useTranslation('clips')
  const missing = requirements.coverage.keywords.filter((item) => !item.held)
  return (
    <>
      <Typography variant="meta" as="p">
        {t('guide.trackerRemaining', {
          keywords: missing.map((item) => item.keyword).join(', ') || t('guide.trackerNone'),
        })}
      </Typography>
      {requirements.hashtags.length > 0 && (
        <Typography variant="meta" as="p" className="break-words">
          {t('guide.trackerTags', {
            tags: requirements.hashtags.map((tag) => `#${tag}`).join(' '),
          })}
        </Typography>
      )}
      <Typography variant="meta" as="p">
        {t('guide.trackerAdvisory')}
      </Typography>
    </>
  )
}
