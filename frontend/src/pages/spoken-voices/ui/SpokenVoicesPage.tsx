import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { AudioLines, ChevronRight, Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import { SpokenSamplePlayer, useSpokenLibrary } from '@/entities/spoken-voice'
import { SpokenVoiceActions } from '@/features/manage-spoken-voice'
import { KitScreenHeader } from '@/widgets/writing-kit'
import { spokenIllustration } from '@/shared/assets'
import {
  LoadingState,
  Typography,
  buttonStyles,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** Guide: 목소리 — the voice a video is read in, under 영상. Each saved voice is a card that leads
 *  with its sample: the owner chooses a voice by ear, so 듣기 comes first, then its name and what
 *  it was made from, then rename and delete. A voice left half-made waits on top to be finished.
 *  An account with none sees how one is made — describe it, listen to the AI's candidates, pick
 *  one — over the one way to start, so the empty screen says what the thing is. */
export function SpokenVoicesPage() {
  const { t } = useTranslation('spokenVoices')
  const { t: model } = useTranslation('models')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const library = useSpokenLibrary(ownerId)
  const [status, setStatus] = useState('')
  const drafts = library.drafts.filter((draft) => !draft.confirmedVoiceId)
  const empty =
    !library.voicesQuery.isPending && !library.voicesQuery.isError && !library.voices.length
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex min-w-0 flex-1 flex-col pt-2' })}>
      <KitScreenHeader
        size="section"
        title={t('heading')}
        statement={t('statement')}
        illustration={spokenIllustration}
        actions={
          <Link
            to="/spoken-voices/new"
            className={buttonStyles({ variant: 'cta', className: 'gap-2' })}
          >
            <Plus aria-hidden="true" className="size-5 shrink-0" />
            {t('new')}
          </Link>
        }
      />
      <Typography role="status" as="p" variant="meta" className="mt-4 min-h-5">
        {library.voicesQuery.isError || library.draftsQuery.isError
          ? t('failed')
          : status
            ? t(status as 'renameFailed' | 'renamed' | 'playFailed')
            : ' '}
      </Typography>
      {!library.voicesQuery.isError && library.voicesQuery.isPending && (
        <LoadingState variant="list" label={t('loading')} className="mt-2" />
      )}

      {drafts.length > 0 && (
        <section aria-labelledby="spoken-drafts" className="mt-2">
          <Typography variant="eyebrow" as="h2" id="spoken-drafts">
            {t('drafts')}
          </Typography>
          <ul className="mt-2 flex flex-col gap-2">
            {drafts.map((draft) => (
              <li key={draft.id}>
                <Link
                  to="/spoken-voices/new"
                  search={{ draft: draft.id }}
                  className="bg-badge-accent-bg hover:bg-card-bg-hover active:bg-card-bg-active flex min-h-14 items-center gap-3 rounded-2xl px-4 py-2"
                >
                  <AudioLines aria-hidden="true" className="text-badge-accent-fg size-5 shrink-0" />
                  <span
                    className={typographyStyles({
                      variant: 'label',
                      className: 'text-content-primary min-w-0 flex-1 truncate',
                    })}
                  >
                    {t('resume', { name: draft.name })}
                  </span>
                  <ChevronRight aria-hidden="true" className="text-content-tertiary size-5" />
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {empty && (
        <section aria-labelledby="spoken-how" className="mt-4">
          <h2 id="spoken-how" className="sr-only">
            {t('howAria')}
          </h2>
          {/* The three steps as one short path, each a hairline apart: what the owner does, not
              three cards to read. */}
          <ol
            className={cardStyles({
              padding: 'none',
              className: 'divide-divider max-w-xl divide-y overflow-hidden shadow-sm',
            })}
          >
            {(['describe', 'listen', 'pick'] as const).map((step, index) => (
              <li
                key={step}
                className="animate-rise flex items-start gap-3 px-4 py-3 sm:px-5"
                style={{ animationDelay: `${index * 60}ms` }}
              >
                <span
                  aria-hidden="true"
                  className={typographyStyles({
                    variant: 'label',
                    className:
                      'bg-badge-accent-bg text-badge-accent-fg flex size-7 shrink-0 items-center justify-center rounded-full',
                  })}
                >
                  {index + 1}
                </span>
                <span className="flex min-w-0 flex-col gap-0.5 pt-0.5">
                  <Typography variant="body" as="span" className="text-content-primary">
                    {t(`how.${step}`)}
                  </Typography>
                  {step === 'describe' && (
                    <Typography variant="meta" as="span">
                      {t('how.describeExample')}
                    </Typography>
                  )}
                </span>
              </li>
            ))}
          </ol>
        </section>
      )}

      {library.voices.length > 0 && (
        <section aria-labelledby="spoken-saved" className="mt-4">
          <Typography variant="eyebrow" as="h2" id="spoken-saved">
            {t('saved')}
          </Typography>
          <ul className="mt-2 grid gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-label={t('title')}>
            {library.voices.map((voice) => (
              <li
                key={voice.id}
                className={cardStyles({ padding: 'md', className: 'flex flex-col gap-3' })}
              >
                <SpokenSamplePlayer
                  ownerId={ownerId}
                  assetId={voice.sampleAssetId}
                  name={voice.name}
                  durationMs={voice.sampleDurationMs}
                  onFailure={() => setStatus('playFailed')}
                />
                <div className="flex min-w-0 flex-col gap-1">
                  <Typography variant="fieldTitle" as="h3" className="break-words">
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
                  onStatus={(key) => setStatus(key === 'failed' ? 'renameFailed' : key)}
                />
              </li>
            ))}
          </ul>
        </section>
      )}
    </main>
  )
}
