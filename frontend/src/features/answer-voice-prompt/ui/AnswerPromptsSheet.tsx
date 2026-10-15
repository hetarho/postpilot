import { useId, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { VoiceProfile, VoiceSample } from '@/entities/voice'
import { Button, Sheet, Typography } from '@/shared/ui'
import { VoiceQuestionnaire } from './VoiceQuestionnaire'

export { AnswerForm } from './AnswerForm'

export function AnswerPromptsSheet({
  ownerId,
  voiceId,
  samples,
  profile,
  renderMakeVoice,
  disabled = false,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  profile: VoiceProfile
  renderMakeVoice: (onStarted: () => void) => ReactNode
  disabled?: boolean
}) {
  const { t } = useTranslation('voices')
  const [open, setOpen] = useState(false)
  const titleId = useId()
  const busy = useRef(false)
  const close = () => {
    if (!busy.current) setOpen(false)
  }
  return (
    <>
      <Button variant="secondary" disabled={disabled} onClick={() => setOpen(true)}>
        {t('prompts.open')}
      </Button>
      {open && (
        <Sheet open labelledBy={titleId} onClose={close}>
          <Typography variant="title" as="h2" id={titleId}>
            {t('prompts.title')}
          </Typography>
          <div className="mt-6">
            <VoiceQuestionnaire
              ownerId={ownerId}
              voiceId={voiceId}
              samples={samples}
              profile={profile}
              renderMakeVoice={renderMakeVoice}
              onClose={close}
              onBusyChange={(pending) => {
                busy.current = pending
              }}
            />
          </div>
        </Sheet>
      )}
    </>
  )
}
