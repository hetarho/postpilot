import { clsx } from 'clsx'
import { useEffect, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSetup } from '@/features/complete-setup'
import { ArrowLeft, ChevronRight, Clapperboard, House, PenLine } from 'lucide-react'
import {
  ActionBar,
  Button,
  ListRowIcon,
  Notice,
  ProgressBar,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import type { VoiceLearningNavigation } from '@/features/prepare-writing-voice'
import { VoiceSetup } from './VoiceSetup'
import { TemplateSetup } from './TemplateSetup'

export function CreationSetup({ restart = false }: { restart?: boolean }) {
  const { user } = useSession()
  return (
    <AccountSetup key={`${user?.id ?? ''}:${restart}`} ownerId={user?.id ?? ''} restart={restart} />
  )
}
function AccountSetup({ ownerId, restart }: { ownerId: string; restart: boolean }) {
  const { t } = useTranslation('creation')
  const navigate = useNavigate()
  const controller = useSetup(ownerId, restart)
  const { state } = controller
  const introStep = state.step === 'welcome' || state.step === 'ready'
  const visible = state.phase !== 'checking'
  const busy = state.phase === 'saving' || state.phase === 'running'
  const voiceNavigation = useRef<VoiceLearningNavigation | null>(null)
  const heading = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (visible) heading.current?.querySelector('h1')?.focus({ preventScroll: true })
  }, [state.step, visible])
  useEffect(() => {
    if (state.phase !== 'completed') return
    // A creation opened from here does not offer again what the owner has just answered
    // (POST-121, CLIP-223).
    if (state.target === '/') void navigate({ to: '/', replace: true })
    else void navigate({ to: state.target, search: { from: 'setup' }, replace: true })
  }, [state.phase, state.target, navigate])
  const loadingError = state.phase === 'checking' && controller.availability.status === 'failed'
  const templateStep = state.step === 'post-template' || state.step === 'clip-template'
  const index =
    state.step === 'welcome'
      ? 0
      : state.step === 'ready'
        ? state.plan.length
        : state.plan.indexOf(state.step) + 1
  if (state.phase === 'checking')
    return (
      <main className={pageStyles({ className: 'flex flex-1 flex-col justify-center gap-6' })}>
        {loadingError ? (
          <>
            <Notice tone="danger" role="alert">
              {t('setup.loadFailed')}
            </Notice>
            <div className="flex flex-wrap gap-3">
              <Button
                variant="cta"
                onClick={() => {
                  controller.availability.retry()
                }}
              >
                {t('setup.retry')}
              </Button>
              <Button variant="ghost" onClick={() => controller.defer('/')}>
                {t('setup.continue')}
              </Button>
            </div>
          </>
        ) : (
          <Typography variant="body" role="status">
            {t('setup.checking')}
          </Typography>
        )}
      </main>
    )
  return (
    // Guide: a conversation — one friendly question per screen in one centred column, its answers
    // as big tiles, and the way back and the skip in the thumb zone.
    <main
      className={pageStyles({
        width: templateStep ? 'wide' : 'prose',
        className: 'flex flex-1 flex-col pt-4 pb-0 sm:pt-12 sm:pb-0',
      })}
    >
      <div key={state.step} className="animate-step-in flex w-full flex-1 flex-col gap-6">
        <section ref={heading} className="min-w-0">
          <div className="flex items-center justify-between gap-4">
            <Typography variant="meta">{t('setup.progress')}</Typography>
            {state.plan.length > 0 && (
              <Typography variant="meta">
                {t('setup.step', {
                  current: Math.min(index, state.plan.length),
                  total: state.plan.length,
                })}
              </Typography>
            )}
          </div>
          <ProgressBar
            label={t('setup.progress')}
            done={index}
            total={Math.max(1, state.plan.length)}
            className="mt-2 md:mt-3"
          />
          {/* The question, large, on every screen and every width. */}
          <Typography
            variant="stepTitle"
            as="h1"
            tabIndex={-1}
            className="mt-6 text-balance focus:outline-none md:mt-10"
          >
            {t(`setup.${state.step}.title`)}
          </Typography>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-3">
            {t(`setup.${state.step}.description`)}
          </Typography>
          {state.step === 'ready' && (
            // The last answer as three big tiles: where to go now.
            <div className="mt-8 grid gap-3">
              {(
                [
                  ['/', 'setup.ready.home', House],
                  ['/posts/new', 'setup.ready.post', PenLine],
                  ['/clips/new', 'setup.ready.clip', Clapperboard],
                ] as const
              ).map(([target, label, Icon], position) => (
                <button
                  key={target}
                  type="button"
                  onClick={() => controller.finish(target)}
                  className={cardStyles({
                    interactive: true,
                    className: clsx(
                      'flex min-h-20 w-full items-center gap-4 text-left',
                      position === 0 && 'bg-badge-accent-bg',
                    ),
                  })}
                >
                  <ListRowIcon icon={Icon} size="lg" />
                  <span className={typographyStyles({ variant: 'title', className: 'flex-1' })}>
                    {t(label)}
                  </span>
                  <ChevronRight
                    aria-hidden="true"
                    className="text-content-tertiary size-6 shrink-0"
                  />
                </button>
              ))}
            </div>
          )}
          {introStep && (
            <Typography variant="meta" className="mt-6 block">
              {t('setup.optional')}
            </Typography>
          )}
        </section>
        <div className="min-w-0">
          <div>
            {state.step === 'welcome' && state.plan.length > 0 && (
              // What the conversation will ask, as numbered cards.
              <ol aria-label={t('setup.overview.label')} className="grid gap-3">
                {state.plan.map((form, position) => (
                  <li
                    key={form}
                    className={cardStyles({
                      padding: 'sm',
                      className: 'flex min-w-0 items-start gap-4 shadow-sm',
                    })}
                  >
                    <span
                      aria-hidden="true"
                      className={typographyStyles({
                        variant: 'label',
                        className:
                          'bg-badge-accent-bg text-badge-accent-fg flex size-10 shrink-0 items-center justify-center rounded-full',
                      })}
                    >
                      {position + 1}
                    </span>
                    <span className="flex min-w-0 flex-col gap-1">
                      <Typography variant="fieldTitle" as="h2">
                        {t(`setup.overview.${form}.title`)}
                      </Typography>
                      <Typography variant="body" className="text-content-secondary">
                        {t(`setup.overview.${form}.description`)}
                      </Typography>
                    </span>
                  </li>
                ))}
              </ol>
            )}
            {state.step === 'voice' && (
              <VoiceSetup
                ownerId={ownerId}
                controller={controller}
                onNavigationChange={(navigation) => {
                  voiceNavigation.current = navigation
                }}
              />
            )}
            {(state.step === 'post-template' || state.step === 'clip-template') && (
              <TemplateSetup
                key={state.step}
                ownerId={ownerId}
                kind={state.step}
                controller={controller}
              />
            )}
          </div>
          {!introStep && (
            <Typography variant="meta" className="mt-4 block md:mt-8">
              {t('setup.optional')}
            </Typography>
          )}
        </div>
      </div>
      {/* The thumb zone: the screen's one big answer on the welcome, and the way back and the
          skip on every question after it. */}
      {state.step === 'welcome' && (
        <ActionBar dock="step" className="mt-auto">
          <div className="flex flex-col gap-1">
            <Button variant="cta" size="hero" className="w-full" onClick={() => controller.next()}>
              {t('setup.welcome.action')}
            </Button>
            <Button variant="ghost" className="w-full" onClick={() => controller.defer('/')}>
              {t('setup.later')}
            </Button>
          </div>
        </ActionBar>
      )}
      {state.step !== 'welcome' && state.step !== 'ready' && (
        <ActionBar dock="step" className="mt-auto">
          <div className="flex items-center justify-between gap-3">
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => {
                if (state.step === 'voice' && voiceNavigation.current?.back()) return
                controller.back()
              }}
            >
              <ArrowLeft aria-hidden="true" className="size-5" />
              {t('setup.back')}
            </Button>
            <Button variant="ghost" disabled={busy} onClick={controller.skip}>
              {t('setup.skip')}
            </Button>
          </div>
        </ActionBar>
      )}
    </main>
  )
}
