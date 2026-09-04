import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useVoices, type Voice } from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { DeleteVoiceButton } from '@/features/delete-voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  Typography,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'

/** The account's voices (PRD §3.4): a list, and the one action that adds to it. Composition only —
 *  every action is its own feature, and a row is a way into one voice. Renaming lives on the
 *  voice's own screen, so the directory carries nothing but the list. */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
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
            {/* Rows are full-bleed against the page gutter, so the list cancels it (§4.2). */}
            <ul className="divide-divider -mx-4 mt-3 divide-y sm:-mx-6 lg:-mx-8">
              {active.map((voice) => (
                <VoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
              ))}
            </ul>
          </section>

          {deleted.length > 0 && (
            // Closed by default: a tombstone is history, and the list the user came for is the
            // active one. It exists at all so a restore stays reachable.
            <details className="mt-10">
              <summary
                className={typographyStyles({
                  variant: 'label',
                  className:
                    'active:bg-row-bg-active text-content-secondary min-h-11 cursor-pointer rounded-md px-4 py-3 select-none',
                })}
              >
                {t('page.deleted', { ns: 'voices', count: deleted.length })}
              </summary>
              <Typography variant="body" className="text-content-secondary mt-2 px-4">
                {t('page.deletedHelp', { ns: 'voices' })}
              </Typography>
              <ul className="divide-divider -mx-4 mt-3 divide-y sm:-mx-6">
                {deleted.map((voice) => (
                  <VoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
                ))}
              </ul>
            </details>
          )}

          {/* One instance at every width, not a phone bar plus a desktop copy: the trigger owns
              the sheet's open state, and two of them would be two overlays waiting to be opened.
              On the phone it docks — `mt-auto` puts it below a short list, `sticky` keeps it there
              once the list is long enough to scroll (§4.3). From `sm:` up the card dissolves and
              it is simply the list's last row. The action spans the column at every width: a lone
              button left-aligned inside a full-width bar reads as a stray control rather than as
              the one thing this screen adds. */}
          <ActionBar
            dock="phone"
            ariaLabel={t('create.dockAria', { ns: 'voices' })}
            className="mt-auto"
          >
            <CreateVoiceSheet ownerId={ownerId} className="w-full" />
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** One voice, one target. The link stretches over the whole row through its `::after`, so the
 *  padding, the badges and the empty space all navigate, while the lifecycle controls paint above
 *  that layer and act without navigating — a row is one target, not a row with buttons inside it
 *  (§4.1), and nothing interactive is nested inside the anchor. */
function VoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  const { t } = useTranslation('common')
  return (
    // `min-h-16`, not the list row's usual `min-h-11`, and `py-2` rather than `py-3`: the
    // lifecycle controls keep the 44px touch floor (§4.1), so a row that carries them is 44 + its
    // padding tall while the default voice — the one row that offers neither, since the server
    // refuses both for it — stayed at 44. The list was one short row among tall ones. The floor is
    // now set by the tallest thing a row can hold, so every row is 64px and the controls sit
    // inside it instead of stretching it (§4.2).
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
      {voice.isDefault && <Badge tone="accent">{t('state.default')}</Badge>}
      {voice.deleted && <Badge tone="warning">{t('state.deleted')}</Badge>}
      <Badge>{t(`contentLanguage.${voice.sourceLanguage}`)}</Badge>
      {(voice.deleted || !voice.isDefault) && (
        <div className="relative ml-auto flex shrink-0 flex-wrap items-center gap-2">
          {voice.deleted ? (
            <RestoreVoiceButton ownerId={ownerId} voiceId={voice.id} />
          ) : (
            <>
              <SetDefaultVoiceButton ownerId={ownerId} voiceId={voice.id} />
              <DeleteVoiceButton ownerId={ownerId} voice={voice} />
            </>
          )}
        </div>
      )}
    </li>
  )
}
