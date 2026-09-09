import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useApplyCatalogDocument,
  useCatalogDocument,
  usePreviewCatalogDocument,
} from '@/entities/model-catalog'
import type { CatalogDocumentIssue, CatalogDocumentPlan } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Notice,
  Sheet,
  Textarea,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { canApply, documentDiff, isKnownCause, type DocumentDiffRow } from '../model/document-view'

interface CatalogDocumentPanelProps {
  open: boolean
  onClose: () => void
}

/** Reopening starts clean rather than showing the previous session's diff over a catalog that
 *  has moved since. The parent remounts this by key instead of an effect resetting state —
 *  the state IS the session, so a new session is a new component. */

/** 일괄 편집: the operator arrives with a list, and this is where it goes in. One surface for
 *  all five tabs rather than a control per tab, because one document names any purpose.
 *
 *  The flow is deliberately two-step: 미리보기 shows what would change and writes nothing,
 *  확정 sends the SAME TEXT again for the server to validate from scratch. The preview's answer
 *  is display state and never a token — the catalog moves between the two calls. */
export function CatalogDocumentPanel({ open, onClose }: CatalogDocumentPanelProps) {
  const { t } = useTranslation('models')
  const [text, setText] = useState('')
  const exported = useCatalogDocument(open)
  const preview = usePreviewCatalogDocument()
  const apply = useApplyCatalogDocument()
  const fieldId = useId()

  // The plan on screen is the apply's answer once there is one, so a rejection the catalog
  // moved into between the two calls replaces the preview that no longer holds.
  const plan: CatalogDocumentPlan | undefined = apply.result ?? preview.plan
  const diff = documentDiff(plan)
  const applied = apply.result?.applied === true

  // Editing invalidates the diff on screen: a plan computed for text the operator has since
  // changed must not be what 확정 commits.
  const onEdit = (next: string) => {
    setText(next)
    if (preview.plan || apply.result) {
      preview.reset()
      apply.reset()
    }
  }

  return (
    <Sheet
      open={open}
      labelledBy="catalog-document-title"
      onClose={onClose}
      header={
        <Typography variant="title" as="h2" id="catalog-document-title">
          {t('document.title')}
        </Typography>
      }
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>
            {t('document.close')}
          </Button>
          <Button
            variant="secondary"
            onClick={() => preview.preview(text)}
            pending={preview.isPending}
            disabled={text.trim() === ''}
          >
            {t('document.preview')}
          </Button>
          <Button
            onClick={() => apply.apply(text)}
            pending={apply.isPending}
            disabled={!canApply(plan) || applied}
          >
            {t('document.apply')}
          </Button>
        </div>
      }
    >
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('document.description')}
      </Typography>
      {/* Said before anything is pasted, not after the diff: a section is the purpose's FINAL
          state, so what it leaves out is deregistered. */}
      <Notice tone="warning" className="mt-3">
        {t('document.syncWarning')}
      </Notice>

      <section className="mt-6">
        <Typography variant="label" className="block">
          {t('document.currentTitle')}
        </Typography>
        <Typography variant="meta" className="text-content-tertiary mt-1 block">
          {t('document.currentHint')}
        </Typography>
        {exported.isError ? (
          <Notice tone="danger" role="alert" className="mt-2">
            {t('document.currentFailed')}
          </Notice>
        ) : (
          <pre
            className={typographyStyles({
              variant: 'meta',
              mono: true,
              className:
                'border-line bg-surface-sunken mt-2 max-h-64 overflow-auto rounded-md border p-3',
            })}
          >
            {exported.isPending ? t('document.currentLoading') : exported.document}
          </pre>
        )}
      </section>

      <section className="mt-6">
        <FieldLabel htmlFor={fieldId}>{t('document.pasteLabel')}</FieldLabel>
        <Textarea
          id={fieldId}
          rows={10}
          value={text}
          onChange={(e) => onEdit(e.target.value)}
          placeholder={t('document.pastePlaceholder')}
          spellCheck={false}
          autoCapitalize="none"
          autoCorrect="off"
          className={typographyStyles({ variant: 'meta', mono: true, className: 'mt-1' })}
        />
      </section>

      {(preview.failure ?? apply.failure) && (
        <div className="mt-4">
          <AppFailureMessage failure={(preview.failure ?? apply.failure)!} />
        </div>
      )}

      {plan?.fetchError !== undefined && plan.fetchError !== '' && (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('document.fetchFailed')}
        </Notice>
      )}

      {plan && plan.issues.length > 0 && (
        <section className="mt-4">
          <Notice tone="danger" role="alert">
            {t('document.rejected', { count: plan.issues.length })}
          </Notice>
          <ul className="mt-3 grid gap-2">
            {plan.issues.map((issue) => (
              <IssueRow key={`${issue.line}-${issue.cause}`} issue={issue} />
            ))}
          </ul>
        </section>
      )}

      {applied && (
        <Notice tone="success" role="status" className="mt-4">
          {t('document.applied', {
            registered: diff.registerCount,
            deregistered: diff.deregisterCount,
          })}
        </Notice>
      )}

      {plan && plan.issues.length === 0 && plan.fetchError === '' && (
        <section className="mt-6">
          <Typography variant="label" className="block">
            {t('document.diffTitle')}
          </Typography>
          {diff.rows.length === 0 ? (
            <Typography variant="body" className="text-content-tertiary mt-2 block">
              {t('document.diffEmpty')}
            </Typography>
          ) : (
            <ul className="mt-2 grid gap-4">
              {diff.rows.map((row) => (
                <DiffRow key={row.purpose} row={row} />
              ))}
            </ul>
          )}
          {diff.untouched.length > 0 && (
            <Typography variant="meta" className="text-content-tertiary mt-4 block">
              {t('document.untouched', {
                purposes: diff.untouched
                  .map((purpose) => t(`catalog.purposeTab.${purpose}`))
                  .join(', '),
              })}
            </Typography>
          )}
        </section>
      )}
    </Sheet>
  )
}

