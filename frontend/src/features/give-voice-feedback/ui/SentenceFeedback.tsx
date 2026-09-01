import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation } from '@connectrpc/connect-query'
import { ContentRevisionConflictError } from '@/entities/post'
import {
  appFailureFromConnect,
  type AppFailure,
  VoiceFeedbackReason,
  VoiceLearningService,
} from '@/shared/api'
import { LONG_PRESS_MS } from '@/shared/config'
import { AppFailureMessage, Button, Dialog, FieldLabel, Listbox, Notice } from '@/shared/ui'

export function SentenceFeedback({
  postSlug,
  text,
  beforeSubmit,
}: {
  postSlug: string
  text: string
  beforeSubmit: () => Promise<void>
}) {
  const { t } = useTranslation('voices')
  const sentences = text
    .split(/(?<=[.!?])\s+|\n+/)
    .map((value) => value.trim())
    .filter(Boolean)
  const [open, setOpen] = useState(false)
  // The user's *choice*, not the sentence in play: `text` is live editable block content, so
  // a stored resolved value would name a sentence the post no longer contains and the server
  // would refuse it. Reconciling here rather than remounting on a `key` keeps the reason
  // dropdown and an open dialog alive through every keystroke in the block.
  const [sentence, setSentence] = useState('')
  const selected = sentences.includes(sentence) ? sentence : (sentences[0] ?? '')
  const [reason, setReason] = useState(VoiceFeedbackReason.VOCABULARY)
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  const timer = useRef<number | undefined>(undefined)
  const mutation = useMutation(VoiceLearningService.method.giveSentenceFeedback)
  // A component unmounted mid-press keeps the long-press timeout scheduled. React ignores
  // the resulting setOpen without complaining, which is why it would never be found later.
  useEffect(() => () => window.clearTimeout(timer.current), [])
  if (sentences.length === 0) return null
  const submit = async () => {
    setPrepareFailure(undefined)
    try {
      await beforeSubmit()
    } catch (cause) {
      setPrepareFailure(
        cause instanceof ContentRevisionConflictError
          ? 'content-conflict'
          : appFailureFromConnect(cause),
      )
      return
    }
    try {
      await mutation.mutateAsync({
        postSlug,
        sentenceRef: selected,
        authoredText: selected,
        reason,
      })
      setOpen(false)
    } catch {
      // The stable application failure remains in the dialog next to the action.
    }
  }
  return (
    <>
      <Button
        variant="ghost"
        className="mt-2"
        onClick={() => setOpen(true)}
        onPointerDown={() => {
          timer.current = window.setTimeout(() => setOpen(true), LONG_PRESS_MS)
        }}
        onPointerUp={() => window.clearTimeout(timer.current)}
        onPointerCancel={() => window.clearTimeout(timer.current)}
      >
        {t('feedback.action')}
      </Button>
      <Dialog
        open={open}
        title={t('feedback.title')}
        confirmLabel={t('feedback.confirm')}
        pending={mutation.isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void submit()}
      >
        <div className="grid gap-4">
          <div>
            <FieldLabel id="feedback-sentence-label" htmlFor="feedback-sentence">
              {t('feedback.sentence')}
            </FieldLabel>
            <Listbox
              id="feedback-sentence"
              aria-labelledby="feedback-sentence-label"
              value={selected}
              options={sentences.map((value) => ({ value, label: value }))}
              onChange={setSentence}
              className="mt-1"
            />
          </div>
          <div>
            <FieldLabel id="feedback-reason-label" htmlFor="feedback-reason">
              {t('feedback.reason')}
            </FieldLabel>
            <Listbox<VoiceFeedbackReason>
              id="feedback-reason"
              aria-labelledby="feedback-reason-label"
              value={reason}
              options={[
                { value: VoiceFeedbackReason.VOCABULARY, label: t('feedback.vocabulary') },
                { value: VoiceFeedbackReason.ENDING, label: t('feedback.ending') },
                { value: VoiceFeedbackReason.LENGTH, label: t('feedback.length') },
                { value: VoiceFeedbackReason.STRUCTURE, label: t('feedback.structure') },
              ]}
              onChange={setReason}
              className="mt-1"
            />
          </div>
          <p>{t('feedback.help')}</p>
          {mutation.error && (
            <Notice tone="danger" role="alert">
              <AppFailureMessage failure={appFailureFromConnect(mutation.error)} />
            </Notice>
          )}
          {prepareFailure && (
            <Notice tone="danger" role="alert">
              {prepareFailure === 'content-conflict' ? (
                t('edit.conflict', { ns: 'posts' })
              ) : (
                <AppFailureMessage failure={prepareFailure} />
              )}
            </Notice>
          )}
        </div>
      </Dialog>
    </>
  )
}
