import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useVoices, voiceAnalysisDate, type Voice } from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  Typography,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'

/** 내 글's row: two stacked lines on a phone, one line on the desk, the whole row one link
 *  (VOICE-52, POST-43). */
const rowClass =
  'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 flex-col items-start justify-center gap-1 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:gap-4 lg:px-8'

/** The account's voices (VOICE-52): one list in 내 글's row shape and the one action that adds
 *  to it. Every lifecycle action lives on the voice's own title row, so a row here is nothing
 *  but a way into one voice. */
export function VoicesPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)

  return (
    // The page gutter lives on each block rather than on `main`, so the rows run edge to edge
    // the way 내 글's do (THEME-23).
    <main
      className={pageStyles({
        width: 'wide',
        gutters: false,
        className: 'flex flex-1 flex-col pt-4 sm:pt-6 lg:pt-8',
      })}
    >
      <div className="px-4 sm:px-6 lg:px-8">
        <Typography variant="display">{t('title', { ns: 'voices' })}</Typography>
      </div>

      {isError && (
        <Notice tone="danger" role="alert" className="mx-4 mt-8 sm:mx-6">
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
      {/* One live region for both states, so finishing the load is a text change inside it
          rather than two nodes swapping (THEME-33). */}
      {!isError && (isPending || active.length === 0) && (
        <Typography
          variant="body"
          role="status"
          className="text-content-tertiary mt-8 px-4 sm:px-6 lg:px-8"
        >
          {isPending ? t('state.loading', { ns: 'common' }) : t('page.empty', { ns: 'voices' })}
        </Typography>
      )}

      {!isError && !isPending && (
        <>
          {active.length > 0 && (
            <ul className="divide-divider mt-4 shrink-0 divide-y">
              {active.map((voice) => (
                <li key={voice.id}>
                  <Link to="/voices/$voiceId" params={{ voiceId: voice.id }} className={rowClass}>
                    <VoiceRowContent voice={voice} />
                  </Link>
                </li>
              ))}
            </ul>
          )}

          {deleted.length > 0 && (
            // Closed by default: a tombstone is history, and the list the user came for is the
            // active one. It exists at all so a restore stays reachable.
            <details className="mt-10">
              <summary
                className={typographyStyles({
                  variant: 'label',
                  className:
                    'active:bg-row-bg-active text-content-secondary mx-4 min-h-11 cursor-pointer rounded-md px-4 py-3 select-none sm:mx-6 lg:mx-8',
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

          {/* One instance at every width: the trigger owns the sheet's open state. It docks at
              every width — `mt-auto` puts it below a short list, `sticky` keeps it there once
              the list is long enough to scroll (THEME-24). */}
          <ActionBar
            dock="list"
            ariaLabel={t('create.dockAria', { ns: 'voices' })}
            className="mt-auto"
          >
            <CreateVoiceSheet ownerId={ownerId} />
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** A row's two lines: the name, then the 기본 badge and the meta line — `만드는 중` until the voice
 *  is made, then how many 학습 글 it holds and the day its analysis was published. */
function VoiceRowContent({ voice }: { voice: Voice }) {
  const { t } = useTranslation(['voices', 'common'])
  return (
    <>
      <Typography
        variant="label"
        className="text-content-primary w-full truncate lg:w-auto lg:min-w-0 lg:flex-1"
      >
        {voice.name}
      </Typography>
      <span className="flex w-full min-w-0 items-center gap-2 lg:w-auto lg:shrink-0 lg:justify-end">
        {voice.isDefault && <Badge tone="accent">{t('state.default', { ns: 'common' })}</Badge>}
        <span className={typographyStyles({ variant: 'meta', className: 'truncate' })}>
          {voice.made
            ? t('page.meta', {
                ns: 'voices',
                count: voice.materialCount,
                date: voiceAnalysisDate(voice.analyzedAt),
              })
            : t('page.making', { ns: 'voices' })}
        </span>
      </span>
    </>
  )
}

/** A tombstone keeps its `복원` (VOICE-52). The link stretches over the row through its
 *  `::after` and the button paints above that layer, so nothing interactive nests in the
 *  anchor. */
function DeletedVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  return (
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
      <div className="relative ml-auto flex shrink-0 items-center">
        <RestoreVoiceButton ownerId={ownerId} voiceId={voice.id} />
      </div>
    </li>
  )
}
