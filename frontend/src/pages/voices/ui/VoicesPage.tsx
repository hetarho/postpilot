import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useVoices, type Voice } from '@/entities/voice'
import { CreateVoiceForm } from '@/features/create-voice'
import { DeleteVoiceButton } from '@/features/delete-voice'
import { RenameVoiceField } from '@/features/rename-voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { Badge, Button, Notice, Typography, typographyStyles } from '@/shared/ui'

/** The account's voices (PRD §3.4): the active ones first, then the tombstones. Composition only —
 *  every action is its own feature, and the rows are links into one voice's profile. */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <Typography variant="display">{t('title', { ns: 'voices' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.description', { ns: 'voices' })}
      </Typography>

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
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!isError && !isPending && (
        <>
          <section aria-labelledby="active-voices-heading" className="mt-8">
            <Typography variant="title" id="active-voices-heading">
              {t('page.active', { ns: 'voices' })}
            </Typography>
            <ul className="divide-divider mt-3 divide-y">
              {active.map((voice) => (
                <VoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
              ))}
            </ul>
          </section>

          <section aria-labelledby="create-voice-heading" className="mt-10">
            <Typography variant="title" id="create-voice-heading">
              {t('page.new', { ns: 'voices' })}
            </Typography>
            <CreateVoiceForm ownerId={ownerId} className="mt-3" />
          </section>

          {deleted.length > 0 && (
            <section aria-labelledby="deleted-voices-heading" className="mt-12">
              <Typography variant="title" id="deleted-voices-heading">
                {t('page.deleted', { ns: 'voices' })}
              </Typography>
              <Typography variant="body" className="text-content-secondary mt-2">
                {t('page.deletedHelp', { ns: 'voices' })}
              </Typography>
              <ul className="divide-divider mt-3 divide-y">
                {deleted.map((voice) => (
                  <DeletedVoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </main>
  )
}

/** The name is the link into the voice; the pencil beside it renames in place. The actions sit on
 *  their own row so the phone gets three full-height targets instead of a crushed strip (§4.1). */
function VoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation('common')
  return (
    <li className="py-3">
      <RenameVoiceField ownerId={ownerId} voice={voice}>
        <div className="flex min-h-11 flex-wrap items-center gap-2">
          <Link
            to="/voices/$voiceId"
            params={{ voiceId: voice.id }}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center underline',
            })}
          >
            <span className="truncate">{voice.name}</span>
          </Link>
          {voice.isDefault && <Badge tone="accent">{t('state.default')}</Badge>}
          <Badge>{t(`contentLanguage.${voice.sourceLanguage}`)}</Badge>
        </div>
      </RenameVoiceField>
      {!voice.isDefault && (
        <div className="mt-2 flex flex-wrap gap-2">
          <SetDefaultVoiceButton ownerId={ownerId} voiceId={voice.id} />
          <DeleteVoiceButton ownerId={ownerId} voice={voice} />
        </div>
      )}
    </li>
  )
}

function DeletedVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation('common')
  return (
    <li className="py-3">
      <RenameVoiceField ownerId={ownerId} voice={voice}>
        <div className="flex min-h-11 flex-wrap items-center gap-2">
          <Link
            to="/voices/$voiceId"
            params={{ voiceId: voice.id }}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center underline',
            })}
          >
            <span className="truncate">{voice.name}</span>
          </Link>
          <Badge tone="warning">{t('state.deleted')}</Badge>
          <Badge>{t(`contentLanguage.${voice.sourceLanguage}`)}</Badge>
        </div>
      </RenameVoiceField>
      <div className="mt-2 flex flex-wrap gap-2">
        <RestoreVoiceButton ownerId={ownerId} voiceId={voice.id} />
      </div>
    </li>
  )
}
