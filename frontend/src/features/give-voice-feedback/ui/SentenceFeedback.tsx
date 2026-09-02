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
import { clsx } from 'clsx'
import {
  AppFailureMessage,
  Button,
  Dialog,
  FieldLabel,
  Listbox,
  Notice,
  Typography,
} from '@/shared/ui'

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
  // Deduplicated: the source is the whole finalized post now, so a repeated sentence would
  // otherwise be two rows carrying the same choice — and the server stores one record per
  // sentence text either way.
  const sentences = [
    ...new Set(
      text
        .split(/(?<=[.!?])\s+|\n+/)
        .map((value) => value.trim())
        .filter(Boolean),
    ),
  ]
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
          {/* `min-w-0` on the group, and rows that WRAP. The sentence used to be chosen from a
              `Listbox`, whose trigger is `whitespace-nowrap`: with a whole sentence as its label
              the grid item's automatic minimum resolved to that sentence's full width and the
              sheet gained a long horizontal scroll. A vertical list of full-width rows is the
              actual fix, and `min-w-0` keeps any future long child from doing it again.
              It is a RADIOGROUP, not a pile of toggles: the control it replaced was a combobox,
              so the select-one semantics and the single tab stop have to survive the shape
              change. Only the chosen row is tabbable and the arrows move between them — the
              WAI-APG radio-group pattern, where focus follows selection. */}
          <div
            role="radiogroup"
            aria-labelledby="feedback-sentence-label"
            className="min-w-0"
            onKeyDown={(event) => {
              const step =
                event.key === 'ArrowDown' || event.key === 'ArrowRight'
                  ? 1
                  : event.key === 'ArrowUp' || event.key === 'ArrowLeft'
                    ? -1
                    : 0
              if (step === 0) return
              event.preventDefault()
              const at = sentences.indexOf(selected)
              const next = (at + step + sentences.length) % sentences.length
              setSentence(sentences[next])
              event.currentTarget
                .querySelectorAll<HTMLButtonElement>('[role="radio"]')
                [next]?.focus()
            }}
          >
            <FieldLabel id="feedback-sentence-label">{t('feedback.sentence')}</FieldLabel>
            <div className="mt-1 grid min-w-0 gap-1">
              {sentences.map((value) => (
                <button
                  key={value}
                  type="button"
                  role="radio"
                  aria-checked={value === selected}
                  tabIndex={value === selected ? 0 : -1}
                  onClick={() => setSentence(value)}
                  className={clsx(
                    'flex min-h-11 w-full min-w-0 items-center rounded-md px-3 py-2 text-left transition-colors',
                    value === selected
                      ? 'bg-row-bg-active text-content-primary'
                      : 'text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active',
                  )}
                >
                  <Typography variant="body" as="span" className="min-w-0 break-words">
                    {value}
                  </Typography>
                </button>
              ))}
            </div>
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
          {/* What this DOES, where a statement of what it does not do used to stand. Leaving
              feedback teaches the voice and changes nothing about the post in front of the user,
              so the surface says so and names the thing that does change the post. */}
          <div className="grid gap-1">
            <Typography variant="body" as="p">
              {t('feedback.purpose')}
            </Typography>
            <Typography variant="body" as="p" className="text-content-secondary">
              {t('feedback.changePost')}
            </Typography>
          </div>
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
