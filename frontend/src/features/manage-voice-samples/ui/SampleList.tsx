import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { type VoiceSample, useDeleteVoiceSample } from '@/entities/voice'
import { formatNumber, formatRelativeTime } from '@/shared/lib'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

export function SampleList({
  ownerId,
  voiceId,
  samples,
  onAnalysisStarted,
  blocked = false,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  onAnalysisStarted: (jobId: string) => void
  blocked?: boolean
}) {
  const { t } = useTranslation(['voices', 'common'])
  const removeSample = useDeleteVoiceSample(ownerId, voiceId)
  // The sample the confirmation sheet is open for. `window.confirm` is not an option for a
  // delete the user repeats while pruning a profile: mobile Chrome and Safari offer to suppress
  // further dialogs on the page, after which every confirm returns false and 삭제 becomes a
  // silent no-op (§7).
  const [confirming, setConfirming] = useState<VoiceSample | null>(null)

  const remove = async (sample: VoiceSample) => {
    try {
      const response = await removeSample.remove(sample.id)
      if (response.jobId) onAnalysisStarted(response.jobId)
    } catch {
      // The mutation error is rendered below.
    } finally {
      // Closed on failure too, so the message under the list is not left behind the scrim.
      setConfirming(null)
    }
  }

  return (
    <section>
      <h2 className="text-lg font-semibold tracking-tight">
        {t('samples.title', { ns: 'voices' })}
      </h2>
      {samples.length === 0 ? (
        <p className="text-content-tertiary mt-4 text-sm">{t('samples.empty', { ns: 'voices' })}</p>
      ) : (
        <ul className="divide-divider mt-3 divide-y">
          {samples.map((sample) => (
            <li key={sample.id} className="flex min-h-14 items-center gap-3 py-2">
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm">{sample.label}</span>
                <span className="text-content-tertiary mt-1 block text-xs">
                  {t('samples.meta', {
                    ns: 'voices',
                    count: sample.chars,
                    characters: formatNumber(sample.chars),
                    time: formatRelativeTime(sample.createdAt),
                  })}
                </span>
              </span>
              <Button
                variant="danger"
                onClick={() => setConfirming(sample)}
                disabled={blocked || removeSample.isPending}
                aria-label={t('samples.deleteAria', { ns: 'voices', label: sample.label })}
              >
                {t('action.delete', { ns: 'common' })}
              </Button>
            </li>
          ))}
        </ul>
      )}
      {removeSample.isError && (
        <FieldMessage className="mt-2">{removeSample.errorMessage}</FieldMessage>
      )}
      <Dialog
        open={confirming !== null}
        title={t('samples.deleteTitle', { ns: 'voices' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={removeSample.isPending}
        onClose={() => setConfirming(null)}
        onConfirm={() => {
          if (confirming) void remove(confirming)
        }}
      >
        {/* The label is stored text the user pasted, so it breaks inside the sheet rather than
            widening it (§3.2). */}
        {t('samples.deleteDescription', { ns: 'voices', label: confirming?.label ?? '' })}
      </Dialog>
    </section>
  )
}
