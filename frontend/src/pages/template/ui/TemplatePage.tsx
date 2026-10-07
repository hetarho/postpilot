import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { usePost } from '@/entities/post'
import {
  AUTHORING_REFERENCE_POST_MAX_CHARS,
  useAuthoringSummaries,
  type AuthoringSavedRef,
} from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { TemplateDirectEditor, postFormReference } from '@/features/edit-template'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { Button, Notice, Typography, pageStyles, typographyStyles } from '@/shared/ui'

export function TemplatePage() {
  const { templateId } = useParams({ strict: false }) as { templateId?: string }
  const { from, session } = useSearch({ strict: false }) as { from?: string; session?: string }
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const stored = templateId ? templates.find((template) => template.id === templateId) : undefined
  if (isError)
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Notice tone="danger" role="alert" className="mt-4">
          {t('loadFailed', { ns: 'templates' })}
          <Button variant="ghost" onClick={refetch} pending={isFetching}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      </main>
    )
  if (templateId && (isPending || (!stored && isFetching)))
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Typography variant="body" role="status" className="mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      </main>
    )
  if (templateId && !stored && !isFetching)
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Notice tone="danger" role="alert" className="mt-4">
          {t('screen.notFound', { ns: 'templates' })}
        </Notice>
      </main>
    )
  return (
    <TemplateAuthoringPage
      key={JSON.stringify([ownerId, stored?.id ?? 'new'])}
      ownerId={ownerId}
      stored={stored}
      sampleSlug={stored ? undefined : from}
      sessionId={stored ? undefined : session}
      onPublished={refetch}
    />
  )
}

