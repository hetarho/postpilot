import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { RequestInspectionView } from '@/entities/request-inspection'
import { copyText } from '@/shared/lib'
import { Button, Disclosure, Typography } from '@/shared/ui'
import { issuedAtText, requestDocument } from '../model/request-document'

export function RequestInspectionDocument({ views }: { views: readonly RequestInspectionView[] }) {
  const { t } = useTranslation('requestInspection')
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied' | 'failed'>('idle')
  const document = useRef<HTMLDivElement>(null)
  const active = useRef(false)
  useEffect(() => {
    active.current = true
    return () => {
      active.current = false
    }
  }, [])
  const copy = async () => {
    const result = await copyText(requestDocument(views), undefined, () => active.current)
    if (!active.current) return
    setCopyStatus(result.copied ? 'copied' : 'failed')
    if (!result.copied && document.current) {
      const range = window.document.createRange()
      range.selectNodeContents(document.current)
      const selection = window.getSelection()
      selection?.removeAllRanges()
      selection?.addRange(range)
    }
  }
  return (
    <div className="min-w-0 space-y-4">
      {views.some((view) => view.status !== 'unavailable') && (
        <Button variant="secondary" onClick={() => void copy()}>
          {t('copy')}
        </Button>
      )}
      <Typography variant="body" role="status">
        {copyStatus === 'copied' ? t('copied') : copyStatus === 'failed' ? t('copyFailed') : ''}
      </Typography>
      <div ref={document} className="min-w-0 space-y-8">
        {views.map((view, index) => (
          <InspectionEvidence key={`${view.callId ?? view.status}-${index}`} view={view} />
        ))}
      </div>
    </div>
  )
}

