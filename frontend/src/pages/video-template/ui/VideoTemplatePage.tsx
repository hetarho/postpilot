import { clsx } from 'clsx'
import { useState } from 'react'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipTemplates, type ClipTemplate } from '@/entities/clip-template'
import { useAuthoringSummaries, type AuthoringSavedRef } from '@/entities/ai-authoring'
import { useSession } from '@/entities/session'
import { ClipTemplateDirectEditor, ClipTemplateEditor } from '@/features/edit-clip-template'
import { ClipTemplateWizard, clipWizardSeed } from '@/features/create-clip-template-wizard'
import { useClipProject } from '@/entities/clip-project'
import { AuthoringPreview } from '@/features/ai-authoring'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { Button, Notice, Typography, pageStyles } from '@/shared/ui'

export function VideoTemplatePage() {
  const { templateId } = useParams({ strict: false }) as { templateId?: string }
  const { session, clip, from } = useSearch({ strict: false }) as {
    session?: string
    clip?: 'new'
    from?: string
  }
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useClipTemplates(ownerId)
  const stored = templateId ? templates.find((template) => template.id === templateId) : undefined
  if (isError)
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <Notice tone="danger" role="alert" className="mt-4">
          {t('directory.loadFailed', { ns: 'clips' })}
          <Button variant="ghost" onClick={() => void refetch()} pending={isFetching}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      </main>
    )
  if (templateId && (isPending || (!stored && isFetching)))
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <Typography variant="body" role="status" className="mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      </main>
    )
  if (templateId && !stored && !isFetching)
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <Notice tone="danger" role="alert" className="mt-4">
          {t('editor.notFound', { ns: 'clips' })}
        </Notice>
      </main>
    )
  return (
    <VideoTemplateHost
      key={JSON.stringify([ownerId, stored?.id ?? 'new', stored ? '' : session])}
      ownerId={ownerId}
      stored={stored}
      sessionId={session}
      returnToClip={stored ? undefined : clip}
      fromClip={stored || session ? undefined : from}
      onPublished={refetch}
    />
  )
}
/** The wizard, started from a clip when one is named: it waits for that clip, then opens with its
 *  storyline paragraphs as stages and its design selection chosen. A clip that cannot be read
 *  leaves the ordinary wizard. Reading the clip starts no work and changes nothing on it. */
function SeededClipTemplateWizard({
  ownerId,
  fromClip,
  onSaved,
}: {
  ownerId: string
  fromClip?: string
  onSaved: (templateId: string) => void
}) {
  const { t } = useTranslation('common')
  const query = useClipProject(ownerId, fromClip)
  if (fromClip && query.isPending)
    return (
      <Typography variant="body" role="status">
        {t('state.loading')}
      </Typography>
    )
  const project = fromClip ? query.data : undefined
  const seed = project
    ? clipWizardSeed({
        paragraphs: project.storyline?.paragraphs ?? [],
        design: project,
      })
    : undefined
  return (
    <ClipTemplateWizard
      key={project?.id ?? 'new'}
      ownerId={ownerId}
      seed={seed}
      onSaved={onSaved}
    />
  )
}

