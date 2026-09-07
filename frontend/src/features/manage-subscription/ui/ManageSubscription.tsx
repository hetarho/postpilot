import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { planLabel, type PlanName } from '@/entities/plan'
import {
  useCancelScheduledChange,
  useCancelSubscription,
  useChangeSubscription,
  useQuoteChange,
  useResumeSubscription,
  type BillingTerm,
  type Subscription,
} from '@/entities/subscription'
import { formatDate } from '@/shared/lib'
import { Button, Dialog, FieldMessage, Notice, Typography } from '@/shared/ui'

export function ScheduledChangeButton({
  plan,
  term,
  label,
}: {
  plan: PlanName
  term: BillingTerm
  label: string
}) {
  const { t } = useTranslation('billing')
  const [open, setOpen] = useState(false)
  const change = useChangeSubscription()
  const { quote, isPending, isError } = useQuoteChange(
    open ? plan : undefined,
    open ? term : undefined,
  )

  const confirm = async () => {
    try {
      await change.changeSubscription(plan, term)
      setOpen(false)
    } catch {
      // The mutation's catalog-backed error remains in the dialog.
    }
  }

  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        {label}
      </Button>
      <Dialog
        open={open}
        title={t('change.scheduleTitle')}
        confirmLabel={t('change.scheduleConfirm')}
        pending={change.isPending || isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void confirm()}
      >
        <span className="grid gap-2">
          <span>
            {t('change.scheduleDescription', {
              plan: planLabel(plan),
              term: t(`subscription.term.${term}`),
              date: quote ? formatDate(quote.effectiveAt) : '—',
            })}
          </span>
          <span>{t('change.noChargeNow')}</span>
          {isError && <FieldMessage>{t('change.quoteFailed')}</FieldMessage>}
          {change.errorMessage && <FieldMessage>{change.errorMessage}</FieldMessage>}
        </span>
      </Dialog>
    </>
  )
}

export function BillingSubscriptionActions({ subscription }: { subscription: Subscription }) {
  const { t } = useTranslation('billing')
  const [cancelling, setCancelling] = useState(false)
  const cancelSchedule = useCancelScheduledChange()
  const cancel = useCancelSubscription()
  const resume = useResumeSubscription()
  const nextTerm: BillingTerm = subscription.term === 'annual' ? 'monthly' : 'annual'

  if (subscription.status !== 'active') return null

  const confirmCancel = async () => {
    try {
      await cancel.cancelSubscription()
      setCancelling(false)
    } catch {
      // The mutation's catalog-backed refusal renders below the actions.
    }
  }

  const clearScheduled = async () => {
    try {
      await cancelSchedule.cancelScheduledChange()
    } catch {
      // The catalog-backed refusal renders below the actions.
    }
  }

  const resumeRenewal = async () => {
    try {
      await resume.resumeSubscription()
    } catch {
      // The catalog-backed refusal renders below the actions.
    }
  }

  return (
    <div className="mt-4 grid gap-3">
      {subscription.scheduledPlan && subscription.scheduledTerm ? (
        <Notice tone="info" role="status">
          <span className="grid gap-2">
            <Typography variant="label" as="span">
              {t('change.scheduled', {
                plan: planLabel(subscription.scheduledPlan),
                term: t(`subscription.term.${subscription.scheduledTerm}`),
                date: formatDate(subscription.termEnd),
              })}
            </Typography>
            <Button
              variant="ghost"
              pending={cancelSchedule.isPending}
              onClick={() => void clearScheduled()}
            >
              {t('change.cancelScheduled')}
            </Button>
          </span>
        </Notice>
      ) : (
        subscription.plan &&
        subscription.term && (
          <ScheduledChangeButton
            plan={subscription.plan}
            term={nextTerm}
            label={t('change.switchTerm', { term: t(`subscription.term.${nextTerm}`) })}
          />
        )
      )}

      {subscription.autoRenew ? (
        <Button variant="danger" onClick={() => setCancelling(true)}>
          {t('change.cancelSubscription')}
        </Button>
      ) : (
        <>
          <Notice tone="info" role="status">
            {t('change.cancelledForDate', { date: formatDate(subscription.termEnd) })}
          </Notice>
          <Button
            variant="secondary"
            pending={resume.isPending}
            onClick={() => void resumeRenewal()}
          >
            {t('change.resume')}
          </Button>
        </>
      )}

      {(cancelSchedule.errorMessage || cancel.errorMessage || resume.errorMessage) && (
        <FieldMessage>
          {cancelSchedule.errorMessage || cancel.errorMessage || resume.errorMessage}
        </FieldMessage>
      )}
      <Dialog
        open={cancelling}
        title={t('change.cancelTitle')}
        confirmLabel={t('change.cancelSubscription')}
        pending={cancel.isPending}
        onClose={() => setCancelling(false)}
        onConfirm={() => void confirmCancel()}
      >
        {t('change.cancelDescription', { date: formatDate(subscription.termEnd) })}
      </Dialog>
    </div>
  )
}
