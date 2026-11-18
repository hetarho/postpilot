import { clsx } from 'clsx'
import { useState } from 'react'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { MessagesSquare, Palette, PencilLine } from 'lucide-react'
import { ClipTemplateSummary, useClipTemplates, type ClipTemplate } from '@/entities/clip-template'
import { useAuthoringSummaries, type AuthoringSavedRef } from '@/entities/ai-authoring'
import { useSession } from '@/entities/session'
import { DeleteClipTemplateButton } from '@/features/delete-clip-template'
import { ClipTemplateDirectEditor, ClipTemplateEditor } from '@/features/edit-clip-template'
import { ClipTemplateWizard, clipWizardSeed } from '@/features/create-clip-template-wizard'
import { useClipProject } from '@/entities/clip-project'
import { AuthoringPreview } from '@/features/ai-authoring'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { videoTemplateIllustration } from '@/shared/assets'
import { Button, LoadingState, Notice, Typography, PageFrame } from '@/shared/ui'

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
      <PageFrame width="wide">
        <Notice tone="danger" role="alert" className="mt-4">
          {t('directory.loadFailed', { ns: 'clips' })}
          <Button variant="ghost" onClick={() => void refetch()} pending={isFetching}>
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
          {t('editor.notFound', { ns: 'clips' })}
        </Notice>
      </PageFrame>
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
  if (fromClip && query.isPending) return <LoadingState variant="form" label={t('state.loading')} />
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
  // goes back to a new clip starting from it.
  const openCreated = (templateId: string) =>
    returnToClip === 'new'
      ? navigate({
          to: '/clips/new',
          search: { template: templateId },
          replace: true,
        })
      : returnToClip
        ? navigate({
            to: '/clips/$clipId',
            params: { clipId: returnToClip },
            search: { guide: 'template' },
            replace: true,
          })
        : navigate({
            to: '/video-templates/$templateId',
            params: { templateId },
            replace: true,
          })
  // A new video template is made by talking to the AI from scratch; only one started from a clip
  // keeps the seeded wizard, since its storyline and design are already decided.
  const [method, setMethod] = useState<'ai' | 'direct' | 'design' | null>(
    sessionId || (!stored && !fromClip) ? 'ai' : null,
  )
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
  const guided = !stored && !sessionId && !!fromClip
  const busyElsewhere =
    !!summary &&
    (summary.hasUnpublishedChanges ||
      !!summary.activeJobId ||
      summary.publicationPending ||
      summary.targetConflict)
  return (
    <PageFrame
      width="board"
      // The wizard's step bar meets the screen's bottom edge, so nothing pads under it.
      className={clsx('flex flex-1 flex-col py-4 sm:py-8', guided && 'pb-0 sm:pb-0')}
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
      ) : stored && method === null ? (
        // The reading keeps a column under the title, its actions in one row beside what they
        // change, as the post template's review does.
        <section
          aria-labelledby="video-template-review"
          className="mt-4 flex max-w-3xl flex-col gap-6 sm:mt-6"
        >
          <div className="flex flex-wrap items-center gap-2">
            {/* The frame's title already names it; this row says how many videos use it and
                holds what to do with it, wrapping under the count on a phone. */}
            <div className="min-w-0 flex-1 basis-40">
              <h2 id="video-template-review" className="sr-only">
                {stored.name}
              </h2>
              <Typography variant="label" as="p">
                {t('directory.projectCount', { ns: 'clips', count: stored.projectCount })}
              </Typography>
            </div>
            <DeleteClipTemplateButton
              ownerId={ownerId}
              template={stored}
              onDeleted={() => void navigate({ to: '/video-templates', replace: true })}
            />
            <Button
              variant="secondary"
              aria-label={t('directory.editDesign', { ns: 'clips', name: stored.name })}
              onClick={() => setMethod('design')}
            >
              <Palette aria-hidden="true" className="size-4" />
              {t('directory.startDesign', { ns: 'clips' })}
            </Button>
            <Button
              variant="secondary"
              aria-label={t('directNamed', { ns: 'authoring', ...named })}
              onClick={() => {
                setActiveSession(undefined)
                setMethod('direct')
              }}
            >
              <PencilLine aria-hidden="true" className="size-4" />
              {t('directory.editDirect', { ns: 'clips' })}
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
              {t('directory.editAI', { ns: 'clips' })}
            </Button>
          </div>
          {summaries.isError && (
            <Notice tone="danger" role="alert">
              {t('directory.workLoadFailed', { ns: 'clips' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {busyElsewhere && summary && (
            <Notice tone={summary.targetConflict ? 'warning' : 'info'} role="status">
              <span>
                {summary.activeJobId
                  ? t('directory.activeAI', { ns: 'clips', name: named.name })
                  : summary.publicationPending
                    ? t('directory.publicationPending', { ns: 'clips', name: named.name })
                    : summary.targetConflict
                      ? t('directory.conflict', { ns: 'clips', name: named.name })
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
          <ClipTemplateSummary template={stored} />
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
        </section>
      ) : method === 'design' ? (
        <section className="mt-8">
          <Typography variant="stepTitle" as="h2">
            {t('directory.editDesign', { ns: 'clips', name: named.name })}
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
          layout="conversation"
          candidateCount={1}
          ownerId={ownerId}
          kind="video-template"
          targetId={stored?.id}
          targetName={stored?.name}
          sessionId={activeSession}
          initialMethod={method ?? 'ai'}
          draftIllustration={videoTemplateIllustration}
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
    </PageFrame>
  )
}
