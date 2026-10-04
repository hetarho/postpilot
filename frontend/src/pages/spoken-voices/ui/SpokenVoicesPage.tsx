import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import { SpokenVoiceActions } from '@/features/manage-spoken-voice'
import { ActionBar, Typography, pageStyles, buttonStyles } from '@/shared/ui'
export function SpokenVoicesPage() {
  const { t } = useTranslation('spokenVoices'),
    { t: model } = useTranslation('models'),
    { user } = useSession(),
    ownerId = user?.id ?? '',
    library = useSpokenLibrary(ownerId),
    [status, setStatus] = useState('')
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex min-w-0 flex-1 flex-col' })}>
      <Typography variant="display">{t('title')}</Typography>
      <Typography variant="body" className="text-content-secondary mt-4">
        {t('about')}
      </Typography>
      <Typography role="status" as="p" variant="meta" className="mt-4 min-h-5">
        {library.voicesQuery.isError || library.draftsQuery.isError
          ? t('failed')
          : library.voicesQuery.isPending
            ? t('loading')
            : status
              ? t(status as 'renameFailed' | 'renamed' | 'playFailed')
              : !library.voices.length
                ? t('empty')
                : '\u00a0'}
      </Typography>
      <ul className="divide-divider mt-4 divide-y" aria-label={t('title')}>
        {library.voices.map((voice) => (
          <li key={voice.id} className="min-w-0 space-y-3 py-5">
            <Typography variant="title" className="break-words">
              {voice.name}
            </Typography>
            <Typography variant="meta" className="break-words">
              {voice.profile.designLabel} → {voice.profile.speechLabel} ·{' '}
              {voice.profile.grade
                ? model(`level.${voice.profile.grade}`)
                : model('catalog.levelUnset')}
            </Typography>
            <SpokenSamplePlayer
              ownerId={ownerId}
              assetId={voice.sampleAssetId}
              name={voice.name}
              durationMs={voice.sampleDurationMs}
              onFailure={() => setStatus('playFailed')}
            />
            <SpokenVoiceActions
              ownerId={ownerId}
              voice={voice}
              onStatus={(key) => setStatus(key === 'failed' ? 'renameFailed' : key)}
            />
          </li>
        ))}
      </ul>
      {library.drafts.some((d) => !d.confirmedVoiceId) && (
        <section className="mt-8 min-w-0">
          <Typography variant="title">{t('drafts')}</Typography>
          <ul className="mt-3">
            {library.drafts
              .filter((d) => !d.confirmedVoiceId)
              .map((d) => (
                <li key={d.id}>
                  <Link
                    to="/spoken-voices/new"
                    search={{ draft: d.id }}
                    className={buttonStyles({
                      variant: 'ghost',
                      className: 'max-w-full break-words',
                    })}
                  >
                    {t('resume', { name: d.name })}
                  </Link>
                </li>
              ))}
          </ul>
        </section>
      )}
      <ActionBar dock="list" className="mt-auto">
        <Link to="/spoken-voices/new" className={buttonStyles({ variant: 'cta' })}>
          {t('new')}
        </Link>
      </ActionBar>
    </main>
  )
}
