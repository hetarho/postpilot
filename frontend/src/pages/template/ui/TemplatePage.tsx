import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { usePost } from '@/entities/post'
import {
  AUTHORING_REFERENCE_POST_MAX_CHARS,
  useAuthoringSummaries,
  type AuthoringSavedRef,
} from '@/entities/ai-authoring'
import { BlockType } from '@/shared/api'
import { useTemplates, type Template } from '@/entities/template'
import { TemplateDirectEditor } from '@/features/edit-template'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { Button, LoadingState, Notice, Typography, PageFrame } from '@/shared/ui'

export function TemplatePage() {
  const { templateId } = useParams({ strict: false }) as { templateId?: string }
  const { from, session, post } = useSearch({ strict: false }) as {
    from?: string
    session?: string
    post?: string
  }
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const stored = templateId ? templates.find((template) => template.id === templateId) : undefined
  if (isError)
    return (
      <PageFrame width="wide">
        <Notice tone="danger" role="alert" className="mt-4">
          {t('loadFailed', { ns: 'templates' })}
          <Button variant="ghost" onClick={refetch} pending={isFetching}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      </PageFrame>
    )
  if (templateId && (isPending || (!stored && isFetching)))
    return (
      <PageFrame width="wide">
        <LoadingState
          variant="detail"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8"
        />
      </PageFrame>
    )
  if (templateId && !stored && !isFetching)
    return (
      <PageFrame width="wide">
        <Notice tone="danger" role="alert" className="mt-4">
          {t('screen.notFound', { ns: 'templates' })}
        </Notice>
      </PageFrame>
    )
  return (
    <TemplateReference slug={stored ? undefined : from}>
      {(reference) => (
        <TemplateAuthoringPage
          key={JSON.stringify([
            ownerId,
            stored?.id ?? 'new',
            stored ? '' : session,
            stored ? '' : from,
          ])}
          ownerId={ownerId}
          stored={stored}
          reference={reference}
          sessionId={stored ? undefined : session}
          returnPost={stored ? undefined : post}
          onPublished={refetch}
        />
      )}
    </TemplateReference>
  )
}

/** `?from=<slug>` (이 글 형식으로 템플릿 만들기): the post the owner asked the new template to follow,
 *  handed to the conversation as its one bounded reference text (EDIT-16) — its headings marked,
 *  its paragraphs as they are. A post that cannot be read leaves the conversation without one. */
function TemplateReference({
  slug,
  children,
}: {
  slug?: string
  children: (reference: string | undefined) => ReactNode
}) {
  const { t } = useTranslation('common')
  const sample = usePost(slug ?? '', { enabled: !!slug })
  if (!slug) return children(undefined)
  if (sample.isPending)
    return (
      <PageFrame width="wide">
        <LoadingState variant="detail" label={t('state.loading')} className="mt-8" />
      </PageFrame>
    )
  const reference = sample.post?.content?.blocks
    .map((block) => (block.type === BlockType.HEADING ? `## ${block.content}` : block.content))
    .filter((text) => text.trim() !== '')
    .join('\n\n')
  return children(
    reference
      ? Array.from(reference).slice(0, AUTHORING_REFERENCE_POST_MAX_CHARS).join('')
      : undefined,
  )
}

/** Guide: a template is made by talking (owner 2026-11-17). New or saved, the screen opens on one
 *  conversation beside the draft's live preview: the owner says what they post, the AI drafts the
 *  template from scratch, each message refines it, and 저장 names and keeps it. Editing the text by
 *  hand is one ⋯ away. There is no wizard and nothing to drag. */
function TemplateAuthoringPage({
  ownerId,
  stored,
  reference,
  sessionId,
  returnPost,
  onPublished,
}: {
  ownerId: string
  stored?: Template
  /** The post the template should follow, as text (`?from=`). */
  reference?: string
  sessionId?: string
  /** The post whose 템플릿 picker opened this screen, or `new` for an unsaved draft (TMPL-32). */
  returnPost?: string
  onPublished: () => void
}) {
  const { t } = useTranslation(['templates', 'authoring', 'nav', 'common'])
  const navigate = useNavigate()
  const [published, setPublished] = useState<AuthoringSavedRef>()
  // A saved template with work not yet saved reopens that conversation rather than a fresh one
  // from the saved copy, so nothing the owner changed is hidden (EDIT-15, EDIT-18).
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const unfinished = stored
    ? summaries.data?.find(
        (row) =>
          row.targetId === stored.id &&
          (row.hasUnpublishedChanges ||
            !!row.activeJobId ||
            row.publicationPending ||
            row.targetConflict),
      )
    : undefined
  const resume = sessionId ?? unfinished?.sessionId
  if (stored && summaries.isPending)
    return (
      <PageFrame width="wide">
        <LoadingState
          variant="detail"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8"
        />
      </PageFrame>
    )
  return (
    <PageFrame
      width="wide"
      className="flex flex-1 flex-col py-4 sm:py-8"
      title={
        <Typography variant="label" as="h1" className="text-content-secondary truncate">
          {t(stored ? 'location.template' : 'location.newTemplate', { ns: 'nav' })}
        </Typography>
      }
    >
      {published?.outcome && (
        <Typography variant="body" role="status" className="mb-4">
          {t(published.outcome === 'updated' ? 'confirmedUpdate' : 'confirmedCreate', {
            ns: 'authoring',
            kind: t('kinds.post-template', { ns: 'authoring' }),
            name: published.name,
          })}
        </Typography>
      )}
      <AIAuthoringStudio
        ownerId={ownerId}
        kind="post-template"
        layout="conversation"
        candidateCount={1}
        targetId={stored?.id}
        targetName={stored?.name}
        sessionId={resume}
        initialMethod={stored ? 'ai' : undefined}
        startFromSaved={!resume}
        referencePost={reference}
        renderDirectEditor={(props) => <TemplateDirectEditor {...props} />}
        onSaved={(saved) => {
          setPublished(saved)
          onPublished()
          // Opened from a post's picker: back to that post with the template assigned.
          if (returnPost === 'new')
            void navigate({ to: '/posts/new', search: { template: saved.id }, replace: true })
          else if (returnPost)
            void navigate({
              to: '/posts/$slug',
              params: { slug: returnPost },
              search: { template: saved.id },
              replace: true,
            })
          else if (!stored)
            void navigate({
              to: '/templates/$templateId',
              params: { templateId: saved.id },
              replace: true,
            })
        }}
        className="flex-1"
      />
    </PageFrame>
  )
}
