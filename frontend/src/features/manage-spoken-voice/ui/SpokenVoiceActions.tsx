import { useId, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSpokenActions, SPOKEN_NAME_MAX, type SpokenVoice } from '@/entities/spoken-voice'
import { Button, Dialog, FieldLabel, Sheet, TextField, Typography, buttonStyles } from '@/shared/ui'
export function SpokenVoiceActions({
  ownerId,
  voice,
  onStatus,
}: {
  ownerId: string
  voice: SpokenVoice
  onStatus: (key: 'failed' | 'renamed') => void
}) {
  const { t } = useTranslation('manageSpokenVoice'),
    id = useId(),
    actions = useSpokenActions(ownerId)
  const [renaming, setRenaming] = useState(false),
    [removing, setRemoving] = useState(false),
    [failed, setFailed] = useState(false),
    [name, setName] = useState(voice.name)
  async function rename() {
    setFailed(false)
    try {
      await actions.rename(voice.id, voice.revision, name, crypto.randomUUID())
      setRenaming(false)
      onStatus('renamed')
    } catch {
      setFailed(true)
      onStatus('failed')
    }
  }
  async function remove() {
    setFailed(false)
    try {
      await actions.remove(voice.id, voice.revision, crypto.randomUUID())
      setRemoving(false)
    } catch {
      setFailed(true)
      onStatus('failed')
    }
  }
  return (
    <>
      <div className="flex min-w-0 flex-wrap gap-3">
        <Button
          variant="ghost"
          onClick={() => {
            setName(voice.name)
            setFailed(false)
            setRenaming(true)
          }}
        >
          {t('rename')}
        </Button>
        <Link
          to="/spoken-voices/new"
          search={{ copy: voice.id }}
          className={buttonStyles({ variant: 'ghost' })}
        >
          {t('changeSound')}
        </Link>
        <Button
          variant="ghost"
          onClick={() => {
            setFailed(false)
            setRemoving(true)
          }}
        >
          {t('remove')}
        </Button>
      </div>
      <Sheet
        open={renaming}
        onClose={() => setRenaming(false)}
        labelledBy={`${id}-title`}
        header={
          <Typography id={`${id}-title`} variant="title">
            {t('rename')}
          </Typography>
        }
        footer={
          <div className="mt-6 flex flex-wrap gap-3">
            <Button variant="ghost" disabled={actions.pending} onClick={() => setRenaming(false)}>
              {t('cancel')}
            </Button>
            <Button
              variant="cta"
              disabled={!name.trim() || [...name.trim()].length > SPOKEN_NAME_MAX}
              pending={actions.pending}
              onClick={() => void rename()}
            >
              {t('save')}
            </Button>
          </div>
        }
      >
        {failed && (
          <Typography variant="body" className="mb-3">
            {t('failed')}
          </Typography>
        )}
        <FieldLabel htmlFor={`${id}-name`}>{t('name')}</FieldLabel>
        <TextField
          id={`${id}-name`}
          value={name}
          maxLength={SPOKEN_NAME_MAX}
          disabled={actions.pending}
          onChange={(e) => setName(e.target.value)}
          autoComplete="off"
        />
      </Sheet>
      <Dialog
        open={removing}
        onClose={() => setRemoving(false)}
        title={t('removeTitle', { name: voice.name })}
        confirmLabel={t('remove')}
        pending={actions.pending}
        onConfirm={() => void remove()}
      >
        <Typography variant="body">{t('removeEffect')}</Typography>
        {failed && (
          <Typography variant="body" className="mt-3">
            {t('failed')}
          </Typography>
        )}
      </Dialog>
    </>
  )
}
