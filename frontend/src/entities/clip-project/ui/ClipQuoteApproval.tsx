import type { ReactNode } from 'react'
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
  const insufficient =
    !!quote && !!balance && !balance.unlimited && quote.maxCredits > balance.credits
  return (
    <div className="w-full min-w-0 space-y-2">
      <Typography variant="meta" role="status">
        {quoting ? t('credits.quoting') : expired ? t('credits.expired') : t('credits.maximumHelp')}
      </Typography>
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
      <Typography variant="body">
        {t(quote?.cancellationPolicy ? 'cancellation.rule' : 'cancellation.policyUnavailable')}
      </Typography>
      {balance?.unlimited && <Typography variant="body">{t('credits.exempt')}</Typography>}
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
