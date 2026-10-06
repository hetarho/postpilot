import { useEffect, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useIsMutating, useIsFetching } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSetup } from '@/features/complete-setup'
import { Button, Notice, ProgressBar, Typography, pageStyles } from '@/shared/ui'
import { ModelSetup } from './ModelSetup'
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
  const mutating = useIsMutating() > 0
  const fetching = useIsFetching() > 0
  const busy = state.phase === 'saving' || state.phase === 'running' || mutating || fetching
  const heading = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (state.phase !== 'checking')
      heading.current?.querySelector('h1')?.focus({ preventScroll: true })
  }, [state.step, state.phase])
  useEffect(() => {
    if (state.phase === 'completed') void navigate({ to: state.target, replace: true })
  }, [state.phase, state.target, navigate])
  const loadingError =
    state.phase === 'checking' &&
    (controller.availability.status === 'failed' || controller.modelError)
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
                  controller.retryModels()
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
    <main className={pageStyles({ className: 'flex flex-1 flex-col py-10 sm:py-16' })}>
      <div ref={heading} className="mx-auto w-full max-w-xl">
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
        <Typography variant="display" tabIndex={-1} className="mt-10 focus:outline-none">
          {t(`setup.${state.step}.title`)}
        </Typography>
        <Typography variant="body" className="text-content-secondary mt-4">
          {t(`setup.${state.step}.description`)}
        </Typography>
        <div className="mt-8">
          {state.step === 'welcome' && (
            <div className="flex flex-col gap-3">
              <Button variant="cta" onClick={() => controller.next()}>
                {t('setup.welcome.action')}
              </Button>
              <Button variant="ghost" onClick={() => controller.defer('/')}>
                {t('setup.later')}
              </Button>
            </div>
          )}
          {state.step === 'models' && <ModelSetup controller={controller} />}
          {state.step === 'voice' && <VoiceSetup ownerId={ownerId} controller={controller} />}
          {(state.step === 'post-template' || state.step === 'clip-template') && (
            <TemplateSetup
              key={state.step}
              ownerId={ownerId}
              kind={state.step}
              controller={controller}
            />
          )}
          {state.step === 'ready' && (
            <div className="flex flex-col gap-3">
              <Button variant="cta" onClick={() => controller.finish('/')}>
                {t('setup.ready.home')}
              </Button>
              <Button variant="secondary" onClick={() => controller.finish('/posts/new')}>
                {t('setup.ready.post')}
              </Button>
              <Button variant="secondary" onClick={() => controller.finish('/clips/new')}>
                {t('setup.ready.clip')}
              </Button>
            </div>
          )}
        </div>
        {state.step !== 'welcome' && state.step !== 'ready' && (
          <div className="mt-8 flex flex-wrap justify-between gap-3">
            <Button variant="ghost" disabled={busy} onClick={controller.back}>
              {t('setup.back')}
            </Button>
            <Button variant="ghost" disabled={busy} onClick={controller.skip}>
              {t('setup.skip')}
            </Button>
          </div>
        )}
        <Typography variant="meta" className="mt-8 block">
          {t('setup.optional')}
        </Typography>
      </div>
    </main>
  )
}
