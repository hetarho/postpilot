import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipChannel,
  readClipReturnContext,
  rememberClipEntry,
  serialClipWrite,
  useMoveClipChannel,
  type ClipProject,
} from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import { flushClipDraft } from '../model/clip-draft-queue'

export function MoveClipChannel({
  ownerId,
  project,
  disabled,
  readOnly = false,
}: {
  ownerId: string
  project: ClipProject
  disabled: boolean
  readOnly?: boolean
}) {
  const { t } = useTranslation('clips')
  const mutation = useMoveClipChannel(ownerId)
  const held = useRef(false)
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  const current = clipChannel(project.channel)
  const next = current === 'naver_clip' ? 'instagram_clip' : 'naver_clip'
  const move = async () => {
    if (disabled || held.current) return
    held.current = true
    setPending(true)
    setFailure(undefined)
    try {
      await flushClipDraft(project.id, true)
      const moved = await serialClipWrite(project.id, () =>
        mutation.mutateAsync({ id: project.id, channel: next }),
      )
      const entry = readClipReturnContext(ownerId, project.id)
      if (entry?.path === '/clips')
        rememberClipEntry(ownerId, {
          ...entry,
          path: '/clips',
          filters: { ...entry.filters, channel: clipChannel(moved.channel) },
        })
    } catch (error) {
      setFailure(appFailureFromConnect(error))
    } finally {
      held.current = false
      setPending(false)
    }
  }
  return (
    <div>
      <Typography variant="fieldTitle" as="p">
        {t('project.channel')}
      </Typography>
      <Typography variant="body" className="mt-2">
        {t(`channel.${current}`)}
      </Typography>
      {!readOnly && (
        <Button
          variant="ghost"
          pending={pending}
          disabled={disabled}
          onClick={(event) => {
            if (event.detail <= 1) void move()
          }}
        >
          {t(`project.move.${next}`)}
        </Button>
      )}
      {failure && (
        <div role="alert">
          <AppFailureMessage failure={failure} />
        </div>
      )}
    </div>
  )
}
