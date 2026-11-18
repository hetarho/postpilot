import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronRight } from 'lucide-react'
import { PostPicker } from '@/entities/post'
import { serialClipWrite, useLinkClipPost, type ClipProject } from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Sheet, Typography } from '@/shared/ui'
import { flushClipDraft } from '../model/clip-draft-queue'

export function ClipLinkedPost({
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
  const [open, setOpen] = useState(false)
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  const titleId = useId()
  const held = useRef(false)
  const authority = useRef(0)
  useEffect(() => {
    authority.current += 1
    return () => {
      authority.current += 1
    }
  }, [ownerId, project.id])
  const mutation = useLinkClipPost(ownerId)
  const label = project.linkedPost?.title || t('linkedPost.none')
  const link = async (slug: string) => {
    if (disabled || readOnly || held.current) return
    held.current = true
    setPending(true)
    setFailure(undefined)
    const operation = authority.current
    try {
      await flushClipDraft(project.id, true)
      if (operation !== authority.current) return
      await serialClipWrite(project.id, () => {
        if (operation !== authority.current) throw new DOMException('Owner changed', 'AbortError')
        return mutation.mutateAsync({ id: project.id, slug })
      })
      if (operation === authority.current) setOpen(false)
    } catch (error) {
      if (operation === authority.current) setFailure(appFailureFromConnect(error))
    } finally {
      held.current = false
      if (operation === authority.current) setPending(false)
    }
  }
  return (
    <div>
      <Typography variant="fieldTitle" as="p">
        {t('linkedPost.title')}
      </Typography>
      {readOnly ? (
        <Typography variant="body" className="mt-2">
          {label}
        </Typography>
      ) : (
        <>
          <Button
            variant="ghost"
            className="mt-2 flex w-full justify-between gap-3 text-left"
            disabled={disabled || pending}
            aria-label={`${t('linkedPost.title')}: ${label}`}
            onClick={() => {
              setFailure(undefined)
              setOpen(true)
            }}
          >
            <span className="line-clamp-2 min-w-0 break-words">{label}</span>
            <ChevronRight aria-hidden="true" className="size-4 shrink-0" />
          </Button>
          <Sheet
            open={open}
            labelledBy={titleId}
            onClose={() => setOpen(false)}
            header={
              <div className="flex items-center justify-between gap-3">
                <Typography variant="title" as="h2" id={titleId}>
                  {t('linkedPost.title')}
                </Typography>
                <Button variant="ghost" onClick={() => setOpen(false)}>
                  {t('linkedPost.close')}
                </Button>
              </div>
            }
          >
            {open && (
              <div className="mt-4">
                <PostPicker
                  initialQuery={project.title}
                  pending={pending || disabled}
                  onPick={(post) => {
                    void link(post.slug)
                  }}
                />
              </div>
            )}
            {project.linkedPost && (
              <Button
                variant="ghost"
                pending={pending}
                disabled={disabled}
                onClick={() => {
                  void link('')
                }}
              >
                {t('linkedPost.unlink')}
              </Button>
            )}
            {failure && (
              <div role="alert">
                <AppFailureMessage failure={failure} />
              </div>
            )}
          </Sheet>
        </>
      )}
    </div>
  )
}