function InspectionEvidence({ view }: { view: RequestInspectionView }) {
  const { t } = useTranslation('requestInspection')
  const unknown = t('unknown')
  const value = (input: string | bigint | number | undefined) =>
    input === undefined || input === '' ? unknown : String(input)
  const boolean = (input: boolean | undefined) =>
    input === undefined ? unknown : t(input ? 'yes' : 'no')
  const reasonKey = {
    blind_test_identity_hidden_until_reveal: 'blind',
    capture_missing_stale_or_purged: 'missing',
    private_test_payload_expired_or_purged: 'expired',
    test_view_requires_retained_execution_capture: 'capturedOnly',
  } as const
  const reason = view.unavailableReason
  const knownReason =
    reason && reason in reasonKey ? reasonKey[reason as keyof typeof reasonKey] : undefined
  return (
    <article className="min-w-0 space-y-6">
      <section>
        <Typography variant="fieldTitle" as="h3">
          {t(`status.${view.status}`)}
        </Typography>
        <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
          {t(`statusHelp.${view.status}`)}
        </Typography>
        <dl className="mt-3 grid gap-2">
          <EvidenceRow
            label={t('actualStage')}
            value={
              view.stage
                ? `${t(`stage.${view.stage}`, { defaultValue: view.stage })} · ${view.stage}`
                : unknown
            }
          />
          <EvidenceRow label={t('actualStatus')} value={t(`status.${view.status}`)} />
        </dl>
      </section>
      {view.status === 'unavailable' ? (
        <Typography variant="body">
          {knownReason ? t(`reason.${knownReason}`) : reason || t('reason.generic')}
        </Typography>
      ) : (
        <>
          <EvidenceSection title={t('identity')}>
            <dl className="grid gap-2">
              <EvidenceRow label={t('mode')} value={value(view.mode)} />
              <EvidenceRow label={t('version')} value={view.version} />
              <EvidenceRow label={t('promptVersion')} value={value(view.promptVersion)} />
              <EvidenceRow label={t('schemaVersion')} value={value(view.schemaVersion)} />
              {view.status === 'captured' && (
                <>
                  <EvidenceRow label={t('callId')} value={value(view.callId)} />
                  <EvidenceRow label={t('issuedAt')} value={value(issuedAtText(view.issuedAt))} />
                </>
              )}
            </dl>
          </EvidenceSection>
          <EvidenceSection title={t('conditions')}>
            <dl className="grid gap-2">
              <EvidenceRow
                label={t('model')}
                value={
                  view.conditions?.model
                    ? `${view.conditions.model.providerId}/${view.conditions.model.modelId}`
                    : unknown
                }
              />
              <EvidenceRow
                label={t('budget')}
                value={value(view.conditions?.maxCompletionTokens)}
              />
              <EvidenceRow label={t('effort')} value={value(view.conditions?.reasoningEffort)} />
              <EvidenceRow
                label={t('structured')}
                value={boolean(view.conditions?.structuredOutput)}
              />
              <EvidenceRow
                label={t('reasoningDisabled')}
                value={boolean(view.conditions?.disableReasoning)}
              />
              <EvidenceRow
                label={t('reasoningOmitted')}
                value={boolean(view.conditions?.reasoningOmitted)}
              />
              <EvidenceRow
                label={t('defaultBudget')}
                value={boolean(view.conditions?.defaultBudget)}
              />
              <EvidenceRow label={t('frozen')} value={boolean(view.conditions?.frozenExecution)} />
            </dl>
          </EvidenceSection>
          <EvidenceSection title={t('measures')}>
            <Typography variant="body" className="text-content-secondary max-w-measure">
              {t('measuresHelp')}
            </Typography>
            <dl className="mt-3 grid gap-2">
              <EvidenceRow label={t('characters')} value={value(view.measures?.characters)} />
              <EvidenceRow label={t('utf8Bytes')} value={value(view.measures?.utf8Bytes)} />
              <EvidenceRow
                label={t('referenceTokenEstimate')}
                value={value(view.measures?.referenceTokenEstimate)}
              />
              <EvidenceRow
                label={t('providerPromptTokens')}
                value={value(view.measures?.providerPromptTokens)}
              />
              <EvidenceRow
                label={t('providerCompletionTokens')}
                value={value(view.measures?.providerCompletionTokens)}
              />
              <EvidenceRow
                label={t('providerReasoningTokens')}
                value={value(view.measures?.providerReasoningTokens)}
              />
            </dl>
          </EvidenceSection>
          <EvidenceSection title={t('fragments')}>
            <Typography variant="body" className="text-content-secondary max-w-measure">
              {t('fragmentsHelp')}
            </Typography>
            <ol className="mt-4 grid min-w-0 gap-6">
              {view.fragments.map((fragment, index) => (
                <li key={`${fragment.id}-${index}`} className="min-w-0 space-y-3">
                  <Typography variant="fieldTitle" as="h4">
                    {t('fragment', { number: index + 1, role: t(`role.${fragment.role}`) })}
                  </Typography>
                  <dl className="grid gap-2">
                    <EvidenceRow label={t('fragmentId')} value={fragment.id} />
                    <EvidenceRow
                      label={t('owner')}
                      value={t(`authorship.${fragment.authorship}`)}
                    />
                    <EvidenceRow label={t('materialRole')} value={value(fragment.materialRole)} />
                    <SourceRows
                      sourceRefs={fragment.sourceRefs}
                      sourceFiles={fragment.sourceFiles}
                      activation={fragment.activation}
                    />
                  </dl>
                  <TechnicalText>{fragment.text}</TechnicalText>
                </li>
              ))}
            </ol>
            {view.fragments.length === 0 && (
              <Typography variant="body" className="mt-2">
                {t('empty')}
              </Typography>
            )}
          </EvidenceSection>
          {view.nativeFields && view.nativeFields.length > 0 && (
            <EvidenceSection title={t('native')}>
              <ul className="grid min-w-0 gap-6">
                {view.nativeFields.map((field) => (
                  <li key={field.id} className="min-w-0 space-y-3">
                    <Typography variant="fieldTitle" as="h4">
                      {field.id}
                    </Typography>
                    <dl className="grid gap-2">
                      <EvidenceRow label={t('owner')} value={t(`authorship.${field.authorship}`)} />
                      <EvidenceRow label={t('materialRole')} value={value(field.materialRole)} />
                      <SourceRows
                        sourceRefs={field.sourceRefs}
                        sourceFiles={field.sourceFiles}
                        activation={field.activation}
                      />
                    </dl>
                    <TechnicalText>{field.text}</TechnicalText>
                  </li>
                ))}
              </ul>
            </EvidenceSection>
          )}
          <EvidenceSection title={t('rules')}>
            <ul className="grid gap-2">
              {view.selectedRuleIds.map((rule) => (
                <Typography key={rule} variant="body" as="li" mono className="break-words">
                  {rule}
                </Typography>
              ))}
            </ul>
            {view.selectedRuleIds.length === 0 && (
              <Typography variant="body">{t('empty')}</Typography>
            )}
          </EvidenceSection>
          <EvidenceSection title={t('omissions')}>
            <ul className="grid gap-4">
              {(view.omissions ?? []).map((omission) => (
                <li key={omission.id} className="min-w-0 space-y-2">
                  <Typography variant="body" mono className="break-words">
                    {omission.id}
                  </Typography>
                  <Typography variant="body" className="break-words">
                    {omission.reason}
                  </Typography>
                  <dl className="grid gap-2">
                    <SourceRows
                      sourceFiles={omission.sourceFiles}
                      activation={omission.activation}
                    />
                  </dl>
                </li>
              ))}
            </ul>
            {!view.omissions?.length && <Typography variant="body">{t('empty')}</Typography>}
          </EvidenceSection>
          {view.attachments && view.attachments.length > 0 && (
            <EvidenceSection title={t('attachments')}>
              <ul className="grid gap-2">
                {view.attachments.map((attachment) => (
                  <Typography key={attachment.id} variant="body" as="li" className="break-words">
                    {t(`attachment.${attachment.kind}`)} · {attachment.id}
                  </Typography>
                ))}
              </ul>
            </EvidenceSection>
          )}
          <EvidenceSection title={t('output')}>
            <dl className="grid gap-2">
              <EvidenceRow label={t('outputName')} value={value(view.output?.name)} />
              <EvidenceRow label={t('outputVersion')} value={value(view.output?.version)} />
            </dl>
            {view.output?.schema && (
              <Disclosure title={t('grammar')} size="row" headingLevel={4} className="mt-3">
                <TechnicalText>{view.output.schema}</TechnicalText>
              </Disclosure>
            )}
          </EvidenceSection>
          <EvidenceSection title={t('composition')}>
            <dl className="grid gap-2">
              <EvidenceRow label={t('composer')} value={value(view.composer)} />
              <EvidenceRow label={t('parser')} value={value(view.parser)} />
              <EvidenceRow label={t('consumer')} value={value(view.consumer)} />
              <SourceRows sourceFiles={view.sourceFiles} activation={view.activation} />
            </dl>
          </EvidenceSection>
        </>
      )}
    </article>
  )
}

function EvidenceSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="min-w-0 space-y-3">
      <Typography variant="fieldTitle" as="h3">
        {title}
      </Typography>
      {children}
    </section>
  )
}

function EvidenceRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid min-w-0 gap-1 sm:grid-cols-2 sm:gap-4">
      <Typography variant="body" as="dt" className="text-content-secondary">
        {label}
      </Typography>
      <Typography variant="body" as="dd" className="min-w-0 break-words">
        {value}
      </Typography>
    </div>
  )
}

function SourceRows({
  sourceRefs,
  sourceFiles,
  activation,
}: {
  sourceRefs?: readonly string[]
  sourceFiles?: readonly string[]
  activation?: string
}) {
  const { t } = useTranslation('requestInspection')
  return (
    <>
      {sourceRefs?.length ? (
        <EvidenceRow label={t('sourceRefs')} value={sourceRefs.join(', ')} />
      ) : null}
      {sourceFiles?.length ? (
        <EvidenceRow label={t('sourceFiles')} value={sourceFiles.join(', ')} />
      ) : null}
      {activation ? <EvidenceRow label={t('activation')} value={activation} /> : null}
    </>
  )
}

function TechnicalText({ children }: { children: string }) {
  return (
    <Typography
      variant="body"
      as="pre"
      mono
      className="bg-surface-recessed min-w-0 rounded-md p-3 break-words whitespace-pre-wrap"
    >
      {children}
    </Typography>
  )
}
