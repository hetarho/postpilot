import { Link, Outlet, useParams } from '@tanstack/react-router'
import { ClipboardCheck, FolderInput, History, IdCard, Scale } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { RenameVoiceField } from '@/features/rename-voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import {
  Badge,
  Button,
  Notice,
  TabLinks,
  Typography,
  typographyStyles,
  type TabLink,
  pageStyles,
} from '@/shared/ui'

/** The five tabs of one voice, in one list so the row and the routes cannot drift — the same
 *  reason `AuthenticatedLayout` keeps one `DESTINATIONS`. They are sub-navigation inside one voice,
 *  not destinations of their own. Every tab carries an icon and a short caption so the row can
 *  compact itself instead of horizontally scrolling on a phone (TabLinks' container mode). */
const VOICE_TABS: readonly (Omit<TabLink, 'params' | 'label' | 'shortLabel'> & {
  labelKey: 'profile' | 'versions' | 'import' | 'rules' | 'validations'
})[] = [
  { to: '/voices/$voiceId', labelKey: 'profile', icon: IdCard },
  { to: '/voices/$voiceId/versions', labelKey: 'versions', icon: History },
  { to: '/voices/$voiceId/import', labelKey: 'import', icon: FolderInput },
  { to: '/voices/$voiceId/rules', labelKey: 'rules', icon: Scale },
  { to: '/voices/$voiceId/validations', labelKey: 'validations', icon: ClipboardCheck },
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
    <div className={pageStyles({ width: 'wide' })}>
      <Link
        to="/voices"
        className={typographyStyles({
          variant: 'label',
          className:
            'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center underline',
        })}
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
        <Typography variant="body" role="status" className="text-content-tertiary mt-4">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      ) : !voice ? (
        <Typography variant="body" role="alert" className="text-notice-danger-fg mt-4">
          {t('missing', { ns: 'voices' })}
        </Typography>
      ) : (
        <>
          {/* The name is edited where the voice is, not on the directory: a row there is one
              target that leads here, and this is the screen that shows what it is being named. */}
          <RenameVoiceField ownerId={ownerId} voice={voice} className="mt-4">
            <div className="flex flex-wrap items-center gap-2">
              <Typography variant="display" className="min-w-0 break-words">
                {voice.name}
              </Typography>
              {voice.isDefault && (
                <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>
              )}
              {voice.deleted && (
                <Badge tone="warning">{t('state.deleted', { ns: 'common' })}</Badge>
              )}
              <Badge>{t(`contentLanguage.${voice.sourceLanguage}`, { ns: 'common' })}</Badge>
            </div>
          </RenameVoiceField>
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
              shortLabel: t(`voice.short.${labelKey}`, { ns: 'nav' }),
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
