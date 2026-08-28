import { useId, useState, type FormEvent } from 'react'
import { useStageSelection } from '@/entities/model-catalog'
import { type VoiceProfile, useAddVoiceSample } from '@/entities/voice-profile'
import { VOICE_SAMPLE_MIN_CHARS } from '@/shared/config'
import { Button, FieldLabel, FieldMessage, Textarea, TextField } from '@/shared/ui'

interface LearnVoiceFormProps {
  ownerId: string
  profile: VoiceProfile
  onStarted: (jobId: string) => void
}

export function LearnVoiceForm({ ownerId, profile, onStarted }: LearnVoiceFormProps) {
  const labelId = useId()
  const bodyId = useId()
  const bodyErrorId = `${bodyId}-error`
  const [label, setLabel] = useState('')
  const [body, setBody] = useState('')
  const { selected, isPending: modelPending } = useStageSelection('analyze')
  const addSample = useAddVoiceSample(ownerId)
  const chars = Array.from(body.trim()).length
  const tooShort = chars < VOICE_SAMPLE_MIN_CHARS
  const disabled = tooShort || !selected || modelPending || addSample.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled || !selected) return
    if (
      profile.styleguide.trim() !== '' &&
      !window.confirm('재분석하면 현재 문체 규칙을 덮어씁니다')
    ) {
      return
    }
    const submittedLabel = label
    const submittedBody = body
    try {
      const response = await addSample.add(submittedLabel, submittedBody, selected)
      setLabel((current) => (current === submittedLabel ? '' : current))
      setBody((current) => (current === submittedBody ? '' : current))
      onStarted(response.jobId)
    } catch {
      // The mutation exposes the server's raw message below.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className="mt-5 space-y-4">
      <div>
        <FieldLabel htmlFor={labelId}>라벨 (선택)</FieldLabel>
        <TextField
          id={labelId}
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          placeholder="예: 제주 여행기"
          className="mt-1"
        />
      </div>
      <div>
        <span className="flex items-center justify-between gap-3">
          <FieldLabel htmlFor={bodyId}>내가 쓴 글</FieldLabel>
          <span className="text-content-tertiary text-xs" aria-live="polite">
            {chars} / {VOICE_SAMPLE_MIN_CHARS}자
          </span>
        </span>
        <Textarea
          id={bodyId}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          placeholder="기존에 쓴 글 한 편을 붙여 넣어 주세요"
          rows={10}
          aria-invalid={addSample.isError || undefined}
          aria-describedby={addSample.isError ? bodyErrorId : undefined}
          className="mt-1 leading-relaxed"
        />
      </div>

      {!modelPending && !selected && (
        <p
          role="status"
          className="bg-notice-warning-bg text-notice-warning-fg rounded-md px-3 py-2 text-sm"
        >
          모델을 선택하세요
        </p>
      )}
      {addSample.isError && <FieldMessage id={bodyErrorId}>{addSample.errorMessage}</FieldMessage>}

      <Button type="submit" variant="cta" disabled={disabled}>
        {addSample.isPending ? '학습을 시작하는 중…' : '학습'}
      </Button>
      <span role="status" className="sr-only">
        {addSample.isPending ? '학습을 시작하는 중' : ''}
      </span>
    </form>
  )
}
