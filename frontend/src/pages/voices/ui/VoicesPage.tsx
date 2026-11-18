import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight, Plus } from 'lucide-react'
import { useSession } from '@/entities/session'
import {
  useVoices,
  useVoiceProfile,
  VoiceMaterialFreshness,
  VoiceRow,
  voiceAnalysisDate,
  type Voice,
} from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { GeneratedWritingVoicesSheet } from '@/features/generate-writing-voices'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { AIAuthoringSheet } from '@/widgets/ai-authoring-studio'
import { KitScreenHeader } from '@/widgets/writing-kit'
import {
  ActionMenu,
  Badge,
  Button,
  LoadingState,
  Meter,
  Notice,
  Typography,
  buttonStyles,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** Guide: 말투 — the screen opens on the voice the owner's posts are actually written in, shown
 *  the way it sounds: one of its own sentences in quotes, the AI's one-line impression of it, the
 *  phrases it reaches for and its habits. A voice still being made shows how far its meter has
 *  come and the way to keep teaching it instead. Every other voice is one row, and making one of
 *  them the writing voice is one tap on that row (VOICE-12). The three ways to make a new voice
 *  open from the ONE 새 말투 in the heading row — learn from my writing, talk to the AI, or pick
 *  up an earlier AI-made one — so no creation path sits on the page as its own button. */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  const [authoringOpen, setAuthoringOpen] = useState(false)
  const [legacyOpen, setLegacyOpen] = useState(false)
  const search = useSearch({ strict: false }) as { new?: unknown }
  const navigate = useNavigate()
  // `?new=1` (an older 새 말투 link, setup's way here) opens the learn-from-writing sheet and is
  // dropped from the address at once.
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

  // The featured voice: the default, else the most nearly made one — the same choice 홈's ring
  // reads. Only the default is the one new posts are written in (POST-101); without one a post
  // starts on 말투 없음, so the featured voice is then not named as the posts' voice.
  const writing =
    active.find((voice) => voice.isDefault) ??
    active.find((voice) => voice.made) ??
    [...active].sort((a, b) => b.readinessPercent - a.readinessPercent)[0]
  const others = active.filter((voice) => voice.id !== writing?.id)

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-4 sm:pt-8' })}>
      <KitScreenHeader
        kit="voice"
        title={t('page.title')}
        statement={t('page.statement')}
        actions={
          <ActionMenu
            label={t('page.newMenu')}
            // A second voice is the rarer errand: once one exists, its own card holds the screen's
            // main action and this steps back.
            triggerVariant={writing || isPending ? 'secondary' : 'cta'}
            triggerLabel={
              <>
                <Plus aria-hidden="true" className="size-5 shrink-0" />
                {t('page.newMenu')}
              </>
            }
            items={[
              {
                id: 'learn',
                label: t('page.method.learn'),
                description: t('page.method.learnHelp'),
                onSelect: () => setCreateOpen(true),
              },
              {
                id: 'ai',
                label: t('page.method.ai'),
                description: t('page.method.aiHelp'),
                onSelect: () => setAuthoringOpen(true),
              },
              {
                id: 'legacy',
                label: t('page.method.legacy'),
                onSelect: () => setLegacyOpen(true),
              },
            ]}
          />
        }
      />
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
      <GeneratedWritingVoicesSheet
        ownerId={ownerId}
        open={legacyOpen}
        onOpenChange={setLegacyOpen}
      />

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed')}</span>
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
          variant="detail"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8"
        />
      )}
      {!isError && !isPending && !writing && (
        <Typography
          variant="body"
          as="p"
          role="status"
          className="text-content-secondary border-divider max-w-measure mt-8 rounded-2xl border border-dashed px-5 py-8"
        >
          {t('page.none')}
        </Typography>
      )}
      {!isError && !isPending && writing && (
        <div className="mt-8 grid grid-cols-[minmax(0,1fr)] gap-10 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] lg:items-start lg:gap-8">
          <section aria-labelledby="voice-current">
            <Typography variant="eyebrow" as="h2" id="voice-current">
              {writing.isDefault ? t('page.current') : t('page.featured')}
            </Typography>
            {!writing.isDefault && (
              <Typography variant="meta" as="p" className="text-content-secondary mt-1">
                {t('page.noDefault')}
              </Typography>
            )}
            <VoicePortrait ownerId={ownerId} voice={writing} />
          </section>
          {others.length > 0 && (
            <section aria-labelledby="voice-others">
              <Typography variant="eyebrow" as="h2" id="voice-others">
                {t('page.others')}
              </Typography>
              <ul className="divide-divider -mx-3 mt-3 divide-y">
                {others.map((voice) => (
                  // A made voice takes the writing voice's place in one tap.
                  <VoiceRow
                    key={voice.id}
                    ownerId={ownerId}
                    voice={voice}
                    action={<SetDefaultVoiceButton ownerId={ownerId} voice={voice} />}
                  />
                ))}
              </ul>
            </section>
          )}
        </div>
      )}

      {!isError && !isPending && deleted.length > 0 && (
        // Closed by default: a tombstone is history. It exists at all so a restore stays reachable.
        <details className="mt-10">
          <summary
            className={typographyStyles({
              variant: 'label',
              className:
                'active:bg-row-bg-active text-content-secondary -mx-4 min-h-11 cursor-pointer rounded-md px-4 py-3 select-none',
            })}
          >
            {t('page.deleted', { count: deleted.length })}
          </summary>
          <ul className="divide-divider mt-3 divide-y">
            {deleted.map((voice) => (
              <DeletedVoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
            ))}
          </ul>
        </details>
      )}
    </main>
  )
}

