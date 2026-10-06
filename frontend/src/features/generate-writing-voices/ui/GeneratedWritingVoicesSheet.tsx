import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'
import { Button, Sheet } from '@/shared/ui'
import { GeneratedWritingVoices } from './GeneratedWritingVoices'

/** Opening this settings surface only reads durable work. Generation stays explicit. */
export function GeneratedWritingVoicesSheet({ ownerId }: { ownerId: string }) {
  return <AccountSheet key={ownerId} ownerId={ownerId} />
}

function AccountSheet({ ownerId }: { ownerId: string }) {
  const { t } = useTranslation('voices')
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        <Sparkles aria-hidden="true" className="size-4 shrink-0" />
        {t('candidateFlow.open')}
      </Button>
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
