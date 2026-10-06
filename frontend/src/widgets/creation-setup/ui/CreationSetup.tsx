import { clsx } from 'clsx'
import { useEffect, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSetup } from '@/features/complete-setup'
import { Button, Notice, ProgressBar, Typography, pageStyles } from '@/shared/ui'
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
    if (state.phase === 'completed') void navigate({ to: state.target, replace: true })
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
    <main
      className={pageStyles({ width: 'board', className: 'flex flex-1 flex-col py-10 sm:py-16' })}
    >
      <div className="grid w-full gap-10 md:grid-cols-2 md:items-start md:gap-12 xl:grid-cols-3">
        <section
          ref={heading}
          className={clsx(
            'min-w-0',
            templateStep ? 'md:col-span-2 xl:col-span-3' : 'md:col-span-1',
          )}
        >
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
            className="mt-3"
          />
          <Typography
            variant={introStep ? 'display' : 'title'}
            as="h1"
            tabIndex={-1}
            className="mt-10 focus:outline-none"
          >
            {t(`setup.${state.step}.title`)}
          </Typography>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-4">
            {t(`setup.${state.step}.description`)}
          </Typography>
          {introStep && (
            <>
              <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
                {state.step === 'welcome' ? (
                  <>
                    <Button variant="cta" onClick={() => controller.next()}>
                      {t('setup.welcome.action')}
                    </Button>
                    <Button variant="ghost" onClick={() => controller.defer('/')}>
                      {t('setup.later')}
                    </Button>
                  </>
                ) : (
                  <>
                    <Button variant="cta" onClick={() => controller.finish('/')}>
                      {t('setup.ready.home')}
                    </Button>
                    <Button variant="ghost" onClick={() => controller.finish('/posts/new')}>
                      {t('setup.ready.post')}
                    </Button>
                    <Button variant="ghost" onClick={() => controller.finish('/clips/new')}>
                      {t('setup.ready.clip')}
                    </Button>
                  </>
                )}
              </div>
              <Typography variant="meta" className="mt-6 block">
                {t('setup.optional')}
              </Typography>
            </>
          )}
        </section>
        <div
          className={clsx(
            'min-w-0',
            templateStep ? 'md:col-span-2 xl:col-span-3' : 'md:col-span-1 xl:col-span-2',
          )}
        >
          <div>
            {introStep && state.plan.length > 0 && (
              <ol
                aria-label={t('setup.overview.label')}
                className="grid gap-8 sm:grid-cols-2 xl:grid-cols-3"
              >
                {state.plan.map((form, position) => (
                  <li key={form} className="min-w-0 space-y-3">
                    <Typography variant="meta" className="text-content-tertiary block">
                      {String(position + 1).padStart(2, '0')}
                    </Typography>
                    <Typography variant="fieldTitle" as="h2">
                      {t(`setup.overview.${form}.title`)}
                    </Typography>
                    <Typography variant="body" className="text-content-secondary">
                      {t(`setup.overview.${form}.description`)}
                    </Typography>
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
          {state.step !== 'welcome' && state.step !== 'ready' && (
            <div className="mt-8 flex flex-wrap justify-between gap-3">
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => {
                  if (state.step === 'voice' && voiceNavigation.current?.back()) return
                  controller.back()
                }}
              >
                {t('setup.back')}
              </Button>
              <Button variant="ghost" disabled={busy} onClick={controller.skip}>
                {t('setup.skip')}
              </Button>
            </div>
          )}
          {!introStep && (
            <Typography variant="meta" className="mt-8 block">
              {t('setup.optional')}
            </Typography>
          )}
        </div>
      </div>
    </main>
  )
}
