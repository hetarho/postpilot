import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
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
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { AIAuthoringSheet } from '@/widgets/ai-authoring-studio'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { KIT_ILLUSTRATIONS } from '@/shared/assets'
import { ChevronRight, Plus, Quote } from 'lucide-react'
import {
  ActionBar,
  ActionMenu,
  Badge,
  Button,
  EmptySlot,
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
  const search = useSearch({ strict: false }) as { new?: unknown }
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
    if (requested) void navigate({ to: '/voices', search: {}, replace: true })
  }, [requested, navigate])
  const { t: authoringText } = useTranslation('authoring')
  const spotlight =
    active.find((voice) => voice.isDefault && voice.made) ?? active.find((voice) => voice.made)
  const others = active.filter((voice) => voice.id !== spotlight?.id)

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
      {!isError && !isPending && active.length === 0 ? (
        // No voice at all: the dashed place the first one will stand, its pen faded. The words
        // are the screen's status, filled once it has settled.
        <div className="mt-6 px-4 sm:px-6 lg:px-8">
          <EmptySlot image={KIT_ILLUSTRATIONS.voice}>
            <LoadingStatus label={t('page.empty', { ns: 'voices' })} className="" />
          </EmptySlot>
        </div>
      ) : (
        <LoadingStatus label={isError || !isPending ? '' : t('state.loading', { ns: 'common' })} />
      )}
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
          {/* Guide: the voice in use leads, drawn as how it SOUNDS — its own sentence, large,
              and the phrases it keeps coming back to — because that is what it does to a post.
              The others follow as one line each, with 기본으로 in reach on the row. */}
          {spotlight ? (
            <div className="mt-6 px-4 sm:px-6 lg:px-8">
              <VoiceSpotlight ownerId={ownerId} voice={spotlight} />
            </div>
          ) : (
            active.length > 0 && (
              // The slot the voice in use will fill: its outline, and what fills it.
              <div className="mt-6 px-4 sm:px-6 lg:px-8">
                <EmptySlot
                  image={KIT_ILLUSTRATIONS.voice}
                  title={t('page.noInUse', { ns: 'voices' })}
                >
                  {t('page.noInUseHelp', { ns: 'voices' })}
                </EmptySlot>
              </div>
            )
          )}
          {others.length > 0 && (
            <section aria-labelledby="voices-others" className="mt-10">
              <Typography
                variant="title"
                as="h2"
                id="voices-others"
                className="px-4 sm:px-6 lg:px-8"
              >
                {t(spotlight ? 'page.others' : 'page.mine', { ns: 'voices' })}
              </Typography>
              <ul className="divide-divider mt-3 divide-y">
                {others.map((voice) => (
                  <OtherVoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
                ))}
              </ul>
            </section>
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
      <details className="mt-6 px-4 sm:px-6 lg:px-8">
        <summary className="text-content-secondary min-h-11 cursor-pointer py-3">
          {authoringText('host.legacyVoice')}
        </summary>
        <GeneratedWritingVoicesSheet ownerId={ownerId} />
      </details>
      {/* The section's one dock (THEME-67): 새 말투, whose two ways of making one open from it. */}
      <ActionBar dock="list" className="action-list-inset mt-auto w-full sm:w-fit">
        <ActionMenu
          label={t('page.new', { ns: 'voices' })}
          triggerLabel={
            <>
              <Plus aria-hidden="true" className="size-6 shrink-0" />
              {t('page.new', { ns: 'voices' })}
            </>
          }
          triggerVariant="cta"
          triggerSize="hero"
          triggerClassName="w-full sm:w-auto"
          className="w-full sm:w-auto"
          items={[
            {
              id: 'learn',
              label: t('page.methods.learn.label', { ns: 'voices' }),
              description: t('page.methods.learn.description', { ns: 'voices' }),
              onSelect: () => setCreateOpen(true),
            },
            {
              id: 'ai',
              label: t('page.methods.ai.label', { ns: 'voices' }),
              description: t('page.methods.ai.description', { ns: 'voices' }),
              onSelect: () => setAuthoringOpen(true),
            },
          ]}
        />
      </ActionBar>
    </main>
  )
}

/** The voice new posts are written in, as it sounds: its own sentence large, the impression the
 *  analysis gave it, the phrases it keeps using, and the facts under it. The whole card opens it. */
