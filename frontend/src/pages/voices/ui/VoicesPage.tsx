import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { CLIP_CHANNELS, type ClipChannel } from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import {
  useVoices,
  useVoiceProfile,
  VoiceMaterialFreshness,
  voiceAnalysisDate,
  type Voice,
} from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { GeneratedWritingVoicesSheet } from '@/features/generate-writing-voices'
import { AIAuthoringSheet } from '@/widgets/ai-authoring-studio'
import { ChannelActions } from '@/widgets/channel-actions'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { Quote } from 'lucide-react'
import {
  Badge,
  Button,
  Notice,
  LoadingState,
  LoadingStatus,
  Typography,
  cardStyles,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'

/** The account's voices (VOICE-52): the 말투 section every channel shares, one list in 내 글's row
 *  shape and explicit creation paths. 새 말투 comes from the section's creation button (`?new=1`);
 *  the other creation paths sit above the list. Every lifecycle action lives on the voice's own
 *  title row, so a row here is nothing but a way into one voice. */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  const [authoringOpen, setAuthoringOpen] = useState(false)
  const search = useSearch({ strict: false }) as { new?: unknown; channel?: unknown }
  const channel = CLIP_CHANNELS.includes(search.channel as ClipChannel)
    ? (search.channel as ClipChannel)
    : undefined
  const navigate = useNavigate()
  // `?new=1` is the section's 새 말투 button (THEME-67): it opens the sheet and is dropped from the
  // address at once, so closing the sheet, or the sheet's own move to the new voice, needs no
  // second navigation back here.
  const requested = search.new === 1 || search.new === '1'
  const [createOpen, setCreateOpen] = useState(requested)
  const [request, setRequest] = useState(requested)
  if (requested !== request) {
    setRequest(requested)
    if (requested) setCreateOpen(true)
  }
  useEffect(() => {
    if (requested)
      void navigate({ to: '/voices', search: channel ? { channel } : {}, replace: true })
  }, [requested, channel, navigate])
  const { t: authoringText } = useTranslation('authoring')

  return (
    // The page gutter lives on each block rather than on `main`, so the rows run edge to edge
    // the way 내 글's do (THEME-23).
    <main
      className={pageStyles({
        width: 'wide',
        gutters: false,
        className: 'flex flex-1 flex-col pt-4 sm:pt-6 lg:pt-8',
      })}
    >
      <div className="px-4 sm:px-6 lg:px-8">
        {/* Guide: the hub's title and the 말투 chip say where this is; the page leads with what a
            voice does — and each card below shows how it sounds before anything about it. */}
        <h1 className="sr-only">{t('title', { ns: 'voices' })}</h1>
        <Typography variant="display" as="p" aria-hidden="true">
          {t('page.lead', { ns: 'voices' })}
        </Typography>
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <Button variant="secondary" onClick={() => setAuthoringOpen(true)}>
            {authoringText('host.createVoice')}
          </Button>
          <AIAuthoringSheet
            open={authoringOpen}
            onOpenChange={setAuthoringOpen}
            ownerId={ownerId}
            kind="writing-voice"
            onSaved={() => {
              refetch()
              setAuthoringOpen(false)
            }}
          />
          <details>
            <summary className="text-content-secondary min-h-11 cursor-pointer py-3">
              {authoringText('host.legacyVoice')}
            </summary>
            <GeneratedWritingVoicesSheet ownerId={ownerId} />
          </details>
        </div>
        <CreateVoiceSheet ownerId={ownerId} open={createOpen} onOpenChange={setCreateOpen} />
      </div>

      {isError && (
        <Notice tone="danger" role="alert" className="mx-4 mt-8 sm:mx-6">
          <span>{t('loadFailed', { ns: 'voices' })}</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}
      <LoadingStatus
        label={
          isError
            ? ''
            : isPending
              ? t('state.loading', { ns: 'common' })
              : active.length === 0
                ? t('page.empty', { ns: 'voices' })
                : ''
        }
        className={
          !isError && !isPending && active.length === 0
            ? typographyStyles({
                variant: 'body',
                className: 'text-content-tertiary mt-8 block px-4 sm:px-6 lg:px-8',
              })
            : 'sr-only'
        }
      />
      {!isError && isPending && (
        <div aria-hidden="true">
          <LoadingState
            variant="cards"
            label={t('state.loading', { ns: 'common' })}
            className="mt-6 px-4 sm:px-6 lg:px-8 sm:[&>.loading-reveal]:grid-cols-1 lg:[&>.loading-reveal]:grid-cols-2"
          />
        </div>
      )}

      {!isError && !isPending && (
        <>
          {active.length > 0 && (
            <ul className="mt-6 grid shrink-0 gap-4 px-4 sm:px-6 lg:grid-cols-2 lg:px-8">
              {active.map((voice) => (
                <li key={voice.id}>
                  <Link
                    to="/voices/$voiceId"
                    params={{ voiceId: voice.id }}
                    className={cardStyles({
                      interactive: true,
                      className: 'flex h-full flex-col gap-4',
                    })}
                  >
                    <VoiceRowContent ownerId={ownerId} voice={voice} />
                  </Link>
                </li>
              ))}
            </ul>
          )}

          {deleted.length > 0 && (
            // Closed by default: a tombstone is history, and the list the user came for is the
            // active one. It exists at all so a restore stays reachable.
            <details className="mt-10">
              <summary
                className={typographyStyles({
                  variant: 'label',
                  className:
                    'active:bg-row-bg-active text-content-secondary mx-4 min-h-11 cursor-pointer rounded-md px-4 py-3 select-none sm:mx-6 lg:mx-8',
                })}
              >
                {t('page.deleted', { ns: 'voices', count: deleted.length })}
              </summary>
              <ul className="divide-divider mt-3 divide-y">
                {deleted.map((voice) => (
                  <DeletedVoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
                ))}
              </ul>
            </details>
          )}
        </>
      )}
      {/* The section's creation button is the page's one dock (THEME-67). */}
      <ChannelActions channel={channel} section="voices" className="action-list-inset mt-auto" />
    </main>
  )
}

/** Guide: a voice card leads with how the voice SOUNDS — the analysis's one-line impression and
 *  one of its own example sentences in quotes, then its signature phrases — and only then its
 *  name, the 기본 badge and the meta line (`만드는 중` until it is made, then how many 학습 글 it
 *  holds and the day its analysis was published). A sentence an AI-made style was read from is a
 *  fictional sample, and the card says so over it (VOICE-69). */
function VoiceRowContent({ voice, ownerId }: { voice: Voice; ownerId: string }) {
  const { t } = useTranslation(['voices', 'common', 'authoring'])
  const { profile } = useVoiceProfile(ownerId, voice.made ? voice.id : '')
  const analysis = profile?.analysis
  const ai = analysis?.ai
  const sample = analysis?.syntheticSample || ai?.examples[0]?.sentence
  const fictional = Boolean(analysis?.syntheticSample) || analysis?.origin === 'synthetic'
  return (
    <>
      <span className="flex min-w-0 items-start gap-3">
        <span
          aria-hidden="true"
          className="bg-badge-accent-bg text-badge-accent-fg flex size-11 shrink-0 items-center justify-center rounded-lg"
        >
          <Quote className="size-5" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <Typography variant="fieldTitle" as="span" className="text-content-primary truncate">
            {voice.name}
          </Typography>
          <span className="flex min-w-0 flex-wrap items-center gap-2">
            {voice.isDefault && <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>}
            {voice.origin === 'synthetic' && <Badge tone="neutral">{t('origin.synthetic')}</Badge>}
            {voice.made && <DirectoryFreshness ownerId={ownerId} voiceId={voice.id} />}
          </span>
        </span>
      </span>
      {sample && (
        <span className="bg-card-well flex flex-col gap-1 rounded-lg px-4 py-3">
          {fictional && (
            <span className={typographyStyles({ variant: 'label' })}>
              {t('fictional', { ns: 'authoring' })}
            </span>
          )}
          <span
            className={typographyStyles({ variant: 'body', className: 'text-content-primary' })}
          >
            “{sample}”
          </span>
        </span>
      )}
      {ai?.impression && (
        <span
          className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}
        >
          {ai.impression}
        </span>
      )}
      {ai && ai.signaturePhrases.length > 0 && (
        <span className="flex flex-wrap gap-1.5">
          {ai.signaturePhrases.slice(0, 3).map((phrase) => (
            <Badge key={phrase}>{phrase}</Badge>
          ))}
        </span>
      )}
      <span className={typographyStyles({ variant: 'meta', className: 'mt-auto truncate' })}>
        {voice.made
          ? t('page.meta', {
              ns: 'voices',
              count: voice.materialCount,
              date: voiceAnalysisDate(voice.analyzedAt),
            })
          : t('page.making', { ns: 'voices', percent: voice.readinessPercent })}
      </span>
    </>
  )
}

/** A tombstone keeps its `복원` (VOICE-52). The link stretches over the row through its
 *  `::after` and the button paints above that layer, so nothing interactive nests in the
 *  anchor. */
function DeletedVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  return (
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2 sm:px-6 lg:px-8">
      <Link
        to="/voices/$voiceId"
        params={{ voiceId: voice.id }}
        className={typographyStyles({
          variant: 'label',
          className: 'min-w-0 truncate after:absolute after:inset-0',
        })}
      >
        {voice.name}
      </Link>
      <div className="relative ml-auto flex shrink-0 items-center">
        <RestoreVoiceButton ownerId={ownerId} voiceId={voice.id} />
      </div>
    </li>
  )
}

function DirectoryFreshness({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { profile } = useVoiceProfile(ownerId, voiceId)
  return profile ? <VoiceMaterialFreshness profile={profile} compact /> : null
}
