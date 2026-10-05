import { useReducer, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  clipDraftKey,
  clipTimelineReducer,
  createClipTimeline,
  spokenScriptValid,
  useClipSpeechCalls,
  type ClipNarration,
} from '@/entities/clip-plan'
import { useRefreshClipProjects } from '@/entities/clip-project'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import { appFailureFromConnect } from '@/shared/api'
import { ClipSpokenProperties } from './ClipSpokenProperties'
export function ClipSpokenRecoveryEditor({
  ownerId,
  projectId,
  durationMs,
  disabled,
}: {
  ownerId: string
  projectId: string
  durationMs: number
  disabled?: boolean
}) {
  const calls = useClipSpeechCalls(),
    query = useQuery({
      queryKey: ['clip-spoken-recovery', ownerId, projectId],
      queryFn: () => calls.recovery(projectId),
      enabled: !!ownerId,
      refetchOnWindowFocus: false,
    })
  return query.data?.narration ? (
    <RecoveryEditor
      key={query.data.digest}
      ownerId={ownerId}
      projectId={projectId}
      durationMs={durationMs}
      disabled={disabled}
      digest={query.data.digest}
      narration={query.data.narration}
      updated={() => query.refetch()}
    />
  ) : null
}
function RecoveryEditor({
  ownerId,
  projectId,
  durationMs,
  disabled,
  digest,
  narration,
  updated,
}: {
  ownerId: string
  projectId: string
  durationMs: number
  disabled?: boolean
  digest: string
  narration: ClipNarration
  updated: () => Promise<unknown>
}) {
  const { t } = useTranslation('clips'),
    calls = useClipSpeechCalls(),
    refresh = useRefreshClipProjects(ownerId),
    [timeline, dispatch] = useReducer(
      clipTimelineReducer,
      { durationMs, cuts: [], narration },
      createClipTimeline,
    ),
    [pending, setPending] = useState(false),
    [error, setError] = useState<unknown>()
  const draft = timeline.plan,
    dirty = clipDraftKey(draft) !== clipDraftKey({ durationMs, cuts: [], narration })
  async function save() {
    setPending(true)
    setError(undefined)
    try {
      await calls.saveRecovery(projectId, digest, draft.narration!)
      await refresh.detail(projectId)
      await updated()
    } catch (e) {
      setError(e)
    } finally {
      setPending(false)
    }
  }
  return (
    <details open className="min-w-0 space-y-3">
      <summary className="text-content-primary cursor-pointer">
        {t('dubbing.retainedDraft')}
      </summary>
      <Typography variant="body">{t('dubbing.retainedHelp')}</Typography>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="ghost"
          disabled={disabled || pending || !timeline.past.length}
          onClick={() => dispatch({ type: 'undo' })}
        >
          {t('timeline.undo')}
        </Button>
        <Button
          variant="ghost"
          disabled={disabled || pending || !timeline.future.length}
          onClick={() => dispatch({ type: 'redo' })}
        >
          {t('timeline.redo')}
        </Button>
      </div>
      {draft.narration!.segments.map((s) => (
        <ClipSpokenProperties
          key={s.id}
          ownerId={ownerId}
          projectId={projectId}
          narration={draft.narration!}
          segment={s}
          durationMs={durationMs}
          disabled={disabled || pending}
          change={(edit, group) => dispatch({ type: 'edit', edit, group, at: Date.now() })}
        />
      ))}
      <Button
        variant="secondary"
        disabled={disabled || pending || draft.narration!.segments.length >= 32}
        onClick={() => {
          const startMs = draft.narration!.segments.at(-1)?.endMs ?? 0
          dispatch({
            type: 'edit',
            edit: {
              type: 'addSpoken',
              id: `new-spoken-${crypto.randomUUID()}`,
              startMs,
              endMs: startMs + 2000,
            },
            at: Date.now(),
          })
        }}
      >
        {t('dubbing.add')}
      </Button>
      {!spokenScriptValid(draft.narration) && (
        <Typography variant="meta" role="alert">
          {t('dubbing.limits')}
        </Typography>
      )}
      {error !== undefined && <AppFailureMessage failure={appFailureFromConnect(error)} />}
      <Button
        variant="cta"
        disabled={disabled || !dirty || !spokenScriptValid(draft.narration)}
        pending={pending}
        onClick={() => void save()}
      >
        {t('dubbing.saveRetained')}
      </Button>
    </details>
  )
}
