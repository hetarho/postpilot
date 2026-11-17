import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Plus, Quote } from 'lucide-react'
import { useSession } from '@/entities/session'
import {
  useVoices,
  useVoiceProfile,
  VoiceMaterialFreshness,
  VoiceReadinessMeter,
  voiceAnalysisDate,
  type Voice,
} from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { GeneratedWritingVoicesSheet } from '@/features/generate-writing-voices'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { AIAuthoringSheet } from '@/widgets/ai-authoring-studio'
import {
  ActionBar,
  ActionMenu,
  Badge,
  Button,
  ChipToggle,
  Notice,
  LoadingState,
  LoadingStatus,
  Skeleton,
  Typography,
  buttonStyles,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'

/** Guide: 말투 — what a voice IS: how the owner's sentences sound. The screen holds one voice at a
 *  time as a specimen — its own sentence in quotes, the impression it leaves, the phrases it keeps
 *  coming back to — and the owner's other voices as a row of names to switch between. A voice
 *  still being made shows how far its material has come instead, with the way to carry on. Making
 *  a new voice is one button whose two ways (from the owner's own writing, or an AI suggestion)
 *  open from it (VOICE-52). */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common', 'authoring', 'creation'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, defaultVoice, isPending, isError, isFetching, refetch } =
    useVoices(ownerId)
  const [authoringOpen, setAuthoringOpen] = useState(false)
  const search = useSearch({ strict: false }) as { new?: unknown }
  const navigate = useNavigate()
  // `?new=1` (홈's 다음 step, an old 새 말투 link) opens the sheet and is dropped from the address at
  // once, so closing the sheet, or the sheet's own move to the new voice, needs no navigation back.
  const requested = search.new === 1 || search.new === '1'
  const [createOpen, setCreateOpen] = useState(requested)
  const [request, setRequest] = useState(requested)
  if (requested !== request) {
    setRequest(requested)
    if (requested) setCreateOpen(true)
  }
  useEffect(() => {
    if (requested) void navigate({ to: '/voices', search: {}, replace: true })
  }, [requested, navigate])
  const [chosenId, setChosenId] = useState<string>()
  const shown =
    active.find((voice) => voice.id === chosenId) ??
    (defaultVoice && !defaultVoice.deleted ? defaultVoice : undefined) ??
    active.find((voice) => voice.made) ??
    active[0]

  return (
    <main
      className={pageStyles({
        width: 'wide',
        className: 'flex flex-1 flex-col pt-2 sm:pt-4',
      })}
    >
      <h1 className="sr-only">{t('title', { ns: 'voices' })}</h1>
      <CreateVoiceSheet ownerId={ownerId} open={createOpen} onOpenChange={setCreateOpen} />
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

      {isError && (
        <Notice tone="danger" role="alert" className="mt-6">
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
      <LoadingStatus label={!isError && isPending ? t('state.loading', { ns: 'common' }) : ''} />
      {!isError && isPending && (
        <LoadingState
          variant="detail"
          label={t('state.loading', { ns: 'common' })}
          className="mt-6"
        />
      )}

      {!isError && !isPending && active.length === 0 && (
        <section aria-labelledby="voices-empty" className="mt-6 flex flex-col items-start gap-3">
          <span
            aria-hidden="true"
            className="bg-kit-voice/15 text-kit-voice flex size-16 items-center justify-center rounded-2xl"
          >
            <Quote className="size-8" />
          </span>
          <Typography variant="title" id="voices-empty">
            {t('specimen.empty', { ns: 'voices' })}
          </Typography>
          <Typography variant="body" className="text-content-secondary max-w-measure">
            {t('kit.does.voices', { ns: 'creation' })}
          </Typography>
        </section>
      )}

      {!isError && !isPending && shown && (
        <>
          {active.length > 1 && (
            // Finding a voice: every one of the owner's voices by name, the one shown marked.
            <nav
              aria-label={t('specimen.pick', { ns: 'voices' })}
              className="-mx-4 mt-2 flex gap-2 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:flex-wrap sm:px-0"
            >
              {active.map((voice) => (
                <ChipToggle
                  key={voice.id}
                  pressed={voice.id === shown.id}
                  onClick={() => setChosenId(voice.id)}
                  className="min-h-10 gap-2 rounded-full px-4"
                >
                  <span
                    aria-hidden="true"
                    className={clsx(
                      'size-2 rounded-full',
                      voice.made ? 'bg-kit-voice' : 'border-content-tertiary border',
                    )}
                  />
                  {voice.name}
                  {voice.isDefault && (
                    <span className="sr-only">{t('state.default', { ns: 'common' })}</span>
                  )}
                </ChipToggle>
              ))}
            </nav>
          )}
          <VoiceSpecimen key={shown.id} ownerId={ownerId} voice={shown} />
        </>
      )}

      {!isError && !isPending && deleted.length > 0 && (
        // Closed by default: a tombstone is history. It exists at all so a restore stays reachable.
        <details className="mt-8">
          <summary
            className={typographyStyles({
              variant: 'label',
              className:
                'active:bg-row-bg-active text-content-secondary -mx-4 min-h-11 cursor-pointer rounded-md px-4 py-3 select-none',
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
      {!isError && !isPending && (
        <details className="mt-2">
          <summary
            className={typographyStyles({
              variant: 'label',
              className:
                'active:bg-row-bg-active text-content-secondary -mx-4 min-h-11 cursor-pointer rounded-md px-4 py-3 select-none',
            })}
          >
            {t('host.legacyVoice', { ns: 'authoring' })}
          </summary>
          <div className="mt-3">
            <GeneratedWritingVoicesSheet ownerId={ownerId} />
          </div>
        </details>
      )}

      {/* The section's one creation, its two ways opening from it (THEME-67). */}
      <ActionBar
        dock="list"
        ariaLabel={t('create.dockAria', { ns: 'voices' })}
        className="mt-auto w-full sm:w-fit"
      >
        <ActionMenu
          label={t('create.open', { ns: 'voices' })}
          triggerVariant="cta"
          triggerLabel={
            <span className="inline-flex items-center gap-2">
              <Plus aria-hidden="true" className="size-6 shrink-0" />
              {t('create.open', { ns: 'voices' })}
            </span>
          }
          className="w-full sm:w-auto [&>button]:min-h-14 [&>button]:w-full [&>button]:px-7"
          items={[
            {
              id: 'own',
              label: t('specimen.createOwn', { ns: 'voices' }),
              description: t('specimen.createOwnHelp', { ns: 'voices' }),
              onSelect: () => setCreateOpen(true),
            },
            {
              id: 'ai',
              label: t('host.createVoice', { ns: 'authoring' }),
              description: t('specimen.createAIHelp', { ns: 'voices' }),
              onSelect: () => setAuthoringOpen(true),
            },
          ]}
        />
      </ActionBar>
    </main>
  )
}

/** One voice, held up to be read: what it sounds like before anything about it. A voice made from
 *  an AI suggestion is read from a fictional sample, and the specimen says so over it (VOICE-69). */
function VoiceSpecimen({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation(['voices', 'common', 'authoring'])
  const { profile, isPending } = useVoiceProfile(ownerId, voice.id)
  const analysis = profile?.analysis
  const ai = analysis?.ai
  const sample = analysis?.syntheticSample || ai?.examples[0]?.sentence
  const fictional = Boolean(analysis?.syntheticSample) || analysis?.origin === 'synthetic'
  return (
    <section
      aria-labelledby="voice-specimen-name"
      className="bg-card-bg animate-step-in relative mt-4 overflow-hidden rounded-3xl p-6 shadow-md sm:p-10"
    >
      {/* The voice's hue, as a soft wash behind the quote. */}
      <div
        aria-hidden="true"
        className="bg-kit-voice/20 pointer-events-none absolute -top-24 -right-16 size-72 rounded-full blur-3xl"
      />
      <div className="relative flex flex-wrap items-center gap-x-3 gap-y-2">
        <Typography variant="eyebrow" as="p" className="text-kit-voice">
          {t('specimen.eyebrow', { ns: 'voices' })}
        </Typography>
        <span className="flex flex-wrap items-center gap-2">
          {voice.isDefault && <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>}
          {voice.origin === 'synthetic' && <Badge tone="neutral">{t('origin.synthetic')}</Badge>}
          {voice.made && <VoiceMaterialFreshnessFor ownerId={ownerId} voiceId={voice.id} />}
        </span>
      </div>
      <Typography
        variant="title"
        as="h2"
        id="voice-specimen-name"
        className="relative mt-1 break-words"
      >
        {voice.name}
      </Typography>

      {voice.made ? (
        <div className="relative mt-6 space-y-5">
          {isPending ? (
            <span aria-busy="true" className="loading-reveal block space-y-2">
              <Skeleton className="h-7 w-full" />
              <Skeleton className="h-7 w-2/3" />
            </span>
          ) : (
            sample && (
              <figure>
                {fictional && (
                  <figcaption className={typographyStyles({ variant: 'label' })}>
                    {t('fictional', { ns: 'authoring' })}
                  </figcaption>
                )}
                <blockquote
                  className={typographyStyles({
                    variant: 'display',
                    className: 'text-content-primary mt-1 text-pretty',
                  })}
                >
                  “{sample}”
                </blockquote>
              </figure>
            )
          )}
          {ai?.impression && (
            <Typography variant="body" className="text-content-secondary max-w-measure">
              {ai.impression}
            </Typography>
          )}
          {ai && ai.signaturePhrases.length > 0 && (
            <div>
              <Typography variant="label" as="p" className="text-content-secondary">
                {t('specimen.phrases', { ns: 'voices' })}
              </Typography>
              <ul className="mt-2 flex flex-wrap gap-2">
                {ai.signaturePhrases.slice(0, 6).map((phrase) => (
                  <li
                    key={phrase}
                    className={typographyStyles({
                      variant: 'label',
                      className: 'bg-kit-voice/15 text-content-primary rounded-full px-3 py-1',
                    })}
                  >
                    {phrase}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <Typography variant="meta" as="p">
            {t('page.meta', {
              ns: 'voices',
              count: voice.materialCount,
              date: voiceAnalysisDate(voice.analyzedAt),
            })}
          </Typography>
        </div>
      ) : (
        <div className="relative mt-6 space-y-4">
          <Typography variant="body" className="text-content-secondary max-w-measure">
            {t('specimen.unmade', { ns: 'voices' })}
          </Typography>
          {profile ? (
            <VoiceReadinessMeter readiness={profile.readiness} className="max-w-measure" />
          ) : (
            <span aria-busy="true" className="loading-reveal block">
              <Skeleton className="h-10 w-full max-w-md" />
            </span>
          )}
        </div>
      )}

      <div className="relative mt-8 flex flex-wrap items-center gap-3">
        <Link
          to="/voices/$voiceId"
          params={{ voiceId: voice.id }}
          className={buttonStyles({ variant: voice.made ? 'secondary' : 'cta' })}
        >
          {t(voice.made ? 'specimen.open' : 'specimen.continue', { ns: 'voices' })}
        </Link>
        <SetDefaultVoiceButton ownerId={ownerId} voice={voice} />
      </div>
    </section>
  )
}

/** A tombstone keeps its `복원` (VOICE-52). The link stretches over the row through its
 *  `::after` and the button paints above that layer, so nothing interactive nests in the
 *  anchor. */
function DeletedVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  return (
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative -mx-4 flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2">
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

function VoiceMaterialFreshnessFor({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { profile } = useVoiceProfile(ownerId, voiceId)
  return profile ? <VoiceMaterialFreshness profile={profile} compact /> : null
}
