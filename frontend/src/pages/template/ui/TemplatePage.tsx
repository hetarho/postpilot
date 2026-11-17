import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { MessageCircle, PencilLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSavedRef } from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { TemplateDirectEditor } from '@/features/edit-template'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { Badge, Button, LoadingState, Notice, Typography, PageFrame } from '@/shared/ui'

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
  const key = JSON.stringify([ownerId, stored?.id ?? 'new', stored ? '' : session])
  return stored ? (
    <StoredTemplate key={key} ownerId={ownerId} stored={stored} onPublished={refetch} />
  ) : (
    <NewTemplate
      key={key}
      ownerId={ownerId}
      sampleSlug={from}
      sessionId={session}
      returnPost={post}
      onPublished={refetch}
    />
  )
}

/** Guide: a new template is made by talking, from scratch. The owner says what they post, the AI
 *  drafts the whole template, they refine it by talking with the draft drawn beside them, and 저장
 *  in the heading row keeps it. There is no wizard and nothing to drag: the AI builds the order,
 *  and the owner changes it by saying so (or by hand, as the secondary path). */
function NewTemplate({
  ownerId,
  sampleSlug,
  sessionId,
  returnPost,
  onPublished,
}: {
  ownerId: string
  sampleSlug?: string
  sessionId?: string
  /** The post whose 템플릿 picker opened this, or `new` for an unsaved draft (TMPL-32). */
  returnPost?: string
  onPublished: () => void
}) {
  const { t } = useTranslation('templates')
  const navigate = useNavigate()
  return (
    <PageFrame
      width="board"
      className="flex flex-1 flex-col pt-4 pb-0 sm:pt-8 sm:pb-0"
      title={
        <Typography variant="title" as="h1" className="text-content-secondary truncate">
          {t('screen.talk.newTitle')}
        </Typography>
      }
    >
      <AIAuthoringStudio
        layout="conversation"
        ownerId={ownerId}
        kind="post-template"
        candidateCount={1}
        sessionId={sessionId}
        startFromSaved={!sessionId}
        referencePost={sampleSlug}
        onSaved={(saved) => {
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
          else
            void navigate({
              to: '/templates/$templateId',
              params: { templateId: saved.id },
              replace: true,
            })
        }}
        className="mt-4 flex-1 sm:mt-6"
      />
    </PageFrame>
  )
}

/** Guide: one saved template opens on what it MAKES — the post drawn from it — with the way to
 *  change it in the heading row: talking to the AI first, by hand second, and the earlier talk
 *  when one was left unsaved. */
function StoredTemplate({
  ownerId,
  stored,
  onPublished,
}: {
  ownerId: string
  stored: Template
  onPublished: () => void
}) {
  const { t } = useTranslation(['templates', 'authoring', 'common'])
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const summary = summaries.data?.find(
    (row) => row.targetId === stored.id || row.lastPublication?.id === stored.id,
  )
  const [method, setMethod] = useState<'ai' | 'direct' | null>(null)
  const [activeSession, setActiveSession] = useState<string>()
  const [published, setPublished] = useState<AuthoringSavedRef>()
  const publication =
    published ?? (summary?.lastPublication?.id === stored.id ? summary.lastPublication : undefined)
  const named = { kind: t('kinds.post-template', { ns: 'authoring' }), name: stored.name }
  const unfinished =
    !!summary &&
    (summary.hasUnpublishedChanges ||
      !!summary.activeJobId ||
      summary.publicationPending ||
      summary.targetConflict)
  return (
    <PageFrame
      width="board"
      className="flex flex-1 flex-col pt-4 pb-0 sm:pt-8 sm:pb-0"
      // While it is being changed, the heading row under this one names the template, so the
      // frame says what is happening rather than the name twice.
      title={
        <Typography variant="title" as="h1" className="text-content-secondary truncate">
          {method ? t('screen.talk.editingTitle', { ns: 'templates' }) : stored.name}
        </Typography>
      }
    >
      {method ? (
        <AIAuthoringStudio
          layout="conversation"
          ownerId={ownerId}
          kind="post-template"
          candidateCount={1}
          targetId={stored.id}
          targetName={stored.name}
          sessionId={activeSession}
          initialMethod={method}
          startFromSaved={!activeSession}
          renderDirectEditor={(props) => <TemplateDirectEditor {...props} />}
          onSaved={(saved) => {
            setPublished(saved)
            onPublished()
            setMethod(null)
          }}
          className="mt-4 flex-1 sm:mt-6"
        />
      ) : (
        <section aria-labelledby="template-makes" className="mt-4 pb-10 sm:mt-6">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-3">
            <div className="min-w-0 flex-1 basis-56">
              <Typography variant="stepTitle" as="h2" id="template-makes">
                {t('screen.talk.makes', { ns: 'templates' })}
              </Typography>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {stored.postCount > 0 && (
                  <Badge tone="neutral">
                    {t('usedIn', { ns: 'templates', count: stored.postCount })}
                  </Badge>
                )}
                {publication?.outcome && (
                  <Typography variant="meta" as="span" role="status">
                    {t(publication.outcome === 'updated' ? 'confirmedUpdate' : 'confirmedCreate', {
                      ns: 'authoring',
                      ...named,
                      name: publication.name,
                    })}
                  </Typography>
                )}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {unfinished && summary && (
                <Button
                  variant="ghost"
                  onClick={() => {
                    setActiveSession(summary.sessionId)
                    setMethod('ai')
                  }}
                >
                  {t('screen.talk.resume', { ns: 'templates' })}
                </Button>
              )}
              <Button
                variant="ghost"
                onClick={() => {
                  setActiveSession(undefined)
                  setMethod('direct')
                }}
              >
                <PencilLine aria-hidden="true" className="size-4" />
                {t('screen.talk.direct', { ns: 'templates' })}
              </Button>
              <Button
                variant="cta"
                onClick={() => {
                  setActiveSession(undefined)
                  setMethod('ai')
                }}
              >
                <MessageCircle aria-hidden="true" className="size-4" />
                {t('screen.talk.edit', { ns: 'templates' })}
              </Button>
            </div>
          </div>
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-4">
              {t('screen.workLoadFailed', { ns: 'templates' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {summary?.targetConflict && (
            <Notice tone="warning" className="mt-4">
              {t('screen.conflict', { ns: 'templates', name: stored.name })}
            </Notice>
          )}
          {/* The post it makes is drawn as one page (the preview's own card), not a card in a
              card; the owner's line about it sits over it. */}
          {stored.description && (
            <Typography variant="body" className="text-content-secondary mt-6 break-words">
              {stored.description}
            </Typography>
          )}
          <TemplatePreview
            titleArea={stored.titleArea}
            body={stored.body}
            className="mt-6 sm:mt-8"
          />
        </section>
      )}
    </PageFrame>
  )
}
