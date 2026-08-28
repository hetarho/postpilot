import { type VoiceSample, useDeleteVoiceSample } from '@/entities/voice-profile'
import { formatRelativeTime } from '@/shared/lib'
import { Button, FieldMessage } from '@/shared/ui'

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

  const remove = async (sample: VoiceSample) => {
    if (!window.confirm(`"${sample.label}"을(를) 삭제할까요?`)) return
    try {
      const response = await removeSample.remove(sample.id)
      if (response.jobId) onAnalysisStarted(response.jobId)
    } catch {
      // The mutation error is rendered below.
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
                onClick={() => void remove(sample)}
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
    </section>
  )
}
