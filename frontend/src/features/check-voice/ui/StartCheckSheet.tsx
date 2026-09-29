import { useId, useState, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  useStartVoiceCheck,
  useVoicePrompts,
  type VoicePrompt,
  type VoiceSample,
} from '@/entities/voice'
import { Badge, Button, FieldMessage, Sheet, Typography, typographyStyles } from '@/shared/ui'
import { useWriteSelection } from '../model/useWriteSelection'

/** `검증하기` and its sheet (VOICE-43): the voice's prompts, answered ones first. An answered one
 *  starts the check at once; an unanswered one opens the answer form first — the answer becomes a
 *  학습 글 too — and the check starts once it is saved. A photo prompt is listed disabled, with
 *  the reason, while the write model does not read images. */
export function StartCheckSheet({
  ownerId,
  voiceId,
  samples,
  blocked,
  busy,
  renderAnswer,
  onStarted,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  /** Why 검증 cannot start at all (a deleted or unmade voice), said in place; '' when it can. */
  blocked: string
  /** A 검증 of this voice is already running. */
  busy: boolean
  /** The answer form for a prompt not yet answered; `onAnswered` runs once it is saved. */
  renderAnswer: (prompt: VoicePrompt, onAnswered: () => void, onBack: () => void) => ReactNode
  onStarted: (jobId: string) => void
}) {
  const { t } = useTranslation('voices')
  const reasonId = useId()
  const [open, setOpen] = useState(false)
  const write = useWriteSelection()
  const reason = blocked || (write.missing ? t('check.noModel') : '')
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <Button
        variant="cta"
        disabled={reason !== '' || write.isPending || busy}
        aria-describedby={reason ? reasonId : undefined}
        onClick={() => setOpen(true)}
      >
        {t('check.open')}
      </Button>
      {reason && (
        <Typography variant="label" as="p" id={reasonId} className="min-w-0">
          {reason}{' '}
          {!blocked && write.missing && (
            <Link
              to="/ai-models"
              className={typographyStyles({
                variant: 'label',
                className: 'text-link-fg hover:text-link-fg-hover underline',
              })}
            >
              {t('check.chooseModel')}
            </Link>
          )}
        </Typography>
      )}
      {open && write.selected && (
        <PromptsPanel
          ownerId={ownerId}
          voiceId={voiceId}
          samples={samples}
          model={write.selected}
          vision={write.vision}
          renderAnswer={renderAnswer}
          onClose={() => setOpen(false)}
          onStarted={(jobId) => {
            setOpen(false)
            onStarted(jobId)
          }}
        />
      )}
    </div>
  )
}

function PromptsPanel({
  ownerId,
  voiceId,
  samples,
  model,
  vision,
  renderAnswer,
  onClose,
  onStarted,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  model: { providerId: string; modelId: string }
  vision: boolean
  renderAnswer: (prompt: VoicePrompt, onAnswered: () => void, onBack: () => void) => ReactNode
  onClose: () => void
  onStarted: (jobId: string) => void
}) {
  const { t } = useTranslation('voices')
  const titleId = useId()
  const { prompts, isPending, isError } = useVoicePrompts()
  const check = useStartVoiceCheck(ownerId, voiceId)
  const [answering, setAnswering] = useState<VoicePrompt | null>(null)
  const answered = new Set(
    samples.filter((sample) => sample.kind === 'answer').map((sample) => sample.promptKey),
  )
  // Answered first, each group in the shared order.
  const ordered = [
    ...prompts.filter((prompt) => answered.has(prompt.key)),
    ...prompts.filter((prompt) => !answered.has(prompt.key)),
  ]

  const start = async (prompt: VoicePrompt) => {
    try {
      const { jobId } = await check.start(prompt.key, model)
      onStarted(jobId)
    } catch {
      // The mutation's message renders in the sheet.
      setAnswering(null)
    }
  }

  return (
    <Sheet open labelledBy={titleId} onClose={onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('check.title')}
      </Typography>
      {check.isError && <FieldMessage className="mt-4">{check.errorMessage}</FieldMessage>}
      {answering ? (
        renderAnswer(
          answering,
          () => void start(answering),
          () => setAnswering(null),
        )
      ) : isError ? (
        <FieldMessage className="mt-4">{t('check.loadFailed')}</FieldMessage>
      ) : isPending ? null : (
        <ul className="divide-divider mt-4 divide-y">
          {ordered.map((prompt) => {
            const done = answered.has(prompt.key)
            const unreadable = prompt.photo && !vision
            const noteId = `${titleId}-${prompt.key}`
            return (
              <li key={prompt.key}>
                <button
                  type="button"
                  disabled={unreadable || check.isPending}
                  aria-describedby={unreadable ? noteId : undefined}
                  onClick={() => (done ? void start(prompt) : setAnswering(prompt))}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active disabled:text-content-tertiary flex min-h-11 w-full flex-wrap items-center gap-2 py-3 text-left"
                >
                  <Typography variant="body" as="span" className="min-w-0 flex-1">
                    {prompt.text}
                  </Typography>
                  {prompt.photo && <Badge>{t('check.photo')}</Badge>}
                  {done && <Badge tone="accent">{t('check.answered')}</Badge>}
                  {unreadable && (
                    <Typography variant="label" as="span" id={noteId} className="w-full">
                      {t('check.photoNeedsVision')}
                    </Typography>
                  )}
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </Sheet>
  )
}
