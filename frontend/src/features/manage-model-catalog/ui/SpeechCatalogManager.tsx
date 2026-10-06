import { useId, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAdminSpeechProfiles, refKey, type AdminSpeechProfile } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Notice,
  TextField,
  Typography,
  Disclosure,
  buttonStyles,
} from '@/shared/ui'
import { SpeechCombinationRow } from './SpeechCombinationRow'
import { SpeechTariffEditor } from './SpeechTariffEditor'

export function SpeechCatalogManager() {
  const { t } = useTranslation('models')
  const id = useId()
  const catalog = useAdminSpeechProfiles()
  const metadataUnavailable =
    catalog.isPending ||
    catalog.isError ||
    Boolean(catalog.browse.fetchError) ||
    catalog.browse.candidates.length === 0
  const [budget, setBudget] = useState('')
  const [testProfile, setTestProfile] = useState<AdminSpeechProfile | null>(null)
  const pairKey = (p: AdminSpeechProfile) =>
    `${refKey(p.binding.designModel)}:${refKey(p.binding.speechModel)}`
  const rows = new Map(catalog.browse.combinations.map((p) => [pairKey(p), p]))
  for (const p of catalog.browse.profiles) {
    // The oldest retained registration owns a legacy duplicate pair.
    if (!rows.get(pairKey(p))?.id) rows.set(pairKey(p), p)
  }
  const busy = catalog.saving || catalog.savingTariff
  return (
    <div className="mt-6 space-y-5">
      <Typography variant="body">{t('speechAdmin.description')}</Typography>
      <div className="flex flex-wrap gap-3">
        <Button variant="ghost" pending={catalog.refreshing} onClick={catalog.refresh}>
          {t('catalog.refresh')}
        </Button>
      </div>
      {(catalog.isError || catalog.browse.fetchError) && (
        <Notice tone="warning" role="status">
          {catalog.isError
            ? t('speechAdmin.loadFailed')
            : t(`speechAdmin.connectionReason.${catalog.browse.fetchError}`, {
                defaultValue: t('speechAdmin.fetchFailed'),
              })}
        </Notice>
      )}
      {catalog.failure && <AppFailureMessage failure={catalog.failure} />}
      {catalog.isPending && (
        <Typography variant="body" role="status">
          {t('catalog.loading')}
        </Typography>
      )}
      {catalog.hasData && !catalog.isPending && !catalog.isError && rows.size === 0 && (
        <Typography variant="body">{t('speechAdmin.empty')}</Typography>
      )}
      <Disclosure title={t('speechAdmin.commonTariff')} size="row">
        <SpeechTariffEditor
          key={catalog.browse.tariff?.revision.toString() ?? 'unset'}
          tariff={catalog.browse.tariff}
          saving={catalog.savingTariff}
          unavailable={metadataUnavailable || busy}
          onSave={catalog.saveTariff}
        />
      </Disclosure>
      <ul>
        {[...rows.values()].map((p) => (
          <SpeechCombinationRow
            key={`${pairKey(p)}:${p.revision}`}
            profile={p}
            designName={
              catalog.browse.candidates.find((m) => refKey(m.ref) === refKey(p.binding.designModel))
                ?.label ?? catalog.browse.choices.find((c) => c.id === p.id)?.designLabel
            }
            speechName={
              catalog.browse.candidates.find((m) => refKey(m.ref) === refKey(p.binding.speechModel))
                ?.label ?? catalog.browse.choices.find((c) => c.id === p.id)?.speechLabel
            }
            saving={busy}
            metadataAvailable={!metadataUnavailable}
            styleSupported={
              catalog.browse.candidates.find((m) => refKey(m.ref) === refKey(p.binding.speechModel))
                ?.style ?? p.binding.settings.style > 0
            }
            reason={catalog.browse.choices.find((choice) => choice.id === p.id)?.unavailableReason}
            onSave={catalog.save}
            onQualify={(profile) => {
              setTestProfile(profile)
              setBudget('')
            }}
          />
        ))}
      </ul>
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
            <div className="space-y-3">
              <Notice tone="success" role="status">
                {t('speechAdmin.sessionPrepared', {
                  session: catalog.qualification.sessionId,
                  at: catalog.qualification.expiresAt,
                })}
              </Notice>
              <Link
                to="/spoken-voices/new"
                search={{ qualification: catalog.qualification.sessionId }}
                className={buttonStyles({ variant: 'cta' })}
              >
                {t('speechAdmin.openCreation')}
              </Link>
            </div>
          )}
        </section>
      )}
    </div>
  )
}
