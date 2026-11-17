import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { AudioLines, Plus } from 'lucide-react'
import { KIT_ILLUSTRATIONS } from '@/shared/assets'
import { useSession } from '@/entities/session'
import { SPOKEN_AUDITION_TEXT, SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import { SpokenVoiceActions } from '@/features/manage-spoken-voice'
import {
  ActionBar,
  DirectoryItem,
  EmptySlot,
  LoadingState,
  Typography,
  pageStyles,
  buttonStyles,
  cardStyles,
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
    <main
      className={pageStyles({
        width: 'wide',
        className: 'flex min-w-0 flex-1 flex-col pt-2 sm:pt-4',
      })}
    >
      {/* Guide: the hub's title and the 목소리 chip say where this is; the page leads with what a
          voice DOES — it reads the video — and each card is something to LISTEN to: the play
          button first, the line it reads under it. */}
      <h1 className="sr-only">{t('title')}</h1>
      <Typography variant="display" as="p" aria-hidden="true">
        {t('lead')}
      </Typography>
      <Typography variant="body" className="text-content-secondary mt-2">
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
      {!library.voicesQuery.isError &&
        !library.voicesQuery.isPending &&
        library.voices.length === 0 && (
          // The empty slot of a voice: its microphone, faded, and the line a voice will read
          // once there is one.
          <EmptySlot image={KIT_ILLUSTRATIONS.spoken} className="mt-2">
            <figure className="min-w-0">
              <figcaption className={typographyStyles({ variant: 'label' })}>
                {t('reads')}
              </figcaption>
              <blockquote
                className={typographyStyles({
                  variant: 'body',
                  className: 'text-content-tertiary mt-1 line-clamp-2',
                })}
              >
                “{SPOKEN_AUDITION_TEXT}”
              </blockquote>
            </figure>
          </EmptySlot>
        )}
      {!library.voicesQuery.isError && library.voicesQuery.isPending && (
        <LoadingState variant="list" label={t('loading')} className="mt-4" />
      )}
      <ul className="mt-6 grid gap-3 lg:grid-cols-2 lg:gap-4" aria-label={t('title')}>
        {library.voices.map((voice) => (
          <li
            key={voice.id}
            className={cardStyles({ padding: 'md', className: 'flex min-w-0 flex-col gap-4' })}
          >
            <div className="flex min-w-0 flex-wrap items-center gap-3">
              <SpokenSamplePlayer
                ownerId={ownerId}
                assetId={voice.sampleAssetId}
                name={voice.name}
                durationMs={voice.sampleDurationMs}
                onFailure={() => setStatus('playFailed')}
              />
              <div className="min-w-0 flex-1 basis-40">
                <Typography variant="fieldTitle" as="h2" className="break-words">
                  {voice.name}
                </Typography>
                {voice.description && (
                  <Typography variant="body" className="text-content-secondary line-clamp-2">
                    {voice.description}
                  </Typography>
                )}
              </div>
            </div>
            {voice.previewText && (
              <figure className="bg-card-well rounded-lg px-4 py-3">
                <figcaption className={typographyStyles({ variant: 'label' })}>
                  {t('reads')}
                </figcaption>
                <blockquote
                  className={typographyStyles({
                    variant: 'body',
                    className: 'text-content-primary mt-1 line-clamp-3 break-words',
                  })}
                >
                  “{voice.previewText}”
                </blockquote>
              </figure>
            )}
            <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
              <Typography variant="meta" as="p" className="min-w-0 break-words">
                {voice.profile.designLabel} → {voice.profile.speechLabel} ·{' '}
                {voice.profile.grade
                  ? model(`level.${voice.profile.grade}`)
                  : model('catalog.levelUnset')}
              </Typography>
              <SpokenVoiceActions
                ownerId={ownerId}
                voice={voice}
                onStatus={(key) => setStatus(key === 'failed' ? 'renameFailed' : key)}
              />
            </div>
          </li>
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
