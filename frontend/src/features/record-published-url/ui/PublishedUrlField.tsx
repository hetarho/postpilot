import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  isPublished,
  isUnfinalized,
  parseNaverBlogUrl,
  useSavePublishedUrl,
  type PostDraft,
} from '@/entities/post'
import { formatAppFailure } from '@/shared/lib'
import { Button, FieldLabel, FieldMessage, TextField, Toggletip, Typography } from '@/shared/ui'

/** ③'s foot (POST-54): the post's Naver Blog address, pasted to publish it, replaced by pasting
 *  another, cleared to reopen it (POST-73, POST-75). It opens once the post is 확정 (POST-87).
 *
 *  An address that is not a Naver Blog post is refused here, before any request, in the same
 *  sentence the server would answer with: the pre-check and the server read one rule (POST-77). */
export function PublishedUrlField({ post, className }: { post: PostDraft; className?: string }) {
  const { t } = useTranslation('posts')
  const headingId = useId()
  return (
    <section aria-labelledby={headingId} className={className}>
      <Typography variant="title" id={headingId}>
        {t('publishedUrl.title')}
      </Typography>
      {/* Keyed on the stored address, so every answer reseeds the field: empty on a finalized
          post, the stored address on a published one. */}
      <PublishedUrlForm key={post.publishedUrl} post={post} />
    </section>
  )
}

function PublishedUrlForm({ post }: { post: PostDraft }) {
  const { t } = useTranslation(['posts', 'common'])
  const id = useId()
  const helpId = `${id}-help`
  const reasonId = `${id}-reason`
  const errorId = `${id}-error`
  const saving = useSavePublishedUrl()
  const [draft, setDraft] = useState(post.publishedUrl)
  const [localError, setLocalError] = useState('')
  const published = isPublished(post)
  // A busy post is the server's to refuse (POST_BUSY); only the lifecycle closes the field here.
  const closed = isUnfinalized(post)
  const error = localError || (saving.failure ? formatAppFailure(saving.failure) : '')

  const send = async (url: string) => {
    try {
      await saving.save(post.slug, url)
    } catch {
      // The refusal renders under the field, and the post stays as the server holds it.
    }
  }

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (closed || saving.pending) return
    const trimmed = draft.trim()
    if (trimmed === '') {
      // An empty 저장 clears a published post's address and means nothing on any other.
      if (published) void send('')
      return
    }
    if (parseNaverBlogUrl(trimmed) === undefined) {
      setLocalError(formatAppFailure({ reason: 'POST_PUBLISHED_URL_INVALID', params: {} }))
      return
    }
    void send(trimmed)
  }

  const describedBy = [helpId, closed ? reasonId : '', error ? errorId : '']
    .filter(Boolean)
    .join(' ')

  return (
    // `noValidate`: the browser's own url check would answer in its own words before ours.
    <form onSubmit={submit} noValidate className="mt-3">
      {closed && (
        <Typography variant="label" as="p" role="status" id={reasonId} className="mb-3">
          {t('publishedUrl.notFinalized', { ns: 'posts' })}
        </Typography>
      )}
      {/* What the address does is behind the ⓘ rather than a standing line under the field
          (owner decision 2026-09-25); it stays the field's description for a screen reader. */}
      <div className="flex items-center gap-1">
        <FieldLabel htmlFor={`${id}-url`}>{t('publishedUrl.label', { ns: 'posts' })}</FieldLabel>
        <Toggletip label={t('publishedUrl.explain', { ns: 'posts' })} className="-my-2">
          {t('publishedUrl.help', { ns: 'posts' })}
        </Toggletip>
      </div>
      <TextField
        id={`${id}-url`}
        type="url"
        inputMode="url"
        autoComplete="url"
        autoCapitalize="none"
        autoCorrect="off"
        enterKeyHint="done"
        spellCheck={false}
        value={draft}
        disabled={closed}
        placeholder={t('publishedUrl.placeholder', { ns: 'posts' })}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        onChange={(event) => {
          setDraft(event.target.value)
          setLocalError('')
          saving.reset()
        }}
        className="mt-1"
      />
      <span id={helpId} className="sr-only">
        {t('publishedUrl.help', { ns: 'posts' })}
      </span>
      {error && (
        <FieldMessage id={errorId} role="alert" className="mt-2">
          {error}
        </FieldMessage>
      )}
      <div className="mt-3 flex flex-wrap gap-2">
        <Button
          type="submit"
          variant="secondary"
          disabled={closed}
          pending={saving.pending}
          className="w-full sm:w-auto"
        >
          {t('action.save', { ns: 'common' })}
        </Button>
        {published && (
          // No confirmation: pasting the address again undoes it.
          <Button
            type="button"
            variant="ghost"
            disabled={saving.pending}
            onClick={() => void send('')}
          >
            {t('publishedUrl.clear', { ns: 'posts' })}
          </Button>
        )}
      </div>
    </form>
  )
}
