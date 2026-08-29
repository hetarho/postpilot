import { useState } from 'react'
import { type VoiceSample, useDeleteVoiceSample } from '@/entities/voice-profile'
import { formatRelativeTime } from '@/shared/lib'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

export function SampleList({
  ownerId,
  samples,
  onAnalysisStarted,
}: {
  ownerId: string
  samples: readonly VoiceSample[]
  onAnalysisStarted: (jobId: string) => void
}) {
  const removeSample = useDeleteVoiceSample(ownerId)
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
      <h2 className="text-lg font-semibold tracking-tight">학습 샘플</h2>
      {samples.length === 0 ? (
        <p className="text-content-tertiary mt-4 text-sm">아직 학습한 글이 없어요.</p>
      ) : (
        <ul className="divide-divider mt-3 divide-y">
          {samples.map((sample) => (
            <li key={sample.id} className="flex min-h-14 items-center gap-3 py-2">
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm">{sample.label}</span>
                <span className="text-content-tertiary mt-1 block text-xs">
                  {sample.chars}자 · {formatRelativeTime(sample.createdAt)}
                </span>
              </span>
              <Button
                variant="danger"
                onClick={() => setConfirming(sample)}
                disabled={removeSample.isPending}
                aria-label={`${sample.label} 삭제`}
              >
                삭제
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
        title="샘플을 삭제할까요?"
        confirmLabel="삭제"
        pending={removeSample.isPending}
        onClose={() => setConfirming(null)}
        onConfirm={() => {
          if (confirming) void remove(confirming)
        }}
      >
        {/* The label is stored text the user pasted, so it breaks inside the sheet rather than
            widening it (§3.2). */}
        <span className="break-words">"{confirming?.label}"</span>을(를) 지우면 남은 샘플로 문체를
        다시 분석해요.
      </Dialog>
    </section>
  )
}