function TemplateAuthoringPage({
  ownerId,
  stored,
  sampleSlug,
  sessionId,
  onPublished,
}: {
  ownerId: string
  stored?: Template
  sampleSlug?: string
  sessionId?: string
  onPublished: () => void
}) {
  const { t } = useTranslation(['templates', 'authoring', 'common'])
  const navigate = useNavigate()
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const summary = summaries.data?.find(
    (row) => row.targetId === stored?.id || (!!stored && row.lastPublication?.id === stored.id),
  )
  const [method, setMethod] = useState<'ai' | 'direct' | null>(sessionId ? 'ai' : null)
  const [activeSession, setActiveSession] = useState(sessionId)
  const [sample, setSample] = useState(sampleSlug ?? '')
  const [published, setPublished] = useState<AuthoringSavedRef>()
  const samplePost = usePost(sample, { enabled: sample !== '' })
  const reference = samplePost.post?.content
    ? postFormReference(samplePost.post.content)
    : undefined
  const tooLong =
    reference !== undefined && Array.from(reference).length > AUTHORING_REFERENCE_POST_MAX_CHARS
  const publication =
    published ??
    (stored && summary?.lastPublication?.id === stored.id ? summary.lastPublication : undefined)
  const named = {
    kind: t('kinds.post-template', { ns: 'authoring' }),
    name: stored?.name ?? t('page.new', { ns: 'templates' }),
  }
  return (
    <main className={pageStyles({ width: 'board', className: 'flex flex-1 flex-col py-8' })}>
      <BackLink />
      <Typography variant="title" as="h1" className="text-content-secondary mt-6">
        {named.name}
      </Typography>
      {publication?.outcome && (
        <Typography variant="body" role="status" className="mt-4">
          {t(publication.outcome === 'updated' ? 'confirmedUpdate' : 'confirmedCreate', {
            ns: 'authoring',
            ...named,
            name: publication.name,
          })}
        </Typography>
      )}
      {method === null ? (
        <section className="mt-8">
          <Typography variant="stepTitle" as="h2">
            {t(stored ? 'reviewNamed' : 'screen.chooseCreation', {
              ns: stored ? 'authoring' : 'templates',
              ...named,
            })}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-4">
            {stored
              ? t('savedUsable', { ns: 'authoring', ...named })
              : t('screen.unsaved', { ns: 'templates' })}
          </Typography>
          {summaries.isPending && (
            <Typography variant="body" role="status" className="mt-4">
              {t('screen.checkingWork', { ns: 'templates' })}
            </Typography>
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-4">
              {t('screen.workLoadFailed', { ns: 'templates' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {summary && (
            <div role="status" className="mt-4 space-y-2">
              {summary.hasUnpublishedChanges && (
                <Typography variant="body">
                  {t('unpublished', { ns: 'authoring', ...named })}
                </Typography>
              )}
              {summary.activeJobId && (
                <Typography variant="body">
                  {t('screen.activeAI', { ns: 'templates', name: named.name })}
                </Typography>
              )}
              {summary.publicationPending && (
                <Typography variant="body">
                  {t('screen.publicationPending', { ns: 'templates', name: named.name })}
                </Typography>
              )}
              {summary.targetConflict && (
                <Notice tone="warning">
                  {t('screen.conflict', { ns: 'templates', name: named.name })}
                </Notice>
              )}
            </div>
          )}
          {stored && (
            <div className="mt-6">
              <TemplatePreview titleArea={stored.titleArea} body={stored.body} />
            </div>
          )}
          {sample !== '' && (
            <div className="mt-6">
              <Typography variant="body">
                {t('screen.reference', {
                  ns: 'templates',
                  name: samplePost.post?.content?.title || samplePost.post?.title || sample,
                })}
              </Typography>
              <Typography variant="body" className="text-content-secondary mt-2">
                {t('screen.referencePrivacy', { ns: 'templates' })}
              </Typography>
              {(samplePost.failure || (samplePost.post && !samplePost.post.content) || tooLong) && (
                <Notice tone="danger" role="alert" className="mt-3">
                  {t(tooLong ? 'screen.referenceTooLong' : 'screen.referenceMissing', {
                    ns: 'templates',
                    max: AUTHORING_REFERENCE_POST_MAX_CHARS,
                  })}
                </Notice>
              )}
              <Button variant="ghost" onClick={() => setSample('')} className="mt-3">
                {t('screen.removeReference', { ns: 'templates' })}
              </Button>
            </div>
          )}
          <div className="mt-8 flex flex-wrap gap-4">
            <Button
              variant="secondary"
              disabled={!!sample && (!reference || tooLong)}
              onClick={() => {
                setActiveSession(undefined)
                setMethod('ai')
              }}
            >
              {stored
                ? t('aiNamed', { ns: 'authoring', ...named })
                : t('screen.createAI', { ns: 'templates' })}
            </Button>
            <Button
              variant="secondary"
              disabled={!!sample && (!reference || tooLong)}
              onClick={() => {
                setActiveSession(undefined)
                setMethod('direct')
              }}
            >
              {stored
                ? t('directNamed', { ns: 'authoring', ...named })
                : t('host.manual', { ns: 'authoring' })}
            </Button>
            {summary &&
              (summary.hasUnpublishedChanges ||
                summary.activeJobId ||
                summary.publicationPending ||
                summary.targetConflict) && (
                <Button
                  variant="ghost"
                  onClick={() => {
                    setActiveSession(summary.sessionId)
                    setMethod('ai')
                  }}
                >
                  {t('continueNamed', { ns: 'authoring', ...named })}
                </Button>
              )}
          </div>
        </section>
      ) : (
        <AIAuthoringStudio
          ownerId={ownerId}
          kind="post-template"
          targetId={stored?.id}
          targetName={stored?.name}
          sessionId={activeSession}
          initialMethod={method}
          startFromSaved={!activeSession}
          referencePost={sample && !tooLong ? reference : undefined}
          renderDirectEditor={(props) => <TemplateDirectEditor {...props} />}
          onSaved={(saved) => {
            setPublished(saved)
            onPublished()
            setMethod(null)
            if (!stored)
              void navigate({
                to: '/templates/$templateId',
                params: { templateId: saved.id },
                replace: true,
              })
          }}
          className="mt-8"
        />
      )}
    </main>
  )
}

function BackLink() {
  const { t } = useTranslation('templates')
  return (
    <Link
      to="/templates"
      className={typographyStyles({
        variant: 'label',
        className:
          'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center underline',
      })}
    >
      {t('screen.backToList')}
    </Link>
  )
}
