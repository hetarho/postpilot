import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { MessagesSquare, PencilLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { usePost } from '@/entities/post'
import {
  AUTHORING_REFERENCE_POST_MAX_CHARS,
  useAuthoringSummaries,
  type AuthoringSavedRef,
} from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { DeleteTemplateButton } from '@/features/delete-template'
import { TemplateDirectEditor } from '@/features/edit-template'
import { BlockType } from '@/shared/api'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { templateIllustration } from '@/shared/assets'
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
    <TemplateAuthoringPage
      key={JSON.stringify([
        ownerId,
        stored?.id ?? 'new',
        stored ? '' : session,
        stored ? '' : from,
      ])}
      ownerId={ownerId}
      stored={stored}
      sampleSlug={stored ? undefined : from}
      sessionId={stored ? undefined : session}
      returnPost={stored ? undefined : post}
      onPublished={refetch}
    />
  )
}

/** Guide: a template is made by talking to the AI, from scratch. A new one opens straight on the
 *  conversation — the AI asks what the owner posts, drafts the post's order from the answer, and
 *  every later message refines it beside the live draft until 저장. No block is dragged, moved or
 *  reordered by hand anywhere on the way.
 *
 *  A saved template opens on what it makes — its preview — with the two ways to change it in the
 *  heading row: AI와 고치기 (the same conversation, starting from it) and 직접 고치기 (its source
 *  text, the secondary path). Deleting it lives here too, where it is being looked at. */
