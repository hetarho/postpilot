import { useNavigate, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  clipReturnDestination,
  markClipHistoryReturn,
  useClipProject,
} from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { ClipProjectForm } from '@/features/edit-clip-project'
import { ClipTopRow, ClipStatusLine, ClipWorkspace } from '@/widgets/clip-workspace'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Typography, pageStyles } from '@/shared/ui'

/** `/clips/new` — a project that does not exist yet. It has no lifecycle, so it shows no step
 *  bar and no delete: just the settings and the one committing action that mints it, which stays
 *  explicit because the ratio it carries can never be changed again (CLIP-9, CLIP-39). */
function NewClip({
  ownerId,
  returnTo,
}: {
  ownerId: string
  returnTo: { href: string; label: string; onReturn: () => Promise<void> }
}) {
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
      <ClipProjectForm ownerId={ownerId} />
    </>
  )
}

export function ClipPage() {
  const { t } = useTranslation('clips')
  const { user } = useSession()
  const { clipId } = useParams({ strict: false })
  const navigate = useNavigate()
  const ownerId = user?.id ?? ''
  const destination = clipReturnDestination(ownerId, clipId)
  const returnTo = {
    href: destination.href,
    label: t(destination.path === '/' ? 'navigation.returnCreation' : 'navigation.returnHistory'),
    onReturn: async () => {
      if (clipId) markClipHistoryReturn(ownerId, clipId)
      await navigate({ href: destination.href })
    },
  }
  // No leave dialog: ② autosaves and sends a waiting edit as it is left (CLIP-39).
  const query = useClipProject(user?.id ?? '', clipId)
  return (
    // `flex-1 flex-col` here plus `mt-auto` on a dock is what puts the bar at the BOTTOM of a
    // short panel: `sticky` can only pull an element up toward the scrollport edge, never push
    // one down.
    <main className={pageStyles({ width: 'workspace', className: 'flex flex-1 flex-col pb-8' })}>
      {!clipId ? (
        <NewClip key={ownerId} ownerId={ownerId} returnTo={returnTo} />
      ) : query.data ? (
        <ClipWorkspace
          key={`${user?.id}-${clipId}`}
          ownerId={user?.id ?? ''}
          project={query.data}
        />
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
