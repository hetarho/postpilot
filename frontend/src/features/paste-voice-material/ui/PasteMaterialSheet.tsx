import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Sheet, Typography } from '@/shared/ui'
import { PasteMaterialForm } from './PasteMaterialForm'
/** Optional further learning; the focused first collection uses the same form inline. */
export function PasteMaterialSheet({
  ownerId,
  voiceId,
  disabled = false,
}: {
  ownerId: string
  voiceId: string
  disabled?: boolean
}) {
  const { t } = useTranslation(['voices', 'common'])
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false),
    [added, setAdded] = useState(false)
  return (
    <>
      <Button
        variant="secondary"
        disabled={disabled}
        onClick={() => {
          setAdded(false)
          setOpen(true)
        }}
      >
        {t('paste.open')}
      </Button>
      {open && (
        <Sheet
          open
          label={t('paste.title')}
          onClose={() => {
            if (!busy) setOpen(false)
          }}
        >
          <Typography variant="title" as="h2">
            {t('paste.title')}
          </Typography>
          <div className="mt-4">
            <PasteMaterialForm
              ownerId={ownerId}
              voiceId={voiceId}
              onBusyChange={setBusy}
              onSaved={() => setAdded(true)}
              focusAfterSave
              onBack={added ? undefined : () => setOpen(false)}
            />
          </div>
          {added && (
            <Button variant="ghost" disabled={busy} className="mt-4" onClick={() => setOpen(false)}>
              {t('action.close', { ns: 'common' })}
            </Button>
          )}
        </Sheet>
      )}
    </>
  )
}
