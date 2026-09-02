import i18next from 'i18next'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { VoiceProfile, VoiceVersion } from '@/entities/voice'
import { voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, Notice, Typography } from '@/shared/ui'

/** The version list and its restore confirmation. Still "edit the voice profile" — restoring
 *  publishes a new head exactly as an override does — but its own screen, so the profile tab
 *  mounts one thing and this tab mounts the other. */
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
  const [restoreVersion, setRestoreVersion] = useState<bigint>()
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
          {versions.map((version) => (
            <li
              key={version.version.toString()}
              className="flex min-h-14 items-center justify-between gap-3 py-2"
            >
              <Typography variant="body" as="span" className="min-w-0 break-words">
                v{version.version.toString()} · {originLabel(version.origin)}
                {version.restoredFromVersion > 0n
                  ? t('versions.restoredFrom', {
                      ns: 'voices',
                      version: version.restoredFromVersion.toString(),
                    })
                  : ''}
              </Typography>
              <Button
                variant="ghost"
                disabled={readOnly || version.version === profile.structured.version}
                onClick={() => setRestoreVersion(version.version)}
              >
                {t('action.restore', { ns: 'common' })}
              </Button>
            </li>
          ))}
        </ul>
      )}
      {restore.error && (
        <Notice tone="danger" role="alert" className="mt-3">
          <AppFailureMessage failure={appFailureFromConnect(restore.error)} />
        </Notice>
      )}
      <Dialog
        open={restoreVersion !== undefined}
        title={t('versions.restoreTitle', { ns: 'voices' })}
        confirmLabel={t('versions.restoreConfirm', { ns: 'voices' })}
        pending={restore.isPending}
        onClose={() => setRestoreVersion(undefined)}
        onConfirm={() =>
          restoreVersion !== undefined &&
          void restore
            .mutateAsync({ voiceId, version: restoreVersion })
            .then(() => {
              setRestoreVersion(undefined)
              refresh()
            })
            .catch(() => undefined)
        }
      >
        {t('versions.restoreDescription', { ns: 'voices' })}
      </Dialog>
    </section>
  )
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
