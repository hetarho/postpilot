import { Link, Outlet, useParams } from '@tanstack/react-router'
import { FileText, History, IdCard } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { DeleteVoiceButton } from '@/features/delete-voice'
import { RenameVoiceField } from '@/features/rename-voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import {
  Button,
  Notice,
  TabLinks,
  Typography,
  typographyStyles,
  type TabLink,
  pageStyles,
} from '@/shared/ui'

/** The three tabs of one voice, in one list so the row and the routes cannot drift — the same
 *  reason `AuthenticatedLayout` keeps one `DESTINATIONS`. They are sub-navigation inside one voice,
 *  not destinations of their own. Every tab carries an icon and a short caption so the row can
 *  compact itself instead of horizontally scrolling on a phone (TabLinks' container mode). */
const VOICE_TABS: readonly (Omit<TabLink, 'params' | 'label' | 'shortLabel'> & {
  labelKey: 'profile' | 'versions' | 'materials'
})[] = [
  { to: '/voices/$voiceId', labelKey: 'profile', icon: IdCard },
  { to: '/voices/$voiceId/materials', labelKey: 'materials', icon: FileText },
  { to: '/voices/$voiceId/versions', labelKey: 'versions', icon: History },
]

/** The frame of `/voices/$voiceId`: which voice this is, its state, and the tab row. The voice
 *  comes from the directory rather than from the profile so an unknown or foreign id can say so
 *  before any tab asks for a profile that does not exist. */
export function VoiceLayout() {
  const { t } = useTranslation(['nav', 'voices', 'common'])
  const { voiceId = '' } = useParams({ strict: false })
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
          {/* The title row carries every lifecycle action of the voice (VOICE-54): the rename,
              then 기본으로 설정 or 기본 해제 on a made voice, then 삭제 — a directory row is one
              target that leads here, and this is the screen that shows what they act on. */}
          <div className="mt-4 flex flex-wrap items-start gap-x-3 gap-y-2">
            <RenameVoiceField ownerId={ownerId} voice={voice} className="min-w-0 flex-1">
              <Typography variant="display" className="min-w-0 break-words">
                {voice.name}
              </Typography>
            </RenameVoiceField>
            {!voice.deleted && (
              <div className="flex shrink-0 flex-wrap items-center gap-2">
                <SetDefaultVoiceButton ownerId={ownerId} voice={voice} />
                <DeleteVoiceButton ownerId={ownerId} voice={voice} />
              </div>
            )}
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
