import { useRef, useState } from 'react'
import { useMutation } from '@connectrpc/connect-query'
import { VoiceFeedbackReason, VoiceLearningService } from '@/shared/api'
import { Button, Dialog, FieldLabel, Select } from '@/shared/ui'

export function SentenceFeedback({
  postSlug,
  text,
  beforeSubmit,
}: {
  postSlug: string
  text: string
  beforeSubmit: () => Promise<void>
}) {
  const sentences = text
    .split(/(?<=[.!?])\s+|\n+/)
    .map((value) => value.trim())
    .filter(Boolean)
  const [open, setOpen] = useState(false)
  const [sentence, setSentence] = useState(sentences[0] ?? '')
  const [reason, setReason] = useState(VoiceFeedbackReason.VOCABULARY)
  const timer = useRef<number | undefined>(undefined)
  const mutation = useMutation(VoiceLearningService.method.giveSentenceFeedback)
  if (sentences.length === 0) return null
  const submit = async () => {
    await beforeSubmit()
    await mutation.mutateAsync({ postSlug, sentenceRef: sentence, authoredText: sentence, reason })
    setOpen(false)
  }
  return (
    <>
      <Button
        variant="ghost"
        className="mt-2"
        onClick={() => setOpen(true)}
        onPointerDown={() => {
          timer.current = window.setTimeout(() => setOpen(true), 600)
        }}
        onPointerUp={() => window.clearTimeout(timer.current)}
        onPointerCancel={() => window.clearTimeout(timer.current)}
      >
        문장 의견
      </Button>
      <Dialog
        open={open}
        title="어떤 점을 바꾸고 싶나요?"
        confirmLabel="의견 남기기"
        pending={mutation.isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void submit()}
      >
        <div className="grid gap-4">
          <div>
            <FieldLabel htmlFor="feedback-sentence">문장</FieldLabel>
            <Select
              id="feedback-sentence"
              value={sentence}
              onChange={(event) => setSentence(event.target.value)}
              className="mt-1"
            >
              {sentences.map((value, index) => (
                <option key={`${index}-${value}`} value={value}>
                  {value}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <FieldLabel htmlFor="feedback-reason">이유</FieldLabel>
            <Select
              id="feedback-reason"
              value={reason}
              onChange={(event) => setReason(Number(event.target.value) as VoiceFeedbackReason)}
              className="mt-1"
            >
              <option value={VoiceFeedbackReason.VOCABULARY}>단어 선택</option>
              <option value={VoiceFeedbackReason.ENDING}>종결어미</option>
              <option value={VoiceFeedbackReason.LENGTH}>문장 길이</option>
              <option value={VoiceFeedbackReason.STRUCTURE}>구조</option>
            </Select>
          </div>
          <p>이 반응만으로 새 규칙이 생기거나 활성화되지는 않습니다.</p>
        </div>
      </Dialog>
    </>
  )
}