function IssueRow({ issue }: { issue: CatalogDocumentIssue }) {
  const { t } = useTranslation('models')
  return (
    <li className="border-line rounded-md border p-3">
      <Typography variant="label" className="block">
        {t('document.issueLine', { line: issue.line })}
      </Typography>
      <Typography variant="meta" className="text-content-secondary mt-1 block break-all">
        {/* A cause slug a newer server invented reads as the fallback rather than as some other
            cause's copy. */}
        {isKnownCause(issue.cause)
          ? t(`document.issueCause.${issue.cause}`)
          : t('document.issueCause.unknown')}
      </Typography>
      {issue.text !== '' && (
        <Typography variant="meta" as="code" mono className="mt-1 block break-all">
          {issue.text}
        </Typography>
      )}
    </li>
  )
}

function DiffRow({ row }: { row: DocumentDiffRow }) {
  const { t } = useTranslation('models')
  return (
    <li>
      <Typography variant="label" className="block">
        {t(`catalog.purposeTab.${row.purpose}`)}
      </Typography>
      {!row.touched && (
        <Typography variant="meta" className="text-content-tertiary mt-1 block">
          {t('document.noChange')}
        </Typography>
      )}
      {/* Deregistration is the loud half: it is the half that takes a model away from every
          account that had selected it. */}
      {row.deregister.length > 0 && (
        <div className="mt-1">
          <Typography variant="meta" className="text-notice-danger-fg block">
            {t('document.deregister', { count: row.deregister.length })}
          </Typography>
          <ul className="mt-1 grid gap-0.5">
            {row.deregister.map((id) => (
              <Typography key={id} variant="meta" as="li" mono className="break-all">
                {id}
              </Typography>
            ))}
          </ul>
        </div>
      )}
      {row.register.length > 0 && (
        <div className="mt-1">
          <Typography variant="meta" className="text-content-secondary block">
            {t('document.register', { count: row.register.length })}
          </Typography>
          <ul className="mt-1 grid gap-0.5">
            {row.register.map((id) => (
              <Typography key={id} variant="meta" as="li" mono className="break-all">
                {id}
              </Typography>
            ))}
          </ul>
        </div>
      )}
      {row.unchanged.length > 0 && (
        <Typography variant="meta" className="text-content-tertiary mt-1 block">
          {t('document.unchanged', { count: row.unchanged.length })}
        </Typography>
      )}
    </li>
  )
}
