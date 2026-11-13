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
import { CreationOfferSheet, useCreationOffer } from '@/features/offer-creation-setup'
import { ClipTopRow, ClipWorkspace, NewClipWorkspace } from '@/widgets/clip-workspace'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, LoadingState, pageStyles } from '@/shared/ui'
import { ClipImportFeedback } from './ClipImportFeedback'

/** `/clips/new` — a project that does not exist yet, running ①'s guide; its first save mints it
 *  (CLIP-130).
 *
 *  An account with no video template opens it on the offer to make one (CLIP-223), so the guide
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
      {offer.settled ? (
        <NewClipWorkspace
          ownerId={ownerId}
          channel={channel}
          initialTemplateId={handedTemplate}
          // Back from the wizard with its template: ① opens on 템플릿 (CLIP-223, CLIP-225).
          initialGuide={handedTemplate ? 'template' : undefined}
          returnTo={returnTo}
        />
      ) : (
        <>
          <ClipTopRow returnTo={returnTo} status={null} />
          <LoadingState variant="form" label={t('state.loading')} className="mt-8" />
        </>
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
          <LoadingState variant="detail" label={t('project.loading')} className="mt-6" />
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
