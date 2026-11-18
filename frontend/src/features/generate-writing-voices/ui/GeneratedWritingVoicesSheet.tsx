import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'
import { Button, Sheet } from '@/shared/ui'
import { GeneratedWritingVoices } from './GeneratedWritingVoices'

/** Opening this settings surface only reads durable work. Generation stays explicit. */
/** `open` / `onOpenChange` make the sheet controlled: a surface that opens it from its own menu
 *  passes both and the sheet draws no trigger of its own (as `CreateVoiceSheet` does). */
export function GeneratedWritingVoicesSheet({
  ownerId,
  open,
  onOpenChange,
}: {
  ownerId: string
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  return <AccountSheet key={ownerId} ownerId={ownerId} open={open} onOpenChange={onOpenChange} />
}

function AccountSheet({
  ownerId,
  open: controlledOpen,
  onOpenChange,
}: {
  ownerId: string
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const { t } = useTranslation('voices')
  const navigate = useNavigate()
  const [ownOpen, setOwnOpen] = useState(false)
  const controlled = controlledOpen !== undefined
  const open = controlledOpen ?? ownOpen
  const setOpen = onOpenChange ?? setOwnOpen
  return (
    <>
      {!controlled && (
        <Button variant="secondary" onClick={() => setOpen(true)}>
          <Sparkles aria-hidden="true" className="size-4 shrink-0" />
          {t('candidateFlow.open')}
        </Button>
      )}
      {open && (
        <Sheet
          open
          label={t('candidateFlow.title')}
          size="wide"
          onClose={() => setOpen(false)}
          header={
            <div className="flex justify-end">
              <Button variant="ghost" onClick={() => setOpen(false)}>
                {t('candidateFlow.dismiss')}
              </Button>
            </div>
          }
        >
          <GeneratedWritingVoices
            ownerId={ownerId}
            onAdopted={(voice) => {
              setOpen(false)
              void navigate({ to: '/voices/$voiceId', params: { voiceId: voice.id } })
            }}
          />
        </Sheet>
      )}
    </>
  )
}
