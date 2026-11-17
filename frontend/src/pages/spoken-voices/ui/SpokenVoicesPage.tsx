import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { AudioLines, Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import { SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import { SpokenVoiceActions } from '@/features/manage-spoken-voice'
import { KitArt, KitEmpty, KitHeader } from '@/widgets/writing-kit'
import {
  ActionBar,
  Button,
  LoadingState,
  Notice,
  Typography,
  cardStyles,
  pageStyles,
  buttonStyles,
  typographyStyles,
} from '@/shared/ui'

/** Waveform bar heights (%) drawn behind a voice's name: a voice is something to HEAR, so each
 *  tile leads with its sample, not with its labels. */
const WAVE = [38, 64, 90, 52, 74, 100, 60, 82, 44, 70, 92, 56, 30, 66, 48]

/** Guide: 목소리 — the voices that read the owner's videos aloud. Each is an audition tile: its
 *  sample to listen to first, then what it is (the voice it was designed as → how it speaks) and
 *  its own actions. A voice still being made waits at the top as one to continue; the one action is
 *  a new voice. Separate from 말투, which shapes the written words. */
export function SpokenVoicesPage() {
  const { t } = useTranslation('spokenVoices'),
    { t: model } = useTranslation('models'),
    { t: common } = useTranslation('common'),
    { user } = useSession(),
    ownerId = user?.id ?? '',
    library = useSpokenLibrary(ownerId),
    [status, setStatus] = useState('')
  const drafts = library.drafts.filter((draft) => !draft.confirmedVoiceId)
  const empty =
    !library.voicesQuery.isPending && !library.voicesQuery.isError && !library.voices.length
  return (
    <main
      className={pageStyles({
        width: 'wide',
        className: 'flex min-w-0 flex-1 flex-col pt-2 sm:pt-4',
      })}
    >
      <KitHeader part="spoken" />
      {/* A list that cannot be read says so with its way back, as every kit screen does. */}
      {(library.voicesQuery.isError || library.draftsQuery.isError) && (
        <Notice tone="danger" role="alert" className="mt-6">
          <span>{t('failed')}</span>
          <Button
            variant="ghost"
            onClick={() => {
              void library.voicesQuery.refetch()
              void library.draftsQuery.refetch()
            }}
            pending={library.voicesQuery.isFetching || library.draftsQuery.isFetching}
            className="text-notice-danger-fg underline"
          >
            {common('action.retry')}
          </Button>
        </Notice>
      )}
      <Typography role="status" as="p" variant="meta" className="mt-4 min-h-5">
        {status ? t(status as 'renameFailed' | 'renamed' | 'playFailed') : ' '}
      </Typography>
      {!library.voicesQuery.isError && library.voicesQuery.isPending && (
        <LoadingState variant="list" label={t('loading')} className="mt-2" />
      )}
      {drafts.length > 0 && (
        <section className="mt-2 min-w-0" aria-labelledby="spoken-drafts">
          <Typography variant="title" as="h2" id="spoken-drafts">
            {t('drafts')}
          </Typography>
          <ul className="mt-3 grid gap-3 lg:grid-cols-2 lg:gap-4">
            {drafts.map((draft) => (
              <li key={draft.id}>
                <Link
                  to="/spoken-voices/new"
                  search={{ draft: draft.id }}
                  className={cardStyles({
                    interactive: true,
                    className: 'flex min-w-0 items-center gap-3',
                  })}
                >
                  <AudioLines aria-hidden="true" className="text-link-fg size-6 shrink-0" />
                  <span
                    className={typographyStyles({
                      variant: 'fieldTitle',
                      className: 'text-content-primary min-w-0 break-words',
                    })}
                  >
                    {t('resume', { name: draft.name })}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}
      {empty ? (
        <KitEmpty className="mt-4" figure={<KitArt part="spoken" />} title={t('empty')}>
          {t('emptyNext')}
        </KitEmpty>
      ) : (
        <ul className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-label={t('title')}>
          {library.voices.map((voice) => (
            <li key={voice.id} className={cardStyles({ className: 'flex min-w-0 flex-col gap-4' })}>
              <div className="bg-card-well flex items-center gap-3 rounded-xl px-3 py-3">
                <SpokenSamplePlayer
                  ownerId={ownerId}
                  assetId={voice.sampleAssetId}
                  name={voice.name}
                  durationMs={voice.sampleDurationMs}
                  onFailure={() => setStatus('playFailed')}
                />
                <Wave />
              </div>
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
              <div className="mt-auto">
                <SpokenVoiceActions
                  ownerId={ownerId}
                  voice={voice}
                  onStatus={(key) => setStatus(key === 'failed' ? 'renameFailed' : key)}
                />
              </div>
            </li>
          ))}
        </ul>
      )}
      {/* The section's creation button (THEME-67). */}
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

function Wave({ muted = false }: { muted?: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={
        muted
          ? 'flex h-10 w-40 items-center justify-center gap-1'
          : 'flex h-10 min-w-0 flex-1 items-center gap-1'
      }
    >
      {WAVE.map((height, index) => (
        <span
          key={index}
          className={
            muted
              ? 'bg-content-tertiary/30 w-1.5 rounded-full'
              : 'bg-meter-fill/60 w-1 flex-1 rounded-full'
          }
          style={{ height: `${height}%` }}
        />
      ))}
    </span>
  )
}
