import { useEffect, useLayoutEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipDraftKey,
  proposeSpokenRetiming,
  spokenScriptValid,
  spokenState,
  useClipSpeechCalls,
  type ClipSpeechQuote,
  type ClipEditPlan,
  type ClipEditingState,
  type ClipSpokenSegment,
  type TimelineEdit,
  type SpokenRetiming,
} from '@/entities/clip-plan'
import { useRefreshClipProjects, type ClipProject } from '@/entities/clip-project'
import { useMyPlan } from '@/entities/plan'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Sheet, Slider, Typography } from '@/shared/ui'
import { ClipVoiceChoice } from './ClipVoiceChoice'
import { ClipSpokenProperties } from './ClipSpokenProperties'
interface Props {
  ownerId: string
  project: ClipProject
  plan: ClipEditPlan
  state: ClipEditingState
  revision: number
  disabled?: boolean
  change: (edit: TimelineEdit, group?: string) => void
  flush: () => Promise<number>
  onSelect: (segment: ClipSpokenSegment) => void
}
export function ClipDubbingEditor({
  ownerId,
  project,
  plan,
  state,
  revision,
  disabled,
  change,
  flush,
  onSelect,
}: Props) {
  const { t } = useTranslation('clips'),
    id = useId(),
    calls = useClipSpeechCalls(),
    refresh = useRefreshClipProjects(ownerId),
    { myPlan } = useMyPlan()
  const [busy, setBusy] = useState(false),
    [error, setError] = useState<unknown>(),
    [quote, setQuote] = useState<{ quote: ClipSpeechQuote; key: string; draftKey: string }>(),
    [now, setNow] = useState(Date.now),
    [proposal, setProposal] = useState<SpokenRetiming>()
  const current = useRef({ plan, revision })
  useLayoutEffect(() => {
    current.current = { plan, revision }
  }, [plan, revision])
  const key = clipDraftKey(plan),
    n = plan.narration,
    segments = n?.segments ?? [],
    missing = segments.filter((s) =>
      ['missing', 'stale'].includes(spokenState(n!, s, plan.durationMs)),
    ),
    conflict = segments.some((s) => spokenState(n!, s, plan.durationMs) === 'conflict'),
    valid = spokenScriptValid(n)
  useEffect(() => {
    if (!quote) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [quote])
  const invalidQuote =
    !!quote &&
    (quote.draftKey !== key ||
      quote.quote.revision !== revision ||
      Date.parse(quote.quote.expiresAt) <= now)
  async function estimate() {
    setBusy(true)
    setError(undefined)
    try {
      const rev = await flush()
      const draftKey = clipDraftKey(current.current.plan)
      const q = await calls.quote(project.id, rev)
      setQuote({ quote: q, key: crypto.randomUUID(), draftKey })
      setNow(Date.now())
    } catch (e) {
      setError(e)
    } finally {
      setBusy(false)
    }
  }
  async function approve() {
    if (!quote || invalidQuote) return
    setBusy(true)
    setError(undefined)
    try {
      const rev = await flush()
      if (rev !== quote.quote.revision || quote.draftKey !== clipDraftKey(current.current.plan))
        throw new Error('Clip draft changed')
      await calls.start(project.id, quote.quote, quote.key)
      setQuote(undefined)
      await refresh.detail(project.id)
    } catch (e) {
      setError(e)
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="min-w-0 space-y-4">
      <ClipVoiceChoice
        ownerId={ownerId}
        enabled={!!n?.enabled}
        voiceId={n?.confirmedVoiceId ?? ''}
        disabled={disabled || busy}
        onChange={(value) =>
          change({
            type: 'narrationOptions',
            enabled: value.enabled,
            confirmedVoiceId: value.voiceId,
          })
        }
      />
      <fieldset disabled={disabled || busy} className="min-w-0 space-y-3">
        <Slider
          label={t('dubbing.sourceVolume')}
          value={plan.sourceVolumePermille ?? 1000}
          min={0}
          max={1000}
          step={10}
          valueText={`${(plan.sourceVolumePermille ?? 1000) / 10}%`}
          onChange={(volumePermille) =>
            change({ type: 'sourceGain', volumePermille }, 'source-gain')
          }
        />
        {n && (
          <Slider
            label={t('dubbing.voiceVolume')}
            value={n.volumePermille}
            min={0}
            max={1000}
            step={10}
            valueText={`${n.volumePermille / 10}%`}
            onChange={(volumePermille) =>
              change({ type: 'narrationOptions', volumePermille }, 'narration-gain')
            }
          />
        )}
      </fieldset>
      <Typography variant="meta">{t('dubbing.independent')}</Typography>
      {segments.map((s, i) => (
        <details key={s.id} className="space-y-2">
          <summary className="text-content-primary cursor-pointer">
            {t('dubbing.segment', { number: i + 1 })} ·{' '}
            {n && t(`dubbing.states.${spokenState(n, s, plan.durationMs)}`)}
          </summary>
          <Button variant="ghost" onClick={() => onSelect(s)}>
            {t('dubbing.select')}
          </Button>
          <ClipSpokenProperties
            ownerId={ownerId}
            projectId={project.id}
            narration={n!}
            segment={s}
            durationMs={plan.durationMs}
            disabled={disabled || busy}
            change={change}
          />
        </details>
      ))}
      {n && (
        <Button
          variant="secondary"
          disabled={disabled || busy || segments.length >= 32}
          onClick={() => {
            const startMs = segments.at(-1)?.endMs ?? 0
            change({
              type: 'addSpoken',
              id: `new-spoken-${crypto.randomUUID()}`,
              startMs,
              endMs: startMs + 2000,
            })
          }}
        >
          {t('dubbing.add')}
        </Button>
      )}
      {!valid && (
        <Typography variant="meta" role="alert">
          {t('dubbing.limits')}
        </Typography>
      )}
      {n?.enabled && (
        <>
          <Typography variant="meta">{t('dubbing.pending', { count: missing.length })}</Typography>
          <Button
            variant="cta"
            disabled={disabled || busy || !valid || !n.confirmedVoiceId || missing.length === 0}
            pending={busy}
            onClick={() => void estimate()}
          >
            {t('dubbing.estimate')}
          </Button>
        </>
      )}
      {conflict && (
        <div className="space-y-2">
          <Typography variant="body">{t('dubbing.conflict')}</Typography>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              disabled={disabled || busy}
              onClick={() =>
                setProposal(
                  proposeSpokenRetiming(
                    plan,
                    state,
                    project.observations,
                    project.targetDurationMs || 60000,
                    project.allowedCaptionStyles,
                  ),
                )
              }
            >
              {t('dubbing.reviewRetiming')}
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                const s = segments.find((s) => spokenState(n!, s, plan.durationMs) === 'conflict')
                if (s) onSelect(s)
              }}
            >
              {t('dubbing.reviseScript')}
            </Button>
          </div>
        </div>
      )}
      {error !== undefined && <AppFailureMessage failure={appFailureFromConnect(error)} />}
      <Sheet
        open={!!quote}
        labelledBy={`${id}-quote`}
        onClose={() => {
          if (!busy) setQuote(undefined)
        }}
        header={
          <Typography id={`${id}-quote`} variant="fieldTitle">
            {t('dubbing.approval')}
          </Typography>
        }
        footer={
          <div className="flex flex-wrap justify-end gap-2">
            <Button variant="ghost" disabled={busy} onClick={() => setQuote(undefined)}>
              {t('dubbing.close')}
            </Button>
            <Button variant="secondary" pending={busy} onClick={() => void estimate()}>
              {t('dubbing.refreshQuote')}
            </Button>
            <Button
              variant="cta"
              pending={busy}
              disabled={
                invalidQuote ||
                !quote?.quote.id ||
                (!myPlan?.balance.unlimited &&
                  (myPlan?.balance.credits ?? -1) < (quote?.quote.maximumCredits ?? 0))
              }
              onClick={() => void approve()}
            >
              {t('dubbing.approve', { credits: quote?.quote.maximumCredits ?? 0 })}
            </Button>
          </div>
        }
      >
        <div className="space-y-3">
          <Typography variant="body">
            {t('dubbing.quoted', { count: quote?.quote.segmentIds.length ?? 0 })}
          </Typography>
          <Typography variant="body">{t('cancellation.rule')}</Typography>
          {invalidQuote && (
            <Typography variant="body" role="alert">
              {t('dubbing.quoteChanged')}
            </Typography>
          )}
          <ul>
            {quote?.quote.segmentIds.map((id) => (
              <li key={id}>
                <Typography variant="meta">
                  {segments.find((s) => s.id === id)?.text ?? id}
                </Typography>
              </li>
            ))}
          </ul>
          {error !== undefined && <AppFailureMessage failure={appFailureFromConnect(error)} />}
        </div>
      </Sheet>
      <Sheet
        open={!!proposal}
        labelledBy={`${id}-retiming`}
        onClose={() => setProposal(undefined)}
        header={
          <Typography id={`${id}-retiming`} variant="fieldTitle">
            {t('dubbing.reviewRetiming')}
          </Typography>
        }
        footer={
          proposal?.available ? (
            <Button
              variant="cta"
              disabled={disabled || proposal.expectedKey !== key}
              onClick={() => {
                if (proposal.available) {
                  change({
                    type: 'replacePlan',
                    plan: proposal.plan,
                    expectedKey: proposal.expectedKey,
                  })
                  setProposal(undefined)
                }
              }}
            >
              {t('dubbing.applyRetiming')}
            </Button>
          ) : undefined
        }
      >
        {proposal?.available ? (
          <div className="space-y-3">
            <Typography variant="body">
              {t('dubbing.proposal', {
                before: (plan.durationMs / 1000).toFixed(2),
                after: (proposal.plan.durationMs / 1000).toFixed(2),
                cuts: proposal.changedCuts.length,
                captions: proposal.changedCaptions.length,
              })}
            </Typography>
            <ul>
              {proposal.plan.narration?.segments.map((s) => (
                <li key={s.id}>
                  <Typography variant="meta">
                    {s.text} · {(s.startMs / 1000).toFixed(2)}–{(s.endMs / 1000).toFixed(2)} s
                  </Typography>
                </li>
              ))}
            </ul>
            <Typography variant="meta">{t('dubbing.protected')}</Typography>
          </div>
        ) : (
          proposal && (
            <Typography variant="body">
              {t(`dubbing.retimingReasons.${proposal.reason}`)}
            </Typography>
          )
        )}
      </Sheet>
    </div>
  )
}
