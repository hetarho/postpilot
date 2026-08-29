import { useState } from 'react'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { VoiceProfile, VoiceVersion } from '@/entities/voice-profile'
import { voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice-profile'
import { VoiceService } from '@/shared/api'
import { Button, Dialog } from '@/shared/ui'

/** The version list and its restore confirmation. Still "edit the voice profile" — restoring
 *  publishes a new head exactly as an override does — but its own screen, so the profile tab
 *  mounts one thing and this tab mounts the other. */
export function VoiceVersionHistory({
  ownerId,
  profile,
  versions,
}: {
  ownerId: string
  profile: VoiceProfile
  versions: VoiceVersion[]
}) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const restore = useMutation(VoiceService.method.restoreVoiceProfile)
  const [restoreVersion, setRestoreVersion] = useState<bigint>()
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) })
    void queryClient.invalidateQueries({ queryKey: voiceVersionsQueryKey(transport, ownerId) })
  }
  return (
    <section aria-label="버전 기록">
      {versions.length === 0 ? (
        <p className="text-content-tertiary text-sm">아직 저장된 버전이 없어요.</p>
      ) : (
        <ul className="divide-divider divide-y">
          {versions.map((version) => (
            <li
              key={version.version.toString()}
              className="flex min-h-14 items-center justify-between gap-3 py-2"
            >
              <span className="min-w-0 text-sm break-words">
                v{version.version.toString()} · {originLabel(version.origin)}
                {version.restoredFromVersion > 0n
                  ? ` (v${version.restoredFromVersion.toString()}에서 복원)`
                  : ''}
              </span>
              <Button
                variant="ghost"
                disabled={version.version === profile.structured.version}
                onClick={() => setRestoreVersion(version.version)}
              >
                복원
              </Button>
            </li>
          ))}
        </ul>
      )}
      <Dialog
        open={restoreVersion !== undefined}
        title="이 버전으로 복원할까요?"
        confirmLabel="새 버전으로 복원"
        pending={restore.isPending}
        onClose={() => setRestoreVersion(undefined)}
        onConfirm={() =>
          restoreVersion !== undefined &&
          void restore.mutateAsync({ version: restoreVersion }).then(() => {
            setRestoreVersion(undefined)
            refresh()
          })
        }
      >
        기존 기록은 지우지 않고, 선택한 스냅샷을 새 현재 버전으로 만듭니다.
      </Dialog>
    </section>
  )
}

const originLabel = (origin: string) =>
  ({ analysis: '분석', manual: '직접 수정', restore: '복원', rule: '규칙 반영', confirmation: '충돌 해결' })[
    origin
  ] ?? origin
