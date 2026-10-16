import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronRight } from 'lucide-react'
import { displayTitle, usePosts } from '@/entities/post'
import { serialClipWrite, useLinkClipPost, type ClipProject } from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { POSTS_SEARCH_DEBOUNCE_MS } from '@/shared/config'
import { AppFailureMessage, Button, FieldLabel, Sheet, TextField, Typography } from '@/shared/ui'
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
            <span className="min-w-0 break-words">{label}</span>
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
              <PostPicker
                title={project.title}
                pending={pending || disabled}
                onPick={(slug) => {
                  void link(slug)
                }}
              />
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

function PostPicker({
  title,
  pending,
  onPick,
}: {
  title: string
  pending: boolean
  onPick: (slug: string) => void
}) {
  const { t } = useTranslation('clips')
  const searchId = useId()
  const [search, setSearch] = useState(title)
  const [settled, setSettled] = useState(title)
  useEffect(() => {
    if (search === settled) return
    const timer = setTimeout(() => setSettled(search), POSTS_SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [search, settled])
  const matching = usePosts(settled)
  const all = usePosts()
  const seen = new Set<string>()
  // POST-91 also matches tags; only actual title matches get priority here.
  const normalize = (value: string) =>
    value.normalize('NFKC').toLowerCase().trim().replace(/\s+/gu, ' ')
  const titleMatches = matching.posts.filter((post) =>
    normalize(post.title).includes(normalize(settled)),
  )
  const rows = [...titleMatches, ...all.posts].filter((post) => {
    if (!post.contentReady || seen.has(post.slug)) return false
    seen.add(post.slug)
    return true
  })
  return (
    <div className="mt-4">
      <FieldLabel htmlFor={searchId}>{t('linkedPost.search')}</FieldLabel>
      <TextField
        id={searchId}
        type="search"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
      />
      {matching.isError || all.isError ? (
        <div role="alert" className="mt-3">
          <Typography variant="body">{t('linkedPost.loadFailed')}</Typography>
          <Button
            variant="ghost"
            onClick={() => {
              matching.refetch()
              all.refetch()
            }}
          >
            {t('linkedPost.retry')}
          </Button>
        </div>
      ) : matching.isPending || all.isPending || search !== settled ? (
        <Typography variant="body" role="status" className="mt-3">
          {t('linkedPost.loading')}
        </Typography>
      ) : (
        <ul className="divide-divider mt-3 divide-y">
          {rows.map((post) => (
            <li key={post.slug}>
              <Button
                variant="ghost"
                className="w-full justify-start text-left break-words"
                disabled={pending}
                onClick={() => onPick(post.slug)}
              >
                <span className="min-w-0 break-words">{displayTitle(post)}</span>
              </Button>
            </li>
          ))}
          {rows.length === 0 && (
            <li>
              <Typography variant="body">{t('linkedPost.none')}</Typography>
            </li>
          )}
        </ul>
      )}
    </div>
  )
}
