import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  clipReturnDestination,
  clipChannel,
  type ClipChannel,
  markClipHistoryReturn,
  useClipProject,
} from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { ClipProjectForm } from '@/features/edit-clip-project'
import { CreationOfferSheet, useCreationOffer } from '@/features/offer-creation-setup'
import { ClipTopRow, ClipStatusLine, ClipWorkspace } from '@/widgets/clip-workspace'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Typography, pageStyles } from '@/shared/ui'
import { ClipImportFeedback } from './ClipImportFeedback'

/** `/clips/new` — a project that does not exist yet. It has no lifecycle, so it shows no step
 *  bar and no delete: just the settings and the one committing action that mints it, which stays
 *  explicit because the ratio it carries can never be changed again (CLIP-9, CLIP-39).
 *
 *  An account with no video template opens it on the offer to make one (CLIP-223), so the form
 *  waits for that directory: the sheet has to stand before anything is typed. */
function NewClip({
  ownerId,
  channel,
  handedTemplate,
  fromSetup,
  returnTo,
}: {
  ownerId: string
  channel: ClipChannel
  /** The template the offer's wizard just saved. */
  handedTemplate?: string
  fromSetup: boolean
  returnTo: { href: string; label: string; onReturn: () => Promise<void> }
}) {
  const { t } = useTranslation('common')
  const navigate = useNavigate()
  // Back from the offer's own wizard, or from setup's finish: the owner has just answered it.
  const offer = useCreationOffer({
    ownerId,
    kind: 'clip',
    suppressed: !!handedTemplate || fromSetup,
  })
  return (
    <>
      {/* The status region is mounted before there is a project to have a status: a live region
          inserted already holding its message announces nothing. */}
      <ClipTopRow
        returnTo={returnTo}
        status={
          <ClipStatusLine
            project={undefined}
            job={undefined}
            upload={{ phase: 'idle' }}
            correction="clean"
            save={{ failing: false, label: '' }}
            className="order-last w-full"
          />
        }
      />
      {offer.settled ? (
        <ClipProjectForm ownerId={ownerId} channel={channel} initialTemplateId={handedTemplate} />
      ) : (
        <Typography variant="body" role="status" className="mt-8">
          {t('state.loading')}
        </Typography>
      )}
      <CreationOfferSheet
        kind="clip"
        open={offer.open}
        missing={offer.missing}
        onCreateTemplate={() => {
          offer.close()
          // 저장 comes back to a new clip in this channel starting from it (CLIP-223).
          void navigate({ to: '/video-templates/new', search: { channel, clip: 'new' } })
        }}
        onClose={offer.close}
        onDismiss={offer.dismiss}
      />
    </>
  )
}

export function ClipPage() {
  const { t } = useTranslation('clips')
  const { user } = useSession()
  const { clipId } = useParams({ strict: false })
  const navigate = useNavigate()
  const search = useSearch({ strict: false })
  const channel = clipChannel(search.channel)
  const ownerId = user?.id ?? ''
  const query = useClipProject(ownerId, clipId)
  const destination = clipReturnDestination(
    ownerId,
    clipId,
    undefined,
    query.data?.channel ?? channel,
  )
  const returnTo = {
    href: destination.href,
    label:
      destination.path === '/my'
        ? t('navigation.returnMine')
        : t('navigation.returnCreation', { channel: t(`channel.${destination.channel}`) }),
    onReturn: async () => {
      if (clipId) markClipHistoryReturn(ownerId, clipId)
      await navigate({ href: destination.href })
    },
  }
  // No leave dialog: ② autosaves and sends a waiting edit as it is left (CLIP-39).
  return (
    // `flex-1 flex-col` here plus `mt-auto` on a dock is what puts the bar at the BOTTOM of a
    // short panel: `sticky` can only pull an element up toward the scrollport edge, never push
    // one down.
    // Only the preparation guide's step dock meets a phone's bottom edge; every other view keeps
    // the page padding under its content or its own dock.
    <main
      className={pageStyles({
        width: 'workspace',
        className:
          'flex flex-1 flex-col pb-8 max-sm:has-[[data-action-bar=step]:not([hidden]_*)]:pb-0',
      })}
    >
      {!clipId ? (
        <NewClip
          key={`${ownerId}-${channel}-${search.template ?? ''}`}
          ownerId={ownerId}
          channel={channel}
          handedTemplate={search.template}
          fromSetup={search.from === 'setup'}
          returnTo={returnTo}
        />
      ) : query.data ? (
        <>
          <ClipWorkspace
            key={`${user?.id}-${clipId}`}
            ownerId={user?.id ?? ''}
            project={query.data}
            // Back from the template wizard: ① opens on 템플릿 (CLIP-225).
            initialGuide={
              (search as { guide?: unknown }).guide === 'template' ? 'template' : undefined
            }
            sourceFeedback={
              <ClipImportFeedback
                key={`import-${ownerId}-${clipId}`}
                ownerId={ownerId}
                clipId={clipId}
              />
            }
          />
        </>
      ) : query.isPending ? (
        <>
          <ClipTopRow status={null} returnTo={returnTo} />
          <Typography variant="body" role="status" className="mt-6">
            {t('project.loading')}
          </Typography>
        </>
      ) : (
        <>
          <ClipTopRow status={null} returnTo={returnTo} />
          <div role="alert" className="mt-6">
            <AppFailureMessage failure={appFailureFromConnect(query.error)} />
            <Button variant="ghost" onClick={() => void query.refetch()}>
              {t('project.retry')}
            </Button>
          </div>
        </>
      )}
    </main>
  )
}