function VoiceSpotlight({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation(['voices', 'common', 'authoring'])
  const { profile } = useVoiceProfile(ownerId, voice.id)
  const analysis = profile?.analysis
  const ai = analysis?.ai
  const sample = analysis?.syntheticSample || ai?.examples[0]?.sentence
  const fictional = Boolean(analysis?.syntheticSample) || analysis?.origin === 'synthetic'
  return (
    <Link
      to="/voices/$voiceId"
      params={{ voiceId: voice.id }}
      className={cardStyles({
        interactive: true,
        padding: 'lg',
        className: 'flex flex-col gap-5',
      })}
    >
      <span className="flex flex-wrap items-center gap-2">
        <Typography variant="eyebrow" as="span">
          {t('page.inUse', { ns: 'voices' })}
        </Typography>
        {voice.isDefault && <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>}
        {voice.origin === 'synthetic' && <Badge tone="neutral">{t('origin.synthetic')}</Badge>}
      </span>
      <span className="flex min-w-0 items-start gap-3">
        <Quote aria-hidden="true" className="text-link-fg mt-1 size-7 shrink-0" />
        <span className="flex min-w-0 flex-col gap-2">
          {fictional && sample && (
            <span className={typographyStyles({ variant: 'label' })}>
              {t('fictional', { ns: 'authoring' })}
            </span>
          )}
          <span
            className={typographyStyles({
              variant: 'stepTitle',
              className: 'text-content-primary break-words',
            })}
          >
            {sample ?? voice.name}
          </span>
          {ai?.impression && (
            <span
              className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}
            >
              {ai.impression}
            </span>
          )}
        </span>
      </span>
      {ai && ai.signaturePhrases.length > 0 && (
        <span className="flex flex-col gap-2">
          <span className={typographyStyles({ variant: 'label' })}>
            {t('page.habits', { ns: 'voices' })}
          </span>
          <span className="flex flex-wrap gap-1.5">
            {ai.signaturePhrases.slice(0, 6).map((phrase) => (
              <span
                key={phrase}
                className={typographyStyles({
                  variant: 'body',
                  className:
                    'bg-voice-habit-highlight text-voice-habit-foreground rounded-md px-2 py-0.5',
                })}
              >
                {phrase}
              </span>
            ))}
          </span>
        </span>
      )}
      <span className="border-divider flex flex-wrap items-center gap-x-3 gap-y-1 border-t pt-4">
        <Typography
          variant="fieldTitle"
          as="span"
          className="text-content-primary min-w-0 break-words"
        >
          {voice.name}
        </Typography>
        <span className={typographyStyles({ variant: 'meta', className: 'min-w-0' })}>
          {voice.isDefault
            ? t('page.meta', {
                ns: 'voices',
                count: voice.materialCount,
                date: voiceAnalysisDate(voice.analyzedAt),
              })
            : t('page.notDefault', { ns: 'voices' })}
        </span>
        <DirectoryFreshness ownerId={ownerId} voiceId={voice.id} />
        <ChevronRight
          aria-hidden="true"
          className="text-content-tertiary ml-auto size-5 shrink-0"
        />
      </span>
    </Link>
  )
}

/** Another voice, in one line: its name, a line of how it sounds (or how far it is made), and
 *  기본으로 on the row for a made one. The link stretches over the row; the button paints above. */
function OtherVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation(['voices', 'common'])
  const { profile } = useVoiceProfile(ownerId, voice.made ? voice.id : '')
  const sample = profile?.analysis?.syntheticSample || profile?.analysis?.ai?.examples[0]?.sentence
  return (
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 sm:px-6 lg:px-8">
      <span
        aria-hidden="true"
        className="bg-badge-accent-bg text-badge-accent-fg flex size-10 shrink-0 items-center justify-center rounded-lg"
      >
        <Quote className="size-5" />
      </span>
      <span className="flex min-w-0 flex-1 basis-48 flex-col gap-1">
        <Link
          to="/voices/$voiceId"
          params={{ voiceId: voice.id }}
          className={typographyStyles({
            variant: 'fieldTitle',
            className: 'text-content-primary truncate after:absolute after:inset-0',
          })}
        >
          {voice.name}
        </Link>
        {voice.made ? (
          sample && (
            <span
              className={typographyStyles({
                variant: 'body',
                className: 'text-content-secondary truncate',
              })}
            >
              “{sample}”
            </span>
          )
        ) : (
          <span className="flex min-w-0 flex-col gap-1.5">
            <span
              aria-hidden="true"
              className="bg-meter-track block h-1.5 w-full max-w-60 overflow-hidden rounded-full"
            >
              <span
                className="bg-meter-fill block h-full rounded-full"
                style={{ width: `${voice.readinessPercent}%` }}
              />
            </span>
            <span className={typographyStyles({ variant: 'meta' })}>
              {voice.readinessPercent >= 100
                ? t('page.readyToMake', { ns: 'voices' })
                : t('page.making', { ns: 'voices', percent: voice.readinessPercent })}
            </span>
          </span>
        )}
      </span>
      <span className="relative flex shrink-0 flex-wrap items-center gap-2">
        <SetDefaultVoiceButton ownerId={ownerId} voice={voice} />
      </span>
    </li>
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
