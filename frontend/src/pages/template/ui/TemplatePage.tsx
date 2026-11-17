import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries } from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { DeleteTemplateButton } from '@/features/delete-template'
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
    <TemplateConversationPage
      key={JSON.stringify([
        ownerId,
        stored?.id ?? 'new',
        stored ? '' : session,
        stored ? '' : from,
      ])}
      ownerId={ownerId}
      stored={stored}
      referencePost={stored ? undefined : from}
      sessionId={stored ? undefined : session}
      returnPost={stored ? undefined : post}
      onPublished={refetch}
    />
  )
}

/** Guide: a template is made by talking (TMPL departure, see the showcase notes). A new one
 *  opens straight on the conversation — the owner says what they post and the AI drafts the whole
 *  template from scratch; an existing one opens on itself, with the same conversation beside it to
 *  change it in words. Typing the template text directly is one ⋯ choice away; nothing is dragged.
 *
 *  A template opened from a post's 템플릿 picker (`?post=`) goes back to that post with the new
 *  template assigned (TMPL-32); one started from a post (`?from=`) is drafted with that post as
 *  its reference. */
function TemplateConversationPage({
  ownerId,
  stored,
  referencePost,
  sessionId,
  returnPost,
  onPublished,
}: {
  ownerId: string
  stored?: Template
  referencePost?: string
  sessionId?: string
  returnPost?: string
  onPublished: () => void
}) {
  const { t } = useTranslation(['templates', 'authoring', 'common'])
  const navigate = useNavigate()
  // The conversation's status and its ⋯ and 저장 sit in this screen's one title row.
  const [statusSlot, setStatusSlot] = useState<HTMLDivElement | null>(null)
  const [controlsSlot, setControlsSlot] = useState<HTMLDivElement | null>(null)
  const [deleting, setDeleting] = useState(false)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  // An existing template with unsaved conversation work reopens that work.
  const resumable = stored
    ? summaries.data?.find(
        (row) =>
          row.targetId === stored.id &&
          (row.hasUnpublishedChanges || row.activeJobId || row.publicationPending),
      )
    : undefined
  const session = sessionId ?? resumable?.sessionId
  if (stored && summaries.isPending)
    return (
      <PageFrame width="wide">
        <LoadingState variant="detail" label={t('state.loading', { ns: 'common' })} />
      </PageFrame>
    )
  return (
    <PageFrame
      width="wide"
      className="flex flex-1 flex-col pb-0 sm:pb-0"
      title={
        <div className="min-w-0">
          <Typography variant="title" as="h1" className="truncate">
            {stored?.name ?? t('page.new', { ns: 'templates' })}
          </Typography>
          <div ref={setStatusSlot} />
        </div>
      }
      actions={<div ref={setControlsSlot} className="flex items-center gap-2" />}
    >
      {stored && (
        <DeleteTemplateButton
          ownerId={ownerId}
          template={stored}
          menu={{ open: deleting, onOpenChange: setDeleting }}
          onDeleted={() => void navigate({ to: '/templates', replace: true })}
        />
      )}
      <AIAuthoringStudio
        layout="conversation"
        ownerId={ownerId}
        kind="post-template"
        targetId={stored?.id}
        targetName={stored?.name}
        sessionId={session}
        initialMethod={session ? 'ai' : undefined}
        startFromSaved={!session}
        referencePost={referencePost}
        savedPreview={stored && <TemplatePreview titleArea={stored.titleArea} body={stored.body} />}
        renderDirectEditor={(props) => <TemplateDirectEditor {...props} />}
        heading={{ status: statusSlot, controls: controlsSlot }}
        menuItems={
          stored
            ? [
                {
                  id: 'delete',
                  label: t('screen.deleteMenu', { ns: 'templates' }),
                  onSelect: () => setDeleting(true),
                },
              ]
            : undefined
        }
        onSaved={(saved) => {
          onPublished()
          if (stored) return
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
          else
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
