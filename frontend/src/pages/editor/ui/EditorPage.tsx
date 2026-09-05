import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from '@tanstack/react-router'
import { usePost } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { AppFailureMessage, Button, typographyStyles, pageStyles } from '@/shared/ui'
import { DraftEditor } from './DraftEditor'

/** `/posts/new` — a draft that does not exist yet. The first autosave creates it and
 *  moves the URL to the minted slug (see DraftEditor's `onMinted`).
 *
 *  The directory is read first: a create must name its voice, and the picker is initialized to
 *  the account's default (spec/legacy/policy/posts.md), so the editor waits for that one answer rather
 *  than letting a first keystroke send a save with no voice for the server to refuse. */
export function NewDraftPage() {
  const { t } = useTranslation(['posts', 'common'])
  const { user } = useSession()
  const { defaultVoice, isPending, isError, refetch } = useVoices(user?.id ?? '')

  if (isError)
    return <LoadFailure message={t('editor.voiceListFailed', { ns: 'posts' })} onRetry={refetch} />
  if (isPending)
    return <EditorPlaceholder>{t('state.loading', { ns: 'common' })}</EditorPlaceholder>
  if (!defaultVoice) {
    return (
      <LoadFailure
        message={t('editor.noDefaultVoice', { ns: 'posts' })}
        action={
          <Link
            to="/voices"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
          >
            {t('editor.goVoices', { ns: 'posts' })}
          </Link>
        }
      />
    )
  }
  return <DraftEditor defaultVoiceId={defaultVoice.id} />
}

/** `/posts/$slug` — an existing post. */
export function PostEditorPage() {
  const { t } = useTranslation(['posts', 'common'])
  // The '/authenticated' prefix is the id of the pathless guard layout every signed-in
  // route hangs off (app/routes/router.tsx) — the URL itself is still '/posts/<slug>'.
  const { slug } = useParams({ from: '/authenticated/posts/$slug' })
  const { post, isPending, failure, refetch } = usePost(slug)

  if (failure) {
    const retryable = failure.reason !== 'POST_NOT_FOUND' && failure.reason !== 'POST_FORBIDDEN'
    const message =
      failure.reason === 'POST_NOT_FOUND' ? (
        t('editor.notFound', { ns: 'posts' })
      ) : failure.reason === 'POST_FORBIDDEN' ? (
        t('editor.forbidden', { ns: 'posts' })
      ) : (
        <AppFailureMessage failure={failure} />
      )
    return (
      <LoadFailure
        message={message}
        // Only for a failure that is not an answer: retrying a 403 or a 404 would just
        // ask the same question again.
        onRetry={retryable ? refetch : undefined}
      />
    )
  }
  if (isPending)
    return <EditorPlaceholder>{t('state.loading', { ns: 'common' })}</EditorPlaceholder>
  // A 200 carrying no post is not an answer the editor can work with; treating it as an
  // empty draft would autosave over whatever the post actually holds.
  if (!post) return <LoadFailure message={t('editor.notFound', { ns: 'posts' })} />

  // Keyed by slug: this route stays mounted when the param changes, so opening another
  // post from the list has to start a new editor rather than keep the previous text.
  // The empty-profile warning is the editor's own now — it belongs below the memo, inside the
  // page's one `<main>`, rather than above it as a sibling (see DraftEditor).
  return <DraftEditor key={slug} post={post} />
}

function LoadFailure({
  message,
  onRetry,
  action,
}: {
  message: ReactNode
  onRetry?: () => void
  action?: ReactNode
}) {
  const { t } = useTranslation(['posts', 'common'])
  return (
    <EditorPlaceholder>
      <span role="alert" className="text-notice-danger-fg">
        {message}
      </span>
      <span className="mt-4 flex flex-wrap items-center gap-2">
        <Link
          to="/posts"
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
        >
          {t('editor.backToListPlain', { ns: 'posts' })}
        </Link>
        {action}
        {onRetry && (
          <Button variant="ghost" onClick={onRetry} className="underline">
            {t('action.retry', { ns: 'common' })}
          </Button>
        )}
      </span>
    </EditorPlaceholder>
  )
}

function EditorPlaceholder({ children }: { children: ReactNode }) {
  return (
    <main
      className={typographyStyles({
        variant: 'body',
        className: pageStyles({
          className: 'text-content-tertiary flex flex-col items-start py-16',
        }),
      })}
    >
      {children}
    </main>
  )
}