function TemplateAuthoringPage({
  ownerId,
  stored,
  sampleSlug,
  sessionId,
  returnPost,
  onPublished,
}: {
  ownerId: string
  stored?: Template
  sampleSlug?: string
  sessionId?: string
  /** The post whose 템플릿 picker opened this, or `new` for an unsaved draft (TMPL-32). */
  returnPost?: string
  onPublished: () => void
}) {
  const { t } = useTranslation(['templates', 'authoring', 'common'])
  const navigate = useNavigate()
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const summary = summaries.data?.find(
    (row) => row.targetId === stored?.id || (!!stored && row.lastPublication?.id === stored.id),
  )
  const [method, setMethod] = useState<'ai' | 'direct' | null>(stored && !sessionId ? null : 'ai')
  const [activeSession, setActiveSession] = useState(sessionId)
  const [published, setPublished] = useState<AuthoringSavedRef>()
  // 이 글 형식으로 템플릿 만들기 (TMPL-64): the post's title and its body's STRUCTURE — headings as
  // written, every other block named by its kind — travel as the conversation's form reference.
  // Its prose, captions, photo files, memo and tags never do (screen.referencePrivacy).
  const sample = usePost(sampleSlug ?? '', { enabled: !stored && !!sampleSlug })
  const structure = (sample.post?.content?.blocks ?? []).map((block) =>
    block.type === BlockType.HEADING
      ? `## ${block.content}`
      : block.type === BlockType.IMAGE
        ? t('screen.referencePhoto', { ns: 'templates' })
        : t('screen.referenceText', { ns: 'templates' }),
  )
  const reference = sample.post?.content
    ? Array.from(
        [
          sample.post.title,
          ...structure.filter(
            (line, index) => line.startsWith('## ') || line !== structure[index - 1],
          ),
        ]
          .filter((line) => line.trim() !== '')
          .join('\n'),
      )
        .slice(0, AUTHORING_REFERENCE_POST_MAX_CHARS)
        .join('')
    : ''
  const publication =
    published ??
    (stored && summary?.lastPublication?.id === stored.id ? summary.lastPublication : undefined)
  const named = {
    kind: t('kinds.post-template', { ns: 'authoring' }),
    name: stored?.name ?? t('page.new', { ns: 'templates' }),
  }
  const busyElsewhere =
    !!summary &&
    (summary.hasUnpublishedChanges ||
      !!summary.activeJobId ||
      summary.publicationPending ||
      summary.targetConflict)
  return (
    <PageFrame
      width="board"
      className="flex flex-1 flex-col py-4 sm:py-8"
      title={
        <Typography variant="title" as="h1" className="text-content-secondary truncate">
          {named.name}
        </Typography>
      }
    >
      {publication?.outcome && (
        <Typography variant="body" role="status" className="mt-4">
          {t(publication.outcome === 'updated' ? 'confirmedUpdate' : 'confirmedCreate', {
            ns: 'authoring',
            ...named,
            name: publication.name,
          })}
        </Typography>
      )}
      {!stored && sampleSlug && sample.isPending ? (
        <LoadingState
          variant="form"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8"
        />
      ) : !stored && sampleSlug && (sample.failure || !sample.post?.content) ? (
        <Notice tone="danger" role="alert" className="mt-6">
          {t('screen.referenceMissing', { ns: 'templates' })}
          <Button
            variant="ghost"
            onClick={() =>
              void navigate({
                to: '/templates/new',
                search: returnPost ? { post: returnPost } : {},
                replace: true,
              })
            }
          >
            {t('screen.removeReference', { ns: 'templates' })}
          </Button>
        </Notice>
      ) : stored && method === null ? (
        <section aria-labelledby="template-review" className="mt-4 flex flex-col gap-6 sm:mt-6">
          <div className="border-divider flex flex-wrap items-center gap-2 border-b pb-3">
            {/* The frame's title already names it; this row says how many posts use it and
                holds the three things to do with it, wrapping under the count on a phone. */}
            <div className="min-w-0 flex-1 basis-40">
              <h2 id="template-review" className="sr-only">
                {stored.name}
              </h2>
              <Typography variant="label" as="p">
                {t('postCount', { ns: 'templates', count: stored.postCount })}
              </Typography>
            </div>
            <DeleteTemplateButton
              ownerId={ownerId}
              template={stored}
              onDeleted={() => void navigate({ to: '/templates', replace: true })}
            />
            <Button
              variant="secondary"
              aria-label={t('directNamed', { ns: 'authoring', ...named })}
              onClick={() => {
                setActiveSession(undefined)
                setMethod('direct')
              }}
            >
              <PencilLine aria-hidden="true" className="size-4" />
              {t('screen.editDirect', { ns: 'templates' })}
            </Button>
            <Button
              variant="cta"
              aria-label={t('aiNamed', { ns: 'authoring', ...named })}
              onClick={() => {
                setActiveSession(undefined)
                setMethod('ai')
              }}
            >
              <MessagesSquare aria-hidden="true" className="size-4" />
              {t('screen.editAI', { ns: 'templates' })}
            </Button>
          </div>
          {stored.description && (
            <Typography variant="body" className="text-content-secondary max-w-measure">
              {stored.description}
            </Typography>
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert">
              {t('screen.workLoadFailed', { ns: 'templates' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {busyElsewhere && summary && (
            <Notice tone={summary.targetConflict ? 'warning' : 'info'} role="status">
              <span>
                {summary.activeJobId
                  ? t('screen.activeAI', { ns: 'templates', name: named.name })
                  : summary.publicationPending
                    ? t('screen.publicationPending', { ns: 'templates', name: named.name })
                    : summary.targetConflict
                      ? t('screen.conflict', { ns: 'templates', name: named.name })
                      : t('unpublished', { ns: 'authoring', ...named })}
              </span>
              <Button
                variant="ghost"
                onClick={() => {
                  setActiveSession(summary.sessionId)
                  setMethod('ai')
                }}
              >
                {t('continueNamed', { ns: 'authoring', ...named })}
              </Button>
            </Notice>
          )}
          <TemplatePreview titleArea={stored.titleArea} body={stored.body} />
        </section>
      ) : (
        <AIAuthoringStudio
          layout="conversation"
          candidateCount={1}
          draftIllustration={templateIllustration}
          ownerId={ownerId}
          kind="post-template"
          targetId={stored?.id}
          targetName={stored?.name}
          sessionId={activeSession}
          initialMethod={method ?? 'ai'}
          startFromSaved={!activeSession}
          referencePost={reference || undefined}
          renderDirectEditor={(props) => <TemplateDirectEditor {...props} />}
          onSaved={(saved) => {
            setPublished(saved)
            onPublished()
            if (stored) setMethod(null)
            // Opened from a post's picker: back to that post with the template assigned.
            else if (returnPost === 'new')
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
          className="mt-4 sm:mt-6"
        />
      )}
    </PageFrame>
  )
}
