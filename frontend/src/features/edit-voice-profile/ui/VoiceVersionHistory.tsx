import i18next from 'i18next'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ChevronDown } from 'lucide-react'
import { clsx } from 'clsx'
import type { PostContent } from '@/shared/api'
import { formatDate } from '@/shared/lib'
import type { VoiceProfile, VoiceVersion } from '@/entities/voice'
import {
  useVoiceVersionSample,
  voiceProfileQueryKey,
  voiceVersionsQueryKey,
} from '@/entities/voice'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { VOICE_VERSION_PREVIEW_CHARS } from '@/shared/config'
import { AppFailureMessage, Button, Notice, Spinner, Typography } from '@/shared/ui'

/** The version list, as a list of OPENABLE rows.
 *
 *  A row used to name a version's origin and nothing about its content, so 복원 was a blind
 *  choice behind a confirmation dialog: the user pressed 복원, read a sentence about snapshots,
 *  and confirmed without ever seeing what the version would make the AI write. Now opening a
 *  version shows the raw AI output of the last post it produced, and `이 버전으로 변경` lives
 *  inside that surface — the preview IS the confirmation, so the dialog is gone (change 16).
 *
 *  Adopting a version still publishes a NEW head and destroys no history, exactly as 복원 did. */
export function VoiceVersionHistory({
  ownerId,
  voiceId,
  profile,
  versions,
  readOnly = false,
}: {
  ownerId: string
  voiceId: string
  profile: VoiceProfile
  versions: VoiceVersion[]
  readOnly?: boolean
}) {
  const { t } = useTranslation(['voices', 'common'])
  const transport = useTransport()
  const queryClient = useQueryClient()
  const restore = useMutation(VoiceService.method.restoreVoiceProfile)
  const [openVersion, setOpenVersion] = useState<bigint>()
  const refresh = () => {
    void queryClient.invalidateQueries({
      queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
    })
    void queryClient.invalidateQueries({
      queryKey: voiceVersionsQueryKey(transport, ownerId, voiceId),
    })
  }
  return (
    <section aria-label={t('versions.title', { ns: 'voices' })}>
      {versions.length === 0 ? (
        <Typography variant="body" className="text-content-tertiary">
          {t('versions.empty', { ns: 'voices' })}
        </Typography>
      ) : (
        <ul className="divide-divider divide-y">
          {versions.map((version) => {
            const open = openVersion === version.version
            const isHead = version.version === profile.structured.version
            return (
              <li key={version.version.toString()} className="py-2">
                <button
                  type="button"
                  aria-expanded={open}
                  onClick={() => setOpenVersion(open ? undefined : version.version)}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 w-full items-center gap-3 rounded-md px-2 text-left transition-colors"
                >
                  <Typography variant="body" as="span" className="min-w-0 flex-1 break-words">
                    v{version.version.toString()} · {originLabel(version.origin)}
                    {version.restoredFromVersion > 0n
                      ? t('versions.restoredFrom', {
                          ns: 'voices',
                          version: version.restoredFromVersion.toString(),
                        })
                      : ''}
                  </Typography>
                  {isHead && (
                    <Typography variant="meta" as="span" className="shrink-0">
                      {t('versions.current', { ns: 'voices' })}
                    </Typography>
                  )}
                  <ChevronDown
                    aria-hidden="true"
                    className={clsx(
                      'text-content-tertiary size-4 shrink-0 transition-transform',
                      open && 'rotate-180',
                    )}
                  />
                </button>
                {open && (
                  <VersionPreview
                    ownerId={ownerId}
                    voiceId={voiceId}
                    version={version}
                    // The head is openable and offers no way to adopt itself; a deleted voice's
                    // versions are readable and nothing more.
                    canAdopt={!readOnly && !isHead}
                    adopting={restore.isPending}
                    onAdopt={() =>
                      void restore
                        .mutateAsync({ voiceId, version: version.version })
                        .then(() => {
                          setOpenVersion(undefined)
                          refresh()
                        })
                        .catch(() => undefined)
                    }
                  />
                )}
              </li>
            )
          })}
        </ul>
      )}
      {restore.error && (
        <Notice tone="danger" role="alert" className="mt-3">
          <AppFailureMessage failure={appFailureFromConnect(restore.error)} />
        </Notice>
      )}
    </section>
  )
}

