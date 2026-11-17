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
  voiceAnalysisDate,
  type Voice,
} from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { GeneratedWritingVoicesSheet } from '@/features/generate-writing-voices'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { AIAuthoringSheet } from '@/widgets/ai-authoring-studio'
import { KitHeader, KitRing } from '@/widgets/writing-kit'
import {
  ActionBar,
  ActionMenu,
  Badge,
  Button,
  Notice,
  LoadingState,
  Typography,
  buttonStyles,
  cardStyles,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'

/** Guide: 말투 — how the owner's sentences sound. The screen opens on the ONE voice new posts are
 *  written in, shown as a specimen of how it writes (its own example sentence, the AI's one-line
 *  reading of it, its signature phrases), or — while it is still being learned — how far the
 *  learning is and the one thing that moves it on. Every other voice is a row whose action is the
 *  thing that row needs: 기본으로 for a made one, 학습 글 넣기 for one still being learned. The three
 *  ways to make a voice wait behind the one 새 말투, opening from it (VOICE departure, see notes). */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  const [authoringOpen, setAuthoringOpen] = useState(false)
  const [suggestionsOpen, setSuggestionsOpen] = useState(false)
  const search = useSearch({ strict: false }) as { new?: unknown }
  const navigate = useNavigate()
  // `?new=1` asks for a new voice from elsewhere (THEME-67): it opens the sheet and is dropped
  // from the address at once.
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
  const writing = active.find((voice) => voice.isDefault)
  const others = active.filter((voice) => voice !== writing)

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}>
      <KitHeader part="voice" />

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
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
      {!isError && isPending && (
        <LoadingState
          variant="cards"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8 sm:[&>.loading-reveal]:grid-cols-1"
        />
      )}

      {!isError && !isPending && (
        <>
          <section aria-labelledby="voices-writing" className="mt-8">
            <h2
              id="voices-writing"
              className={typographyStyles({ variant: 'groupLabel', className: 'mb-2' })}
            >
              {t('page.writing', { ns: 'voices' })}
            </h2>
            {writing ? (
              <VoiceSpecimen ownerId={ownerId} voice={writing} />
            ) : (
              <div className="border-content-tertiary/40 rounded-2xl border-2 border-dashed px-5 py-6">
                <Typography variant="fieldTitle" as="p">
                  {t(active.length ? 'page.noDefault' : 'page.none', { ns: 'voices' })}
                </Typography>
                <Typography variant="body" className="text-content-secondary mt-1">
                  {t(active.length ? 'page.noDefaultNext' : 'page.noneNext', { ns: 'voices' })}
                </Typography>
              </div>
            )}
          </section>

          {others.length > 0 && (
            <section aria-labelledby="voices-others" className="mt-10">
              <h2
                id="voices-others"
                className={typographyStyles({ variant: 'groupLabel', className: 'mb-2' })}
              >
                {t('page.others', { ns: 'voices', count: others.length })}
              </h2>
              <ul className="divide-divider bg-card-bg divide-y overflow-hidden rounded-2xl shadow-sm">
                {others.map((voice) => (
                  <VoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
                ))}
              </ul>
            </section>
          )}

          {deleted.length > 0 && (
            // Closed by default: a tombstone is history. It exists so a restore stays reachable.
            <details className="mt-10">
              <summary
                className={typographyStyles({
                  variant: 'label',
                  className:
                    'active:bg-row-bg-active text-content-secondary min-h-11 cursor-pointer rounded-md py-3 select-none',
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

      <CreateVoiceSheet ownerId={ownerId} open={createOpen} onOpenChange={setCreateOpen} />
      <GeneratedWritingVoicesSheet
        ownerId={ownerId}
        open={suggestionsOpen}
        onOpenChange={setSuggestionsOpen}
      />
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
      {/* The one creation; its three ways open from it (owner: a choice opens from the control
          that needs it). */}
      <ActionBar dock="list" className="mt-auto w-full sm:w-fit">
        <ActionMenu
          label={t('page.newAria', { ns: 'voices' })}
          triggerVariant="cta"
          triggerLabel={
            <>
              <Plus aria-hidden="true" className="size-6 shrink-0" />
              {t('page.new', { ns: 'voices' })}
            </>
          }
          className="w-full sm:w-auto [&>button]:w-full"
          items={[
            {
              id: 'materials',
              label: t('page.fromWriting', { ns: 'voices' }),
              onSelect: () => setCreateOpen(true),
            },
            {
              id: 'talk',
              label: t('page.fromTalk', { ns: 'voices' }),
              onSelect: () => setAuthoringOpen(true),
            },
            {
              id: 'suggested',
              label: t('page.fromSuggestions', { ns: 'voices' }),
              onSelect: () => setSuggestionsOpen(true),
            },
          ]}
        />
      </ActionBar>
    </main>
  )
}

/** The voice new posts are written in, as a specimen of how it writes. A voice still being
 *  learned shows how far it is and the way to feed it more. */
function VoiceSpecimen({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation(['voices', 'common', 'authoring'])
  const { profile } = useVoiceProfile(ownerId, voice.made ? voice.id : '')
  const analysis = profile?.analysis
  const ai = analysis?.ai
  const sample = analysis?.syntheticSample || ai?.examples[0]?.sentence
  const fictional = Boolean(analysis?.syntheticSample) || analysis?.origin === 'synthetic'
  return (
    <article
      className={cardStyles({
        padding: 'lg',
        className: 'relative flex flex-col gap-4 overflow-hidden',
      })}
    >
      <Quote
        aria-hidden="true"
        className="text-badge-accent-bg pointer-events-none absolute -top-2 -right-2 size-32"
      />
      <div className="relative flex min-w-0 flex-wrap items-center gap-2">
        <Link
          to="/voices/$voiceId"
          params={{ voiceId: voice.id }}
          className={typographyStyles({
            variant: 'title',
            className: 'text-content-primary min-w-0 break-words hover:underline',
          })}
        >
          {voice.name}
        </Link>
        <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>
        {voice.origin === 'synthetic' && <Badge tone="neutral">{t('origin.synthetic')}</Badge>}
        {voice.made && profile && <VoiceMaterialFreshness profile={profile} compact />}
      </div>
      {voice.made ? (
        <>
          {sample && (
            <figure className="relative">
              {fictional && (
                <figcaption className={typographyStyles({ variant: 'label', className: 'mb-1' })}>
                  {t('fictional', { ns: 'authoring' })}
                </figcaption>
              )}
              <blockquote
                className={typographyStyles({
                  variant: 'fieldTitle',
                  className: 'text-content-primary text-pretty',
                })}
              >
                “{sample}”
              </blockquote>
            </figure>
          )}
          {ai?.impression && (
            <Typography variant="body" className="text-content-secondary relative">
              {ai.impression}
            </Typography>
          )}
          {ai && ai.signaturePhrases.length > 0 && (
            <div className="relative flex flex-wrap gap-1.5">
              {ai.signaturePhrases.slice(0, 5).map((phrase) => (
                <Badge key={phrase}>{phrase}</Badge>
              ))}
            </div>
          )}
          <Typography variant="meta" className="relative">
            {t('page.meta', {
              ns: 'voices',
              count: voice.materialCount,
              date: voiceAnalysisDate(voice.analyzedAt),
            })}
          </Typography>
        </>
      ) : (
        <LearningState voice={voice} />
      )}
    </article>
  )
}

/** A voice not yet made: its real readiness, and the one way to move it on. */
function LearningState({ voice }: { voice: Voice }) {
  const { t } = useTranslation('voices')
  return (
    <div className="relative flex flex-wrap items-center gap-4">
      <KitRing
        part={{ key: 'voice', state: 'partial', progress: voice.readinessPercent / 100 }}
        size="lg"
      />
      <div className="min-w-0 flex-1">
        <Typography variant="fieldTitle" as="p">
          {voice.readinessPercent >= 100
            ? t('page.analyzeLeft')
            : t('page.learning', { percent: voice.readinessPercent })}
        </Typography>
        <Typography variant="meta">
          {t('page.materials', { count: voice.materialCount })}
        </Typography>
      </div>
      <NextStep voice={voice} />
    </div>
  )
}

/** Another voice: its state at a glance and the one action it needs. The name is the row's link,
 *  stretched over the row; the action paints above it. */
function VoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation('voices')
  return (
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
      <KitRing
        part={{
          key: 'voice',
          state: voice.made ? 'ready' : 'partial',
          progress: voice.made ? 1 : voice.readinessPercent / 100,
        }}
      />
      <div className="min-w-0 flex-1">
        <Link
          to="/voices/$voiceId"
          params={{ voiceId: voice.id }}
          className={typographyStyles({
            variant: 'fieldTitle',
            className: 'text-content-primary block truncate after:absolute after:inset-0',
          })}
        >
          {voice.name}
        </Link>
        <span className={typographyStyles({ variant: 'meta', className: 'block truncate' })}>
          {voice.made
            ? t('page.meta', {
                count: voice.materialCount,
                date: voiceAnalysisDate(voice.analyzedAt),
              })
            : voice.readinessPercent >= 100
              ? t('page.analyzeLeft')
              : t('page.learning', { percent: voice.readinessPercent })}
        </span>
      </div>
      <div className={clsx('relative flex shrink-0 flex-wrap items-center gap-2')}>
        {voice.made ? (
          <SetDefaultVoiceButton ownerId={ownerId} voice={voice} />
        ) : (
          <NextStep voice={voice} />
        )}
      </div>
    </li>
  )
}

/** What moves a voice still being learned on: more writing, or — once there is enough — the
 *  analysis on the voice's own screen (which asks before it spends). */
function NextStep({ voice }: { voice: Voice }) {
  const { t } = useTranslation('voices')
  return voice.readinessPercent >= 100 ? (
    <Link
      to="/voices/$voiceId"
      params={{ voiceId: voice.id }}
      className={buttonStyles({ variant: 'secondary' })}
    >
      {t('page.analyze')}
    </Link>
  ) : (
    <Link
      to="/voices/$voiceId/materials"
      params={{ voiceId: voice.id }}
      className={buttonStyles({ variant: 'secondary' })}
    >
      {t('page.feed')}
    </Link>
  )
}

/** A tombstone keeps its `복원` (VOICE-52). */
function DeletedVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  return (
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 py-2">
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