/** The writing voice as it sounds (VOICE-52's directory facts, drawn large): one of its own
 *  sentences, the impression, the phrases and habits; or, until it is made, its meter. A sentence
 *  an AI-made style was read from is a fictional sample, and the card says so (VOICE-69). */
function VoicePortrait({ ownerId, voice }: { ownerId: string; voice: Voice }) {
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
        className: 'animate-rise mt-3 flex flex-col gap-5',
      })}
    >
      <div className="flex flex-wrap items-center gap-2">
        <Typography variant="title" as="h3" className="min-w-0 break-words">
          {voice.name}
        </Typography>
        {voice.isDefault && <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>}
        {voice.origin === 'synthetic' && <Badge tone="neutral">{t('origin.synthetic')}</Badge>}
        {voice.made && profile && <VoiceMaterialFreshness profile={profile} compact />}
      </div>
      {voice.made ? (
        <>
          {sample && (
            <blockquote className="border-button-cta-bg flex flex-col gap-1 border-l-4 pl-4">
              {fictional && (
                <span className={typographyStyles({ variant: 'label' })}>
                  {t('fictional', { ns: 'authoring' })}
                </span>
              )}
              <Typography variant="stepTitle" as="p" className="text-content-primary text-pretty">
                “{sample}”
              </Typography>
            </blockquote>
          )}
          {ai?.impression && (
            <Typography variant="body" as="p" className="text-content-secondary">
              {ai.impression}
            </Typography>
          )}
          {ai && ai.signaturePhrases.length > 0 && (
            <div className="flex flex-col gap-2">
              <Typography variant="label" as="p">
                {t('page.phrases')}
              </Typography>
              <p className="flex flex-wrap gap-1.5">
                {ai.signaturePhrases.slice(0, 5).map((phrase) => (
                  <Badge key={phrase} tone="accent">
                    {phrase}
                  </Badge>
                ))}
              </p>
            </div>
          )}
          {ai && ai.tics.length > 0 && (
            <div className="flex flex-col gap-2">
              <Typography variant="label" as="p">
                {t('page.tics')}
              </Typography>
              <ul className="flex flex-col gap-1">
                {ai.tics.slice(0, 3).map((tic) => (
                  <li key={tic.phrase} className={typographyStyles({ variant: 'body' })}>
                    <span className="text-content-primary">“{tic.phrase}”</span>
                    {tic.when && <span className="text-content-tertiary"> · {tic.when}</span>}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <Typography variant="meta" as="p">
            {t('page.meta', {
              count: voice.materialCount,
              date: voiceAnalysisDate(voice.analyzedAt),
            })}
          </Typography>
        </>
      ) : (
        <div className="flex flex-col gap-3">
          <Meter
            label={t('page.learningLabel')}
            value={voice.readinessPercent}
            max={100}
            valueText={`${voice.readinessPercent}%`}
          />
          <Typography variant="body" as="p" className="text-content-secondary">
            {t(voice.readinessPercent >= 100 ? 'page.learningFull' : 'page.learningMore')}
          </Typography>
        </div>
      )}
      <Link
        to="/voices/$voiceId"
        params={{ voiceId: voice.id }}
        className={buttonStyles({
          variant: voice.made ? 'secondary' : 'cta',
          className: 'self-start',
        })}
      >
        {voice.made
          ? t('page.openVoice')
          : voice.readinessPercent >= 100
            ? t('page.goAnalyse')
            : t('page.continueLearning')}
        <ChevronRight aria-hidden="true" className="size-4" />
      </Link>
    </article>
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