/** What one version WROTE, plus the way to adopt it. The snapshot is fetched on open and only
 *  for a version that has one, so opening the list costs nothing. */
function VersionPreview({
  ownerId,
  voiceId,
  version,
  canAdopt,
  adopting,
  onAdopt,
}: {
  ownerId: string
  voiceId: string
  version: VoiceVersion
  canAdopt: boolean
  adopting: boolean
  onAdopt: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const [expanded, setExpanded] = useState(false)
  // Asked for on OPEN, not gated on `version.hasSample`: a generation completing elsewhere
  // records a snapshot without invalidating this list, so a cached `hasSample: false` would hide
  // a preview that exists. The flag is a hint for the row, and the response is the truth.
  const { sample, isPending, isError } = useVoiceVersionSample(ownerId, voiceId, version.version)
  const body = sample ? snapshotBody(sample.content) : ''
  const truncated = !expanded && body.length > VOICE_VERSION_PREVIEW_CHARS
  return (
    <div className="mt-2 grid gap-3 px-2">
      <Typography variant="meta" as="p">
        {formatDate(version.createdAt)}
      </Typography>
      {isPending ? (
        <Spinner aria-label={t('state.loading', { ns: 'common' })} />
      ) : isError || !sample ? (
        /* Said plainly, with no empty box: a version that has not produced a post yet is an
           ordinary state, not a failure. */
        <Typography variant="body" className="text-content-tertiary">
          {t('versions.noSample', { ns: 'voices' })}
        </Typography>
      ) : (
        <div className="bg-surface-raised grid gap-2 rounded-lg p-3">
          <Typography variant="meta" as="p">
            {t('versions.sampleTitle', { ns: 'voices' })}
            {sample.createdAt ? ` · ${formatDate(sample.createdAt)}` : ''}
          </Typography>
          <Typography variant="fieldTitle" as="h4" className="break-words">
            {sample.content.title}
          </Typography>
          <Typography variant="body" as="p" className="break-words whitespace-pre-line">
            {truncated ? `${body.slice(0, VOICE_VERSION_PREVIEW_CHARS)}…` : body}
          </Typography>
          {truncated && (
            <Button variant="ghost" size="compact" onClick={() => setExpanded(true)}>
              {t('versions.sampleMore', { ns: 'voices' })}
            </Button>
          )}
        </div>
      )}
      {canAdopt && (
        <Button variant="cta" className="w-full sm:w-auto" pending={adopting} onClick={onAdopt}>
          {t('versions.adopt', { ns: 'voices' })}
        </Button>
      )}
    </div>
  )
}

/** The snapshot's body as plain text. A snapshot has no image rows behind its file references,
 *  so it is READ rather than re-rendered as an article: an image block would resolve to nothing
 *  and a heading would compete with the page's own outline. */
function snapshotBody(content: PostContent): string {
  return content.blocks
    .map((block) => (block.items.length > 0 ? block.items.join('\n') : block.content))
    .filter((value) => value.trim() !== '')
    .join('\n\n')
}

const originLabel = (origin: string) => {
  switch (origin) {
    case 'analysis':
      return i18next.t('versions.reason.analysis', { ns: 'voices' })
    case 'seed':
      return i18next.t('versions.reason.seed', { ns: 'voices' })
    case 'manual':
      return i18next.t('versions.reason.manual', { ns: 'voices' })
    case 'restore':
      return i18next.t('versions.reason.restore', { ns: 'voices' })
    case 'rule':
      return i18next.t('versions.reason.rule', { ns: 'voices' })
    case 'confirmation':
      return i18next.t('versions.reason.confirmation', { ns: 'voices' })
    default:
      return origin
  }
}
