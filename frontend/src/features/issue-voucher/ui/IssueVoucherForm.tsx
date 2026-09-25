import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  CopyGiftLink,
  VOUCHER_LIMITS,
  useIssueVoucher,
  type VoucherPreset,
} from '@/entities/voucher'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Notice,
  SegmentedControl,
  TextField,
  Typography,
} from '@/shared/ui'
import {
  digitsOnly,
  toIssue,
  type IssueDraft,
  type IssueError,
  type IssueMode,
} from '../model/issue-form'

const EMPTY: IssueDraft = {
  mode: 'pro',
  credits: '',
  days: '',
  kind: 'sold',
  amount: '',
  payer: '',
  message: '',
}

const ERROR_MAX: Record<IssueError, number> = {
  credits: VOUCHER_LIMITS.maxCredits,
  days: VOUCHER_LIMITS.maxDays,
  amount: VOUCHER_LIMITS.maxSaleKrw,
  payer: VOUCHER_LIMITS.maxPayer,
  message: VOUCHER_LIMITS.maxMessage,
}

const won = new Intl.NumberFormat('ko-KR')

/** Issues a voucher from a paid rung's preset or from custom numbers, recorded as sold or given
 *  (GIFT-3, GIFT-4, GIFT-5), and hands back the gift link to copy (GIFT-6). */
export function IssueVoucherForm({ presets }: { presets: readonly VoucherPreset[] }) {
  const { t } = useTranslation('plans')
  const id = useId()
  const issue = useIssueVoucher()
  const [draft, setDraft] = useState<IssueDraft>(EMPTY)
  const [errors, setErrors] = useState<IssueError[]>([])
  const set = (patch: Partial<IssueDraft>) => setDraft((current) => ({ ...current, ...patch }))
  const errorFor = (field: IssueError) =>
    errors.includes(field) ? (
      <FieldMessage id={`${id}-${field}-error`}>
        {t(`issueVoucher.error.${field}`, { max: won.format(ERROR_MAX[field]) })}
      </FieldMessage>
    ) : null

  const modes: { value: IssueMode; label: string }[] = [
    ...presets.map((preset) => ({
      value: preset.plan,
      label: t('issueVoucher.preset', {
        plan: preset.plan,
        credits: preset.credits,
        days: preset.validityDays,
      }),
    })),
    { value: 'custom', label: t('issueVoucher.custom') },
  ]

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const result = toIssue(draft, presets)
    if ('errors' in result) {
      setErrors(result.errors)
      return
    }
    setErrors([])
    issue.issue(result.issue)
  }

  return (
    <section className="bg-surface-raised mt-8 rounded-md p-4" aria-labelledby={`${id}-heading`}>
      <Typography variant="title" as="h2" id={`${id}-heading`}>
        {t('issueVoucher.heading')}
      </Typography>
      <form onSubmit={submit} className="mt-4 grid gap-4" noValidate>
        <div className="grid gap-2">
          <Typography variant="label" as="p">
            {t('issueVoucher.contents')}
          </Typography>
          <SegmentedControl
            value={draft.mode}
            options={modes}
            onChange={(mode) => set({ mode })}
            ariaLabel={t('issueVoucher.contents')}
            className="flex-wrap"
          />
        </div>
        {draft.mode === 'custom' && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-2">
              <FieldLabel htmlFor={`${id}-credits`}>{t('issueVoucher.credits')}</FieldLabel>
              <TextField
                id={`${id}-credits`}
                inputMode="numeric"
                value={draft.credits}
                onChange={(event) => set({ credits: digitsOnly(event.target.value) })}
                aria-invalid={errors.includes('credits')}
              />
              {errorFor('credits')}
            </div>
            <div className="grid gap-2">
              <FieldLabel htmlFor={`${id}-days`}>{t('issueVoucher.days')}</FieldLabel>
              <TextField
                id={`${id}-days`}
                inputMode="numeric"
                value={draft.days}
                onChange={(event) => set({ days: digitsOnly(event.target.value) })}
                aria-invalid={errors.includes('days')}
              />
              {errorFor('days')}
            </div>
          </div>
        )}
        <div className="grid gap-2">
          <Typography variant="label" as="p">
            {t('issueVoucher.kind')}
          </Typography>
          <SegmentedControl
            value={draft.kind}
            options={[
              { value: 'sold', label: t('issueVoucher.sold') },
              { value: 'given', label: t('issueVoucher.given') },
            ]}
            onChange={(kind) => set({ kind })}
            ariaLabel={t('issueVoucher.kind')}
          />
        </div>
        {draft.kind === 'sold' && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-2">
              <FieldLabel htmlFor={`${id}-amount`}>{t('issueVoucher.amount')}</FieldLabel>
              <TextField
                id={`${id}-amount`}
                inputMode="numeric"
                value={draft.amount === '' ? '' : won.format(Number(draft.amount))}
                onChange={(event) => set({ amount: digitsOnly(event.target.value) })}
                aria-invalid={errors.includes('amount')}
              />
              {errorFor('amount')}
            </div>
            <div className="grid gap-2">
              <FieldLabel htmlFor={`${id}-payer`}>{t('issueVoucher.payer')}</FieldLabel>
              <TextField
                id={`${id}-payer`}
                value={draft.payer}
                onChange={(event) => set({ payer: event.target.value })}
                aria-invalid={errors.includes('payer')}
              />
              {errorFor('payer')}
            </div>
          </div>
        )}
        <div className="grid gap-2">
          <FieldLabel htmlFor={`${id}-message`}>{t('issueVoucher.message')}</FieldLabel>
          <TextField
            id={`${id}-message`}
            value={draft.message}
            onChange={(event) => set({ message: event.target.value })}
            aria-invalid={errors.includes('message')}
          />
          {errorFor('message')}
        </div>
        {issue.failure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={issue.failure} />
          </Notice>
        )}
        <div>
          <Button type="submit" variant="cta" pending={issue.isPending}>
            {t('issueVoucher.submit')}
          </Button>
        </div>
      </form>
      {issue.issued?.token && (
        <div className="mt-6 grid gap-3">
          <Notice tone="success" role="status">
            {t('issueVoucher.issued')}
          </Notice>
          <CopyGiftLink token={issue.issued.token} />
        </div>
      )}
    </section>
  )
}
