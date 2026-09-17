import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { AppFailure } from '@/shared/api'
import { Button, Typography } from '@/shared/ui'
import type { ClipQuote } from '../model/types'
import { ClipFailureNotice } from './ClipFailureNotice'

/** The ONE approval surface a charged clip action gets (CLIP-19, CLIP-81): the
 *  ceiling, the writing calls it pays for, the cancellation rule beside it and
 *  the refusals that stop it. A generation and an owner-written revision are two
 *  different requests but the same promise, so neither writes its own. */
export function ClipQuoteApproval({
  quote,
  quoting,
  expired,
  balance,
  error,
  approveLabel,
  approveDisabled = false,
  onRefresh,
  onApprove,
  children,
}: {
  quote?: ClipQuote
  quoting: boolean
  expired: boolean
  /** The owner's credit balance, when it is known; absent keeps the action shut. */
  balance?: { credits: number; unlimited: boolean; renewsAt: string }
  error?: AppFailure
  approveLabel: string
  approveDisabled?: boolean
  onRefresh: () => void
  onApprove: (quote: ClipQuote) => void
  /** What this particular action adds to the ceiling — a resumed generation's
   *  reuse line, for one. */
  children?: ReactNode
}) {
  const { t } = useTranslation('clips')
  const details = useId()
  // The panel is docked, so on a phone its explanations covered the step behind it. What the
  // approval must state stays out here — the ceiling is on the action itself, the cancellation
  // rule sits beside it (CLIP-19, CLIP-81) and every refusal is in plain sight — while the
  // breakdown that repeats what those already say folds away.
  const [open, setOpen] = useState(false)
  const insufficient =
    !!quote && !!balance && !balance.unlimited && quote.maxCredits > balance.credits
  /** One line, whatever the clip is: the captions and the seconds they add when
   *  a plan says how many there are, the styles that would draw that way before
   *  one exists, and plainly none where every style is static. */
  const sequenceCaptionLine = (cost: NonNullable<ClipQuote['sequenceCaptions']>) => {
    if (cost.fromPlan)
      return cost.captions
        ? t('credits.sequenceCaptions', {
            captions: cost.captions,
            seconds: Math.max(1, Math.round(cost.addedRenderMs / 1000)),
          })
        : t('credits.sequenceNone')
    return cost.selectedStyles
      ? t('credits.sequenceStyles', { styles: cost.selectedStyles })
      : t('credits.sequenceNone')
  }
  return (
    <div className="w-full min-w-0 space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-x-3">
        {/* One live region whose TEXT changes: a region mounted only when it already holds its
            message announces nothing. */}
        <Typography variant="meta" role="status" className="min-w-0 flex-1">
          {quoting
            ? t('credits.quoting')
            : expired
              ? t('credits.expired')
              : open
                ? t('credits.maximumHelp')
                : ''}
        </Typography>
        <Button
          variant="ghost"
          className="-mr-3 shrink-0"
          aria-expanded={open}
          aria-controls={details}
          onClick={() => setOpen(!open)}
        >
          {t(open ? 'credits.detailsHide' : 'credits.detailsShow')}
        </Button>
      </div>
      <div id={details} hidden={!open} className="space-y-2">
        {quote && quote.calls.some((call) => call.label !== 'observe' && call.calls > 0) && (
          <ul className="space-y-1" aria-label={t('credits.writingCalls')}>
            {quote.calls
              .filter((call) => call.label !== 'observe' && call.calls > 0)
              .map((call) => (
                <li key={call.label}>
                  <Typography variant="meta">
                    {call.label === 'flow'
                      ? t('credits.call.flow', { calls: call.calls })
                      : t('credits.call.narration', { calls: call.calls })}
                  </Typography>
                </li>
              ))}
          </ul>
        )}
        {children}
        {balance?.unlimited && <Typography variant="body">{t('credits.exempt')}</Typography>}
      </div>
      {/* What the sequence-rendered captions add to the render this approval
          leads to (CDS-81). It states work, never a limit: nothing is refused
          for the count (CLIP-145) and the render itself is credit-free
          (CLIP-20). */}
      {quote?.sequenceCaptions && (
        <Typography variant="meta" className="text-content-secondary block">
          {sequenceCaptionLine(quote.sequenceCaptions)}
        </Typography>
      )}
      <Typography variant="body">
        {t(quote?.cancellationPolicy ? 'cancellation.rule' : 'cancellation.policyUnavailable')}
      </Typography>
      {error && <ClipFailureNotice failure={error} />}
      {insufficient && balance && quote && (
        <ClipFailureNotice
          failure={{
            reason: 'INSUFFICIENT_CREDITS',
            params: {
              required: String(quote.maxCredits),
              balance: String(balance.credits),
              renews_at: balance.renewsAt,
            },
          }}
        />
      )}
      {(expired || !!error) && (
        <Button variant="secondary" pending={quoting} onClick={onRefresh}>
          {t('credits.refresh')}
        </Button>
      )}
      <Button
        variant="cta"
        className="w-full whitespace-normal"
        disabled={!quote?.cancellationPolicy || approveDisabled || insufficient}
        pending={quoting}
        onClick={() => {
          if (quote?.cancellationPolicy && !insufficient) onApprove(quote)
        }}
      >
        {approveLabel}
      </Button>
    </div>
  )
}
