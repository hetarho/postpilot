import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { AudioLines, Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import { SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import { SpokenVoiceActions } from '@/features/manage-spoken-voice'
import {
  ActionBar,
  DirectoryItem,
  LoadingState,
  Typography,
  pageStyles,
  buttonStyles,
  typographyStyles,
} from '@/shared/ui'
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
          : status
            ? t(status as 'renameFailed' | 'renamed' | 'playFailed')
            : !library.voicesQuery.isPending && !library.voices.length
              ? t('empty')
              : '\u00a0'}
      </Typography>
      {!library.voicesQuery.isError && library.voicesQuery.isPending && (
        <LoadingState variant="list" label={t('loading')} className="mt-4" />
      )}
      <ul className="mt-6 grid gap-3 lg:grid-cols-2 lg:gap-4" aria-label={t('title')}>
        {library.voices.map((voice) => (
          <DirectoryItem
            key={voice.id}
            icon={<AudioLines className="size-6" />}
            title={
              <Typography variant="fieldTitle" as="h2" className="break-words">
                {voice.name}
              </Typography>
            }
            description={
              <Typography variant="meta" as="p" className="break-words">
                {voice.profile.designLabel} → {voice.profile.speechLabel} ·{' '}
                {voice.profile.grade
                  ? model(`level.${voice.profile.grade}`)
                  : model('catalog.levelUnset')}
              </Typography>
            }
            status={
              <SpokenSamplePlayer
                ownerId={ownerId}
                assetId={voice.sampleAssetId}
                name={voice.name}
                durationMs={voice.sampleDurationMs}
                onFailure={() => setStatus('playFailed')}
              />
            }
            metadata={
              <SpokenVoiceActions
                ownerId={ownerId}
                voice={voice}
                onStatus={(key) => setStatus(key === 'failed' ? 'renameFailed' : key)}
              />
            }
          />
        ))}
      </ul>
      {library.drafts.some((d) => !d.confirmedVoiceId) && (
        <section className="mt-8 min-w-0">
          <Typography variant="title">{t('drafts')}</Typography>
          <ul className="mt-3 grid gap-3 lg:grid-cols-2 lg:gap-4">
            {library.drafts
              .filter((d) => !d.confirmedVoiceId)
              .map((d) => (
                <DirectoryItem
                  key={d.id}
                  interactive
                  icon={<AudioLines className="size-6" />}
                  title={
                    <Link
                      to="/spoken-voices/new"
                      search={{ draft: d.id }}
                      className={typographyStyles({
                        variant: 'fieldTitle',
                        className: 'break-words after:absolute after:inset-0',
                      })}
                    >
                      {t('resume', { name: d.name })}
                    </Link>
                  }
                />
              ))}
          </ul>
        </section>
      )}
      {/* The section's creation button: the full width on a phone and a right-aligned pill from
          `sm:`, like every sibling section's (THEME-67). */}
      <ActionBar dock="list" className="mt-auto w-full sm:w-fit">
        <Link
          to="/spoken-voices/new"
          className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full sm:w-auto' })}
        >
          <Plus aria-hidden="true" className="size-6 shrink-0" />
          {t('new')}
        </Link>
      </ActionBar>
    </main>
  )
}