function VideoTemplateHost({
  ownerId,
  stored,
  sessionId,
  returnToClip,
  fromClip,
  onPublished,
}: {
  ownerId: string
  stored?: ClipTemplate
  sessionId?: string
  /** The clip whose storyline and design the wizard starts from (CLIP-233). */
  fromClip?: string
  /** `new`: a new clip's offer opened this creation (CLIP-223); an id: a saved clip's 템플릿
   *  screen did, and 저장 goes back to that screen (CLIP-225). */
  returnToClip?: string
  onPublished: () => void
}) {
  const { t } = useTranslation(['clips', 'authoring', 'common'])
  const navigate = useNavigate()
  // A new template opens its own page, unless a new clip's offer sent the owner here: then 저장
  // goes back to a new clip in that channel starting from it.
  const openCreated = (templateId: string) =>
    returnToClip === 'new'
      ? navigate({
          to: '/clips/new',
          search: ({ channel }) => ({ channel, template: templateId }),
          replace: true,
        })
      : returnToClip
        ? navigate({
            to: '/clips/$clipId',
            params: { clipId: returnToClip },
            search: ({ channel }) => ({ channel, guide: 'template' }),
            replace: true,
          })
        : navigate({
            to: '/video-templates/$templateId',
            params: { templateId },
            search: ({ channel }) => ({ channel }),
            replace: true,
          })
  const [method, setMethod] = useState<'ai' | 'direct' | 'design' | null>(sessionId ? 'ai' : null)
  const [activeSession, setActiveSession] = useState(sessionId)
  const [published, setPublished] = useState<AuthoringSavedRef>()
  const summaries = useAuthoringSummaries({ ownerId, kind: 'video-template' })
  const summary = stored
    ? summaries.data?.find(
        (row) => row.targetId === stored.id || row.lastPublication?.id === stored.id,
      )
    : undefined
  const publication =
    published ??
    (stored && summary?.lastPublication?.id === stored.id ? summary.lastPublication : undefined)
  const named = {
    kind: t('kinds.video-template', { ns: 'authoring' }),
    name: stored?.name ?? t('directory.create', { ns: 'clips' }),
  }
  const guided = !stored && !sessionId
  return (
    <main
      className={pageStyles({
        width: 'board',
        // The wizard's step bar meets the screen's bottom edge, so nothing pads under it.
        className: clsx('flex flex-1 flex-col py-4 sm:py-8', guided && 'pb-0 sm:pb-0'),
      })}
    >
      <Typography variant="title" as="h1" className="text-content-secondary">
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
      {guided ? (
        <div className="mt-4 flex flex-1 flex-col sm:mt-8">
          <SeededClipTemplateWizard
            ownerId={ownerId}
            fromClip={fromClip}
            onSaved={(templateId) => {
              onPublished()
              void openCreated(templateId)
            }}
          />
        </div>
      ) : method === null ? (
        <section className="mt-4 flex flex-col sm:mt-8">
          <Typography variant="stepTitle" as="h2">
            {stored ? (
              <>
                <span className="sm:hidden">{t('directory.reviewKind', { ns: 'clips' })}</span>
                <span className="hidden sm:inline">
                  {t('reviewNamed', { ns: 'authoring', ...named })}
                </span>
              </>
            ) : (
              t('directory.chooseCreation', { ns: 'clips' })
            )}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-2 sm:mt-4">
            {stored ? (
              <>
                <span className="sm:hidden">{t('directory.savedAvailable', { ns: 'clips' })}</span>
                <span className="hidden sm:inline">
                  {t('savedUsable', { ns: 'authoring', ...named })}
                </span>
              </>
            ) : (
              t('directory.unsaved', { ns: 'clips' })
            )}
          </Typography>
          {summaries.isPending && (
            <Typography variant="body" role="status" className="mt-4">
              {t('directory.checkingWork', { ns: 'clips' })}
            </Typography>
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-4">
              {t('directory.workLoadFailed', { ns: 'clips' })}
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
                  {t('directory.activeAI', { ns: 'clips', name: named.name })}
                </Typography>
              )}
              {summary.publicationPending && (
                <Typography variant="body">
                  {t('directory.publicationPending', { ns: 'clips', name: named.name })}
                </Typography>
              )}
              {summary.targetConflict && (
                <Notice tone="warning">
                  {t('directory.conflict', { ns: 'clips', name: named.name })}
                </Notice>
              )}
            </div>
          )}
          {stored && (
            <div className="order-2 mt-4 sm:order-none sm:mt-6">
              <AuthoringPreview
                kind="video-template"
                artifact={{
                  id: stored.id,
                  name: stored.name,
                  description: '',
                  body: stored.compositionBody,
                  titleArea: '',
                }}
              />
            </div>
          )}
          <div className="order-1 mt-4 flex flex-wrap gap-2 sm:order-none sm:mt-8 sm:gap-4">
            <Button
              aria-label={stored ? t('aiNamed', { ns: 'authoring', ...named }) : undefined}
              variant="secondary"
              onClick={() => {
                setActiveSession(undefined)
                setMethod('ai')
              }}
            >
              {stored ? (
                <>
                  <span className="sm:hidden">{t('directory.editAI', { ns: 'clips' })}</span>
                  <span className="hidden sm:inline">
                    {t('aiNamed', { ns: 'authoring', ...named })}
                  </span>
                </>
              ) : (
                t('directory.createAI', { ns: 'clips' })
              )}
            </Button>
            <Button
              aria-label={stored ? t('directNamed', { ns: 'authoring', ...named }) : undefined}
              variant="secondary"
              onClick={() => {
                setActiveSession(undefined)
                setMethod('direct')
              }}
            >
              {stored ? (
                <>
                  <span className="sm:hidden">{t('directory.editDirect', { ns: 'clips' })}</span>
                  <span className="hidden sm:inline">
                    {t('directNamed', { ns: 'authoring', ...named })}
                  </span>
                </>
              ) : (
                t('host.manual', { ns: 'authoring' })
              )}
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
            {stored && (
              <Button variant="ghost" onClick={() => setMethod('design')}>
                {t('directory.editDesign', { ns: 'clips', name: stored.name })}
              </Button>
            )}
          </div>
        </section>
      ) : method === 'design' ? (
        <section className="mt-8">
          <Typography variant="stepTitle" as="h2">
            {t('directory.editDesign', { ns: 'clips', name: named.name })}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-3">
            {t('directory.designHelp', { ns: 'clips' })}
          </Typography>
          <ClipTemplateEditor
            ownerId={ownerId}
            stored={stored}
            designOnly
            onClose={() => {
              onPublished()
              setMethod(null)
            }}
          />
        </section>
      ) : (
        <AIAuthoringStudio
          ownerId={ownerId}
          kind="video-template"
          targetId={stored?.id}
          targetName={stored?.name}
          sessionId={activeSession}
          initialMethod={method}
          startFromSaved={!activeSession}
          renderDirectEditor={(props) => <ClipTemplateDirectEditor {...props} />}
          onSaved={(saved) => {
            setPublished(saved)
            onPublished()
            if (stored) setMethod(null)
            else void openCreated(saved.id)
          }}
          className="mt-8"
        />
      )}
    </main>
  )
}
