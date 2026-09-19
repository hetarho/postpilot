import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { useStageSelection } from '@/entities/model-catalog'
import type { VoiceProfile } from '@/entities/voice'
import { useStartVoiceRuleComparison, useVoiceRuleActions } from '@/entities/voice'
import { appFailureFromConnect, VoiceRuleStatus } from '@/shared/api'
import { AppFailureMessage, Badge, Button, Dialog, Notice, Typography } from '@/shared/ui'

interface RuleConfirmation {
  id: string
  ruleId: string
  existingStatement: string
  proposedStatement: string
  status: string
}

/** Contrast rules and their pending conflicts, for one voice. Rule-derived calls name only the
 *  rule — the server reads the voice off it — so `voiceId` here is for the caches, which are
 *  partitioned per voice, and for the comparison route. */
export function VoiceRulesManager({
  ownerId,
  voiceId,
  profile,
  confirmations,
  blocked = '',
}: {
  ownerId: string
  voiceId: string
  profile: VoiceProfile
  confirmations: RuleConfirmation[]
  blocked?: string
}) {
  const { t } = useTranslation('voices')
  const navigate = useNavigate()
  const write = useStageSelection('write')
  const rules = useVoiceRuleActions(ownerId, voiceId)
  const comparison = useStartVoiceRuleComparison()
  const [compareRule, setCompareRule] = useState<string>()
  const changeStatus = (ruleId: string, status: VoiceRuleStatus) => {
    if (blocked) return
    void rules.setStatus(ruleId, status).catch(() => undefined)
  }
  const startComparison = async () => {
    if (blocked) return
    const source = profile.structured.sources[0]
    if (!compareRule || !source || !write.selected) return
    const response = await comparison.start({
      ruleId: compareRule,
      sourceId: source.id,
      writeModel: write.selected,
    })
    setCompareRule(undefined)
    if (response.comparisonId) {
      void navigate({
        to: '/voices/$voiceId/rules/$id/compare',
        params: { voiceId, id: response.comparisonId },
      })
    }
  }
  const pending = confirmations.filter((item) => item.status === 'pending')
  const actionError = rules.error ?? comparison.error
  return (
    <section aria-label={t('rules.title')}>
      {blocked && (
        <Notice tone="warning" role="status" className="mb-4">
          {blocked}
        </Notice>
      )}
      {actionError && (
        <Notice tone="danger" role="alert" className="mb-4">
          <AppFailureMessage failure={appFailureFromConnect(actionError)} />
        </Notice>
      )}
      {profile.structured.rules.length === 0 ? (
        <Typography variant="body" className="text-content-tertiary mt-4">
          {t('rules.empty')}
        </Typography>
      ) : (
        <ul className="divide-divider divide-y">
          {profile.structured.rules.map((rule) => (
            <li key={rule.id} className="py-4">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <Typography variant="body" className="min-w-0">
                  {rule.statement}
                </Typography>
                <Badge
                  tone={
                    rule.status === 'active'
                      ? 'success'
                      : rule.status === 'candidate'
                        ? 'warning'
                        : 'neutral'
                  }
                >
                  {t('rules.evidence', {
                    status: t(`rules.status.${rule.status}`),
                    count: rule.evidenceCount,
                  })}
                </Badge>
              </div>
              <Typography variant="meta" as="p" className="mt-1">
                {t(`rules.layer.${rule.layer}`)}
              </Typography>
              <div className="mt-3 flex flex-wrap gap-2">
                {rule.status !== 'active' && (
                  <Button
                    variant="secondary"
                    disabled={Boolean(blocked)}
                    onClick={() => changeStatus(rule.id, VoiceRuleStatus.ACTIVE)}
                  >
                    {t('rules.activate')}
                  </Button>
                )}
                {rule.status !== 'retired' && (
                  <Button
                    variant="ghost"
                    disabled={Boolean(blocked)}
                    onClick={() => changeStatus(rule.id, VoiceRuleStatus.RETIRED)}
                  >
                    {t('rules.retire')}
                  </Button>
                )}
                {rule.status === 'candidate' && (
                  <Button
                    variant="ghost"
                    disabled={Boolean(blocked) || !profile.structured.sources[0] || !write.selected}
                    onClick={() => setCompareRule(rule.id)}
                  >
                    {t('rules.compare')}
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {pending.length > 0 && (
        <section className="mt-8">
          <Typography variant="title" as="h3">
            {t('rules.conflicts')}
          </Typography>
          {pending.map((item) => (
            <Notice key={item.id} tone="warning" className="mt-3">
              <p>{t('rules.current', { statement: item.existingStatement })}</p>
              <p className="mt-1">{t('rules.proposed', { statement: item.proposedStatement })}</p>
              <div className="mt-3 flex gap-2">
                <Button
                  variant="secondary"
                  disabled={Boolean(blocked)}
                  onClick={() => void rules.resolve(item.id, false).catch(() => undefined)}
                >
                  {t('rules.keep')}
                </Button>
                <Button
                  variant="cta"
                  disabled={Boolean(blocked)}
                  onClick={() => void rules.resolve(item.id, true).catch(() => undefined)}
                >
                  {t('rules.replace')}
                </Button>
              </div>
            </Notice>
          ))}
        </section>
      )}
      <Dialog
        open={compareRule !== undefined}
        title={t('rules.compareTitle')}
        confirmLabel={t('rules.compareConfirm')}
        pending={comparison.isPending}
        onClose={() => setCompareRule(undefined)}
        onConfirm={() => void startComparison().catch(() => undefined)}
      >
        {t('rules.compareDescription')}
      </Dialog>
    </section>
  )
}
