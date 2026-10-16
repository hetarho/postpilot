import { useEffect, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import type { PostDraft } from '@/entities/post'
import {
  CLIP_CHANNELS,
  CLIP_PROJECT_LIMITS,
  DEFAULT_CLIP_CHANNEL,
  freeTextLength,
  rememberClipEntry,
  clipChannelPath,
  useClipProjectMutations,
  useClipSourceCalls,
  type ClipChannel,
} from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  Dialog,
  Notice,
  SegmentedControl,
  Typography,
} from '@/shared/ui'
import { clipFromPostCopy } from '../config/i18n'
import { createClipFromPost, type CreatedPostClip } from '../model/createClipFromPost'

export function CreateClipFromPostButton({
  ownerId,
  post,
  beforeCreate,
}: {
  ownerId: string
  post: PostDraft
  beforeCreate?: () => Promise<void>
}) {
  const { t, i18n } = useTranslation('clips')
  const copy = clipFromPostCopy[i18n.resolvedLanguage?.startsWith('ko') ? 'ko' : 'en']
  const navigate = useNavigate()
  const { save } = useClipProjectMutations(ownerId)
  const sources = useClipSourceCalls()
  const [open, setOpen] = useState(false)
  const [channel, setChannel] = useState<ClipChannel>(DEFAULT_CLIP_CHANNEL)
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  const [created, setCreated] = useState<CreatedPostClip>()
  const held = useRef(false)
  const operation = useRef<AbortController | undefined>(undefined)
  useEffect(() => () => operation.current?.abort(), [ownerId, post.slug])

  const confirm = async () => {
    if (held.current) return
    held.current = true
    const controller = new AbortController()
    operation.current = controller
    setPending(true)
    setFailure(undefined)
    try {
      if (!created) await beforeCreate?.()
      controller.signal.throwIfAborted()
      const result =
        created ??
        (await createClipFromPost({
          post,
          channel,
          create: (draft) => save.mutateAsync({ draft }),
          sources,
          signal: controller.signal,
        }))
      controller.signal.throwIfAborted()
      setCreated(result)
      const actualChannel = result.project.channel ?? channel
      rememberClipEntry(ownerId, {
        path: clipChannelPath(actualChannel),
        section: actualChannel,
        filters: { channel: actualChannel },
        scrollY: 0,
        targetId: result.project.id,
      })
      await navigate({
        to: '/clips/$clipId',
        params: { clipId: result.project.id },
        state: {
          clipImport: {
            ownerId,
            projectId: result.project.id,
            problems: result.sources.problems,
            uncertain: result.sources.uncertain,
          },
        },
      })
    } catch (error) {
      if (!controller.signal.aborted) setFailure(appFailureFromConnect(error))
    } finally {
      held.current = false
      if (!controller.signal.aborted) setPending(false)
    }
  }

  return (
    <>
      <Button
        variant="secondary"
        disabled={pending}
        onClick={() => {
          setFailure(undefined)
          setOpen(true)
        }}
      >
        {copy.action}
      </Button>
      <Dialog
        open={open}
        title={copy.action}
        confirmLabel={created ? copy.open : pending ? copy.importing : copy.confirm}
        pending={pending}
        onConfirm={() => void confirm()}
        onClose={() => {
          if (!held.current) setOpen(false)
        }}
      >
        <div className="space-y-4">
          <Typography variant="body" as="p">
            {copy.description}
          </Typography>
          <SegmentedControl
            value={channel}
            options={CLIP_CHANNELS.map((value) => ({ value, label: t(`channel.${value}`) }))}
            onChange={setChannel}
            ariaLabel={copy.channel}
            disabled={pending || Boolean(created)}
          />
          {freeTextLength(post.memo) > CLIP_PROJECT_LIMITS.instruction && (
            <Notice tone="info">{copy.memoCut(CLIP_PROJECT_LIMITS.instruction)}</Notice>
          )}
          {failure && (
            <div role="alert">
              <AppFailureMessage failure={failure} />
            </div>
          )}
        </div>
      </Dialog>
    </>
  )
}
