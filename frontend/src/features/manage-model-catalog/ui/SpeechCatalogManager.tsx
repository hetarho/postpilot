import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAdminSpeechProfiles, type AdminSpeechProfile } from '@/entities/model-catalog'
import { AppFailureMessage, Button, FieldLabel, Notice, TextField, Typography } from '@/shared/ui'
import { SpeechProfileForm } from './SpeechProfileForm'

export function SpeechCatalogManager() {
  const { t } = useTranslation('models')
  const id = useId()
  const catalog = useAdminSpeechProfiles()
  const [editing, setEditing] = useState<AdminSpeechProfile | null | undefined>()
  const [session, setSession] = useState(0)
  const [budget, setBudget] = useState('')
  const [testProfile, setTestProfile] = useState<AdminSpeechProfile | null>(null)
  const open = (p: AdminSpeechProfile | null) => {
    setEditing(p)
    setSession((s) => s + 1)
  }
  const save = async (p: AdminSpeechProfile) => {
    try {
      await catalog.save(p)
      setEditing(undefined)
    } catch {
      /* The mutation owns the displayed failure. */
    }
  }
  return (
    <div className="mt-6 space-y-5">
      <Typography variant="body">{t('speechAdmin.description')}</Typography>
      <div className="flex flex-wrap gap-3">
        <Button variant="secondary" onClick={() => open(null)}>
          {t('speechAdmin.new')}
        </Button>
        <Button variant="ghost" pending={catalog.refreshing} onClick={catalog.refresh}>
          {t('catalog.refresh')}
        </Button>
      </div>
      {(catalog.isError || catalog.browse.fetchError) && (
        <Notice tone="warning" role="status">
          {t('speechAdmin.fetchFailed')}
        </Notice>
      )}
      {catalog.failure && <AppFailureMessage failure={catalog.failure} />}
      {catalog.isPending && (
        <Typography variant="body" role="status">
          {t('catalog.loading')}
        </Typography>
      )}
      {!catalog.isPending && catalog.browse.profiles.length === 0 && (
        <Typography variant="body">{t('speechAdmin.empty')}</Typography>
      )}
      {catalog.browse.profiles.map((p) => {
        const choice = catalog.browse.choices.find((c) => c.id === p.id)
        return (
          <div key={p.id} className="space-y-2 py-3">
            <Typography variant="fieldTitle">
              {p.label} · {p.grade ? t(`level.${p.grade}`) : t('catalog.levelUnset')}
            </Typography>
            <Typography variant="meta">
              {p.binding.designModel.modelId} → {p.binding.speechModel.modelId} ·{' '}
              {t('speechAdmin.revision', { revision: p.revision.toString() })}
            </Typography>
            <Typography variant="meta">
              {t(p.voiceReady ? 'speechAdmin.voiceReady' : 'speechAdmin.voicePending')} ·{' '}
              {t(p.exportReady ? 'speechAdmin.exportReady' : 'speechAdmin.exportPending')}
            </Typography>
            {choice && !choice.available && (
              <Typography variant="meta">
                {t(`speech.reason.${choice.unavailableReason}`, {
                  defaultValue: t('speech.unavailable'),
                })}
              </Typography>
            )}
            <div className="flex flex-wrap gap-3">
              <Button variant="ghost" onClick={() => open(p)}>
                {t('speechAdmin.edit')}
              </Button>
              <Button
                variant="secondary"
                onClick={() => {
                  setTestProfile(p)
                  setBudget('')
                }}
                disabled={!p.enabled}
              >
                {t('speechAdmin.qualify')}
              </Button>
            </div>
          </div>
        )
      })}
      {editing !== undefined && (
        <SpeechProfileForm
          key={session}
          profile={editing}
          candidates={catalog.browse.candidates}
          saving={catalog.saving}
          onSave={save}
          onCancel={() => setEditing(undefined)}
        />
      )}
      {testProfile && (
        <section className="max-w-measure space-y-3">
          <Typography variant="fieldTitle">
            {t('speechAdmin.qualificationTitle', { name: testProfile.label })}
          </Typography>
          <Typography variant="body">{t('speechAdmin.qualificationHelp')}</Typography>
          <FieldLabel htmlFor={`${id}-budget`}>{t('speechAdmin.budget')}</FieldLabel>
          <TextField
            id={`${id}-budget`}
            inputMode="decimal"
            value={budget}
            onChange={(e) => setBudget(e.target.value)}
          />
          <Button
            pending={catalog.qualifying}
            disabled={budget === ''}
            onClick={() =>
              void catalog
                .startQualification(testProfile.id, testProfile.revision, budget)
                .catch(() => {})
            }
          >
            {t('speechAdmin.prepareQualification')}
          </Button>
          {catalog.qualification?.profileId === testProfile.id && (
            <Notice tone="success" role="status">
              {t('speechAdmin.sessionPrepared', {
                session: catalog.qualification.sessionId,
                at: catalog.qualification.expiresAt,
              })}
            </Notice>
          )}
        </section>
      )}
    </div>
  )
}
