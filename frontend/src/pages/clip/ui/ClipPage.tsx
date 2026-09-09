import { useState } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipProject, type ClipProject } from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { ClipProjectForm } from '@/features/edit-clip-project'
import { ClipSourcePicker, useClipSourceUpload } from '@/features/upload-clip-sources'
import { appFailureFromConnect } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  ProgressBar,
  Typography,
  buttonStyles,
  pageStyles,
} from '@/shared/ui'

function ExistingClip({ ownerId, project }: { ownerId: string; project: ClipProject }) {
  const { t } = useTranslation('clips')
  const [uploadAllowed, setUploadAllowed] = useState(false)
  const upload = useClipSourceUpload(project.id)
  const busy = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  return (
    <ClipProjectForm
      ownerId={ownerId}
      stored={project}
      onUploadAllowed={setUploadAllowed}
      status={(saved) => (
        <Typography variant="meta">
          {t(saved && upload.phase === 'idle' ? 'project.saved' : `source.phase.${upload.phase}`)}
        </Typography>
      )}
      progress={
        busy && (
          <div className="sm:top-header sticky top-0 z-10 -mx-4 sm:-mx-6 lg:-mx-8">
            <ProgressBar
              label={t('source.phase.uploading')}
              done={
                upload.phase === 'uploading'
                  ? upload.entries.reduce((sum, e) => sum + e.metadata.bytes * e.percent, 0)
                  : undefined
              }
              total={upload.entries.reduce((sum, e) => sum + e.metadata.bytes * 100, 0)}
            />
          </div>
        )
      }
    >
      <ClipSourcePicker upload={upload} disabled={!uploadAllowed} />
    </ClipProjectForm>
  )
}
export function ClipPage() {
  const { t } = useTranslation('clips')
  const { user } = useSession()
  const { clipId } = useParams({ strict: false })
  const query = useClipProject(user?.id ?? '', clipId)
  return (
    <main className={pageStyles({ width: 'prose', className: 'flex flex-1 flex-col pb-8' })}>
      <Link to="/clips" className={buttonStyles({ variant: 'ghost', className: 'self-start' })}>
        {t('project.back')}
      </Link>
      <Typography variant="display" className="mt-6">
        {t(clipId ? 'project.edit' : 'project.new')}
      </Typography>
      {!clipId ? (
        <ClipProjectForm ownerId={user?.id ?? ''} />
      ) : query.data ? (
        <ExistingClip key={`${user?.id}-${clipId}`} ownerId={user?.id ?? ''} project={query.data} />
      ) : query.isPending ? (
        <Typography variant="body" role="status" className="mt-6">
          {t('project.loading')}
        </Typography>
      ) : (
        <div role="alert" className="mt-6">
          <AppFailureMessage failure={appFailureFromConnect(query.error)} />
          <Button variant="ghost" onClick={() => void query.refetch()}>
            {t('project.retry')}
          </Button>
        </div>
      )}
    </main>
  )
}
