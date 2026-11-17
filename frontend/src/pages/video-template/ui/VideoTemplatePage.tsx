import { useState } from 'react'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ClipTemplateSummary, useClipTemplates, type ClipTemplate } from '@/entities/clip-template'
import { useAuthoringSummaries } from '@/entities/ai-authoring'
import { useClipProject } from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { DeleteClipTemplateButton } from '@/features/delete-clip-template'
import { ClipTemplateDirectEditor, ClipTemplateEditor } from '@/features/edit-clip-template'
import { AuthoringPreview } from '@/features/ai-authoring'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
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
    <VideoTemplateConversationPage
      key={JSON.stringify([ownerId, stored?.id ?? 'new', stored ? '' : session])}
      ownerId={ownerId}
      stored={stored}
      sessionId={stored ? undefined : session}
      returnToClip={stored ? undefined : clip}
      fromClip={stored || session ? undefined : from}
      onPublished={() => void refetch()}
    />
  )
}

/** How much of each storyline paragraph the first message quotes. */
const PARAGRAPH_CHARS = 80

/** Guide: a video template is made by talking, as a post template is (TMPL departure, see the
 *  showcase notes). A new one opens on the conversation — the owner says what their videos show
 *  and the AI drafts the whole template; an existing one opens on itself with the conversation
 *  beside it. Typing the template text is one ⋯ choice away, and the intro, outro and caption
 *  design is the template's own second screen (시작 디자인). Nothing is dragged.
 *
 *  Started from a clip (`?from=`), the first message is typed for the owner from that clip's
 *  storyline — never sent until they send it. A new clip's offer (`?clip=new`) or a saved clip's
 *  템플릿 screen (`?clip=<id>`) gets the owner back there once the template is saved. */
function VideoTemplateConversationPage({
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
  /** `new`: a new clip's offer opened this creation (CLIP-223); an id: a saved clip's 템플릿
   *  screen did, and 저장 goes back to that screen (CLIP-225). */
  returnToClip?: string
  /** The clip whose storyline the first message is typed from. */
  fromClip?: string
  onPublished: () => void
}) {
  const { t } = useTranslation(['clips', 'common'])
  const navigate = useNavigate()
  const [designing, setDesigning] = useState(false)
  // The conversation's status and its ⋯ and 저장 sit in this screen's one title row.
  const [statusSlot, setStatusSlot] = useState<HTMLDivElement | null>(null)
  const [controlsSlot, setControlsSlot] = useState<HTMLDivElement | null>(null)
  const [deleting, setDeleting] = useState(false)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'video-template' })
  const clip = useClipProject(ownerId, fromClip)
  // An existing template with unsaved conversation work reopens that work.
  const resumable = stored
    ? summaries.data?.find(
        (row) =>
          row.targetId === stored.id &&
          (row.hasUnpublishedChanges || row.activeJobId || row.publicationPending),
      )
    : undefined
  const session = sessionId ?? resumable?.sessionId
  const paragraphs = fromClip ? (clip.data?.storyline?.paragraphs ?? []) : []
  const firstMessage = paragraphs.length
    ? t('directory.fromClipMessage', {
        ns: 'clips',
        outline: paragraphs
          .map((paragraph, index) => {
            const text = paragraph.text.replace(/\s+/gu, ' ').trim()
            const cut =
              Array.from(text).length > PARAGRAPH_CHARS
                ? Array.from(text).slice(0, PARAGRAPH_CHARS).join('') + '…'
                : text
            return `${index + 1}. ${cut}`
          })
          .join('\n'),
      })
    : undefined
  if ((stored && summaries.isPending) || (fromClip && clip.isPending))
    return (
      <PageFrame width="wide">
        <LoadingState variant="detail" label={t('state.loading', { ns: 'common' })} />
      </PageFrame>
    )
  const openCreated = (templateId: string) =>
    returnToClip === 'new'
      ? navigate({ to: '/clips/new', search: { template: templateId }, replace: true })
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
  return (
    <PageFrame
      width="wide"
      className="flex flex-1 flex-col pb-0 sm:pb-0"
      title={
        <div className="min-w-0">
          <Typography variant="title" as="h1" className="truncate">
            {stored?.name ?? t('directory.create', { ns: 'clips' })}
          </Typography>
          <div ref={setStatusSlot} />
        </div>
      }
      actions={<div ref={setControlsSlot} className="flex items-center gap-2" />}
    >
      {stored && (
        <DeleteClipTemplateButton
          ownerId={ownerId}
          template={stored}
          menu={{ open: deleting, onOpenChange: setDeleting }}
          onDeleted={() => void navigate({ to: '/video-templates', replace: true })}
        />
      )}
      {designing && stored ? (
        <section className="flex flex-1 flex-col">
          <Typography variant="stepTitle" as="h2" className="mt-2">
            {t('directory.design', { ns: 'clips' })}
          </Typography>
          <ClipTemplateEditor
            ownerId={ownerId}
            stored={stored}
            designOnly
            onClose={() => {
              onPublished()
              setDesigning(false)
            }}
          />
        </section>
      ) : (
        <AIAuthoringStudio
          layout="conversation"
          ownerId={ownerId}
          kind="video-template"
          targetId={stored?.id}
          targetName={stored?.name}
          sessionId={session}
          initialMethod={session ? 'ai' : undefined}
          startFromSaved={!session}
          initialMessage={firstMessage}
          savedPreview={
            stored && (
              <div className="space-y-6">
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
              </div>
            )
          }
          renderDirectEditor={(props) => <ClipTemplateDirectEditor {...props} />}
          heading={{ status: statusSlot, controls: controlsSlot }}
          menuItems={
            stored
              ? [
                  {
                    id: 'design',
                    label: t('directory.designMenu', { ns: 'clips' }),
                    onSelect: () => setDesigning(true),
                  },
                  {
                    id: 'delete',
                    label: t('directory.deleteMenu', { ns: 'clips' }),
                    onSelect: () => setDeleting(true),
                  },
                ]
              : undefined
          }
          onSaved={(saved) => {
            onPublished()
            if (!stored) void openCreated(saved.id)
          }}
          className="flex-1"
        />
      )}
    </PageFrame>
  )
}
