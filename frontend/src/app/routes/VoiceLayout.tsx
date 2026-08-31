import { Link, Outlet, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { Badge, Button, Notice, TabLinks, type TabLink } from '@/shared/ui'

/** The five tabs of one voice, in one list so the row and the routes cannot drift — the same
 *  reason `AuthenticatedLayout` keeps one `DESTINATIONS`. They are sub-navigation inside one voice,
 *  not destinations of their own. */
const VOICE_TABS: readonly (Omit<TabLink, 'params' | 'label'> & {
  labelKey: 'profile' | 'versions' | 'import' | 'rules' | 'validations'
})[] = [
  { to: '/voices/$voiceId', labelKey: 'profile' },
  { to: '/voices/$voiceId/versions', labelKey: 'versions' },
  { to: '/voices/$voiceId/import', labelKey: 'import' },
  { to: '/voices/$voiceId/rules', labelKey: 'rules' },
  { to: '/voices/$voiceId/validations', labelKey: 'validations' },
]

/** The frame of `/voices/$voiceId`: which voice this is, its state, and the tab row. The voice
 *  comes from the directory rather than from the profile so an unknown or foreign id can say so
 *  before any tab asks for a profile that does not exist. */
export function VoiceLayout() {
  const { t } = useTranslation(['nav', 'voices', 'common'])
  const { voiceId } = useParams({ from: '/authenticated/voices/$voiceId' })
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { voices, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  const voice = voices.find((candidate) => candidate.id === voiceId)

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <Link
        to="/voices"
        className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm underline"
      >
        {t('voice.backToList', { ns: 'nav' })}
      </Link>
      {isError ? (
        <Notice tone="danger" role="alert" className="mt-4">
          <span>{t('voiceLoadFailed', { ns: 'voices' })}</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      ) : isPending ? (
        <p role="status" className="text-content-tertiary mt-4 text-sm">
          {t('state.loading', { ns: 'common' })}
        </p>
      ) : !voice ? (
        <p role="alert" className="text-notice-danger-fg mt-4 text-sm">
          {t('missing', { ns: 'voices' })}
        </p>
      ) : (
        <>
          <div className="mt-4 flex flex-wrap items-center gap-2">
            <h1 className="min-w-0 text-2xl font-semibold tracking-tight break-words">
              {voice.name}
            </h1>
            {voice.isDefault && <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>}
            {voice.deleted && <Badge tone="warning">{t('state.deleted', { ns: 'common' })}</Badge>}
            <Badge>{t(`contentLanguage.${voice.sourceLanguage}`, { ns: 'common' })}</Badge>
          </div>
          {voice.deleted && (
            <Notice tone="warning" role="status" className="mt-4">
              <span className="w-full min-w-0">{t('deletedWarning', { ns: 'voices' })}</span>
              <RestoreVoiceButton
                ownerId={ownerId}
                voiceId={voice.id}
                variant="ghost"
                className="text-notice-warning-fg shrink-0 underline"
              />
            </Notice>
          )}
          <TabLinks
            items={VOICE_TABS.map(({ labelKey, ...tab }) => ({
              ...tab,
              label: t(`voice.${labelKey}`, { ns: 'nav' }),
              params: { voiceId },
            }))}
            ariaLabel={t('voice.settings', { ns: 'nav' })}
            className="mt-4"
          />
          <Outlet />
        </>
      )}
    </div>
  )
}
