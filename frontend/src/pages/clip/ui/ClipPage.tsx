import { useRef, useState } from 'react'
import { useBlocker, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipProject } from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { ClipProjectForm } from '@/features/edit-clip-project'
import { ClipTopRow, ClipStatusLine, ClipWorkspace } from '@/widgets/clip-workspace'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, Typography, pageStyles } from '@/shared/ui'

/** `/clips/new` — a project that does not exist yet. It has no lifecycle, so it shows no step
 *  bar and no delete: just the settings and the one committing action that mints it, which stays
 *  explicit because the ratio it carries can never be changed again (CLIP-9, CLIP-39). */
function NewClip({ ownerId }: { ownerId: string }) {
  return (
    <>
      {/* The status region is mounted before there is a project to have a status: a live region
          inserted already holding its message announces nothing. */}
      <ClipTopRow
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
  const query = useClipProject(user?.id ?? '', clipId)
  // The unsaved-correction guard belongs to the PAGE, because leaving is a ROUTE change: the
  // workspace says whether it still holds work (its draft outlives a step change), and the page
  // is what may refuse the navigation.
  const [unsaved, setUnsaved] = useState(false)
  const leaving = useRef(false)
  const guard = () => unsaved && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })
  return (
    // `flex-1 flex-col` here plus `mt-auto` on a dock is what puts the bar at the BOTTOM of a
    // short panel: `sticky` can only pull an element up toward the scrollport edge, never push
    // one down.
    <main className={pageStyles({ width: 'prose', className: 'flex flex-1 flex-col pb-8' })}>
      {!clipId ? (
        <NewClip ownerId={user?.id ?? ''} />
      ) : query.data ? (
        <ClipWorkspace
          key={`${user?.id}-${clipId}`}
          ownerId={user?.id ?? ''}
          project={query.data}
          onUnsavedChange={setUnsaved}
        />
      ) : query.isPending ? (
        <>
          <ClipTopRow status={null} />
          <Typography variant="body" role="status" className="mt-6">
            {t('project.loading')}
          </Typography>
        </>
      ) : (
        <>
          <ClipTopRow status={null} />
          <div role="alert" className="mt-6">
            <AppFailureMessage failure={appFailureFromConnect(query.error)} />
            <Button variant="ghost" onClick={() => void query.refetch()}>
              {t('project.retry')}
            </Button>
          </div>
        </>
      )}
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('correction.leaveTitle')}
        confirmLabel={t('project.leave')}
        onClose={() => blocker.reset?.()}
        onConfirm={() => {
          leaving.current = true
          blocker.proceed?.()
        }}
      >
        {t('correction.leaveBody')}
      </Dialog>
    </main>
  )
}
