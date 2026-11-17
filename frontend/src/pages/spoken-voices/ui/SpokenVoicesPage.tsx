import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { AudioLines, Play, Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import { SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import { SpokenVoiceActions } from '@/features/manage-spoken-voice'
import {
  ActionBar,
  LoadingState,
  Typography,
  pageStyles,
  buttonStyles,
  typographyStyles,
} from '@/shared/ui'

/** The bars of a voice card's waveform: a fixed, decorative shape (no audio is analysed). */
const WAVE = [6, 14, 22, 12, 28, 18, 34, 20, 26, 10, 30, 16, 24, 8, 20, 12] as const

/** Guide: 목소리 — what a spoken voice IS: the voice a video is read in. Each voice is a card made
 *  for listening — its sample's play control first, beside its waveform, then its name and how it
 *  was made; renaming, re-making and removing wait behind the card's ⋯. A voice still being made
 *  waits under 이어 만들기, and 새 목소리 is the one creation. */
export function SpokenVoicesPage() {
  const { t } = useTranslation('spokenVoices'),
    { t: model } = useTranslation('models'),
    { user } = useSession(),
    ownerId = user?.id ?? '',
    library = useSpokenLibrary(ownerId),
    [status, setStatus] = useState('')
  const drafts = library.drafts.filter((draft) => !draft.confirmedVoiceId)
  return (
    <main
      className={pageStyles({
        width: 'wide',
        className: 'flex min-w-0 flex-1 flex-col pt-2 sm:pt-4',
      })}
    >
      <h1 className="sr-only">{t('title')}</h1>
      <Typography variant="display" as="p" aria-hidden="true">
        {t('lead')}
      </Typography>
      <Typography role="status" as="p" variant="meta" className="mt-3 min-h-5">
        {library.voicesQuery.isError || library.draftsQuery.isError
          ? t('failed')
          : status
            ? t(status as 'renameFailed' | 'renamed' | 'playFailed')
            : ' '}
      </Typography>
      {!library.voicesQuery.isError && library.voicesQuery.isPending && (
        <LoadingState variant="cards" label={t('loading')} className="mt-4" />
      )}
      {!library.voicesQuery.isPending && !library.voices.length && (
        // Empty, the screen still shows what a voice here is: a card to listen to, not yet filled.
        <section
          aria-labelledby="spoken-voices-empty"
          className="mt-4 grid items-center gap-5 sm:grid-cols-2 lg:grid-cols-3"
        >
          <div
            aria-hidden="true"
            className="border-divider flex min-w-0 flex-col gap-4 rounded-3xl border-2 border-dashed p-5"
          >
            <span className="flex flex-col gap-2">
              <span className="bg-surface-recessed h-4 w-2/5 rounded-full" />
              <span className="bg-surface-recessed h-3 w-3/5 rounded-full" />
            </span>
            <span className="bg-surface-recessed flex min-w-0 items-center gap-4 rounded-2xl px-4 py-3">
              <span className="bg-link-fg-current/15 text-link-fg-current flex size-11 shrink-0 items-center justify-center rounded-full">
                <Play className="size-5 translate-x-px" />
              </span>
              <span className="flex h-9 min-w-0 flex-1 items-center gap-1 overflow-hidden">
                {WAVE.map((height, index) => (
                  <span
                    key={index}
                    className="bg-link-fg-current/20 w-1 shrink-0 rounded-full"
                    style={{ height }}
                  />
                ))}
              </span>
            </span>
          </div>
          <div className="lg:col-span-2">
            <Typography variant="title" as="h2" id="spoken-voices-empty">
              {t('empty')}
            </Typography>
            <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
              {t('emptyHelp')}
            </Typography>
          </div>
        </section>
      )}
      {library.voices.length > 0 && (
        <ul className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-label={t('title')}>
          {library.voices.map((voice) => (
            <li
              key={voice.id}
              className="bg-card-bg relative flex min-w-0 flex-col gap-4 overflow-hidden rounded-3xl p-5 shadow-md"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <Typography variant="fieldTitle" as="h2" className="break-words">
                    {voice.name}
                  </Typography>
                  <Typography variant="meta" as="p" className="break-words">
                    {voice.profile.designLabel} → {voice.profile.speechLabel} ·{' '}
                    {voice.profile.grade
                      ? model(`level.${voice.profile.grade}`)
                      : model('catalog.levelUnset')}
                  </Typography>
                </div>
                <SpokenVoiceActions
                  ownerId={ownerId}
                  voice={voice}
                  presentation="menu"
                  onStatus={(key) => setStatus(key === 'failed' ? 'renameFailed' : key)}
                />
              </div>
              {/* The card's job: hearing the voice. */}
              <div className="bg-surface-recessed flex min-w-0 flex-wrap items-center gap-4 rounded-2xl px-4 py-3">
                <SpokenSamplePlayer
                  ownerId={ownerId}
                  assetId={voice.sampleAssetId}
                  name={voice.name}
                  durationMs={voice.sampleDurationMs}
                  onFailure={() => setStatus('playFailed')}
                />
                <span
                  aria-hidden="true"
                  className="flex h-9 min-w-0 flex-1 items-center gap-1 overflow-hidden"
                >
                  {WAVE.map((height, index) => (
                    <span
                      key={index}
                      className="bg-link-fg-current/60 w-1 shrink-0 rounded-full"
                      style={{ height }}
                    />
                  ))}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
      {drafts.length > 0 && (
        <section className="mt-8 min-w-0">
          <Typography variant="title" as="h2">
            {t('drafts')}
          </Typography>
          <ul className="mt-3 grid gap-3 lg:grid-cols-2">
            {drafts.map((draft) => (
              <li
                key={draft.id}
                className="bg-card-bg hover:bg-card-bg-hover relative flex min-h-16 items-center gap-3 rounded-2xl px-4 py-3 shadow-sm"
              >
                <AudioLines aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
                <Link
                  to="/spoken-voices/new"
                  search={{ draft: draft.id }}
                  className={typographyStyles({
                    variant: 'fieldTitle',
                    className: 'min-w-0 break-words after:absolute after:inset-0',
                  })}
                >
                  {t('resume', { name: draft.name })}
                </Link>
              </li>
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
