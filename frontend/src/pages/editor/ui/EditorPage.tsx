import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { usePost, postReturnDestination } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { CreateVoiceSheet } from '@/features/create-voice'
import { CreationOfferSheet, useCreationOffer } from '@/features/offer-creation-setup'
import {
  AppFailureMessage,
  Button,
  WorkspaceHeader,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'
import { DraftEditor } from './DraftEditor'
import { editorNavigationCopy } from '../config/i18n'

/** `/posts/new` — a draft that does not exist yet. The first autosave creates it and
 *  moves the URL to the minted slug (see DraftEditor's `onMinted`).
 *
 *  The directory is read first: the picker is seeded with the account's 기본, or 말투 없음 when
 *  there is none (POST-101), so the editor waits for that one answer rather than letting a first
 *  keystroke create the post on 말투 없음 because the 기본 had not arrived yet.
 *
 *  It also waits for the template directory: an account with no made voice or no template opens
 *  on the offer to make it (POST-121), and that sheet has to stand before anything is typed. */
export function NewDraftPage() {
  const { t } = useTranslation(['posts', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const navigate = useNavigate()
  const { defaultVoice, isPending, isError, refetch } = useVoices(ownerId)
  const { template, from } = useSearch({ strict: false }) as {
    template?: string
    from?: 'setup'
  }
  // A handed template is the offer's own wizard coming back; `from=setup` is setup's finish.
  // Either way the owner has just answered it.
  const offer = useCreationOffer({
    ownerId,
    kind: 'post',
    suppressed: !!template || from === 'setup',
  })
  const [creatingVoice, setCreatingVoice] = useState(false)

  if (isError)
    return <LoadFailure message={t('editor.voiceListFailed', { ns: 'posts' })} onRetry={refetch} />
  if (isPending || !offer.settled)
    return <EditorPlaceholder>{t('state.loading', { ns: 'common' })}</EditorPlaceholder>
  return (
    <>
      <DraftEditor defaultVoice={defaultVoice} handedTemplate={template} />
      <CreationOfferSheet
        kind="post"
        open={offer.open}
        missing={offer.missing}
        onCreateVoice={() => {
          offer.close()
          setCreatingVoice(true)
        }}
        onContinueVoice={(voiceId) => {
          offer.close()
          void navigate({ to: '/voices/$voiceId', params: { voiceId } })
        }}
        onCreateTemplate={() => {
          offer.close()
          // As ①'s picker opens it: 저장 returns to a new draft with the template (TMPL-32).
          void navigate({ to: '/templates/new', search: { post: 'new' } })
        }}
        onClose={offer.close}
        onDismiss={offer.dismiss}
      />
      <CreateVoiceSheet ownerId={ownerId} open={creatingVoice} onOpenChange={setCreatingVoice} />
    </>
  )
}

/** `/posts/$slug` — an existing post. */
export function PostEditorPage() {
  const { t } = useTranslation(['posts', 'common'])
  // The page owns its public parameter, not the shell's pathless layout ids.
  const { slug = '' } = useParams({ strict: false })
  const { post, isPending, failure, refetch } = usePost(slug)
  const { template } = useSearch({ strict: false }) as { template?: string }

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
  // The voice warning is the editor's own — it belongs below the memo, inside the page's one
  // `<main>`, rather than above it as a sibling (see EditorGeneratePanel).
  return <DraftEditor key={slug} post={post} handedTemplate={template} />
}

function LoadFailure({ message, onRetry }: { message: ReactNode; onRetry?: () => void }) {
  const { t } = useTranslation(['posts', 'common'])
  return (
    <EditorPlaceholder>
      <span role="alert" className="text-notice-danger-fg">
        {message}
      </span>
      <span className="mt-4 flex flex-wrap items-center gap-2">
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
  const { i18n } = useTranslation('posts')
  const { user } = useSession()
  const { slug } = useParams({ strict: false })
  const navigate = useNavigate()
  const returnTo = postReturnDestination(user?.id ?? '', slug)
  const copy = editorNavigationCopy[i18n.resolvedLanguage?.startsWith('ko') ? 'ko' : 'en']
  return (
    <main
      className={typographyStyles({
        variant: 'body',
        className: pageStyles({
          width: 'prose',
          className: 'text-content-tertiary flex flex-1 flex-col items-start gap-4 pt-3 sm:pt-6',
        }),
      })}
    >
      <WorkspaceHeader
        returnTo={{
          href: returnTo.href,
          label: returnTo.path === '/my' ? copy.mine : copy.home,
          onReturn: async () => {
            await navigate({ href: returnTo.href })
            window.requestAnimationFrame(() => window.scrollTo(0, returnTo.scrollY))
          },
        }}
      />
      {children}
    </main>
  )
}
