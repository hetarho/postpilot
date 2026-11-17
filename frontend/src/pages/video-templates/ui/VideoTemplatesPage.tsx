import { Clapperboard, Plus } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useRef, type Ref } from 'react'
import {
  ClipTemplateDescription,
  useClipTemplates,
  type ClipTemplate,
} from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { DeleteClipTemplateButton } from '@/features/delete-clip-template'
import { KitScreenHeader } from '@/widgets/writing-kit'
import { templateIllustration } from '@/shared/assets'
import {
  Badge,
  DirectoryItem,
  Button,
  Notice,
  LoadingState,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
  useListRemovalFocus,
} from '@/shared/ui'

/** Saved video compositions share directory controls while showing their own visual vocabulary. */
export function VideoTemplatesPage() {
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isFetching, isError, refetch } = useClipTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'video-template' })
  const page = useRef<HTMLElement>(null)
  const deleteButtons = useRef(new Map<string, HTMLButtonElement>())
  const requestRemoval = useListRemovalFocus({
    keys: templates.map((template) => template.id),
    getTarget: (id) => deleteButtons.current.get(id),
    getFallback: () =>
      page.current?.querySelector<HTMLAnchorElement>('a[href="/video-templates/new"]'),
  })

  return (
    <main
      ref={page}
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}
    >
      {/* The same opening as its siblings under 영상 — 영상 지침 and 목소리 — with the one way to
          make a template in the heading row. */}
      <KitScreenHeader
        size="section"
        title={t('directory.title', { ns: 'clips' })}
        statement={t('directory.statement', { ns: 'clips' })}
        illustration={templateIllustration}
        actions={
          <Link
            to="/video-templates/new"
            className={buttonStyles({ variant: 'cta', className: 'gap-2' })}
          >
            <Plus aria-hidden="true" className="size-5 shrink-0" />
            {t('directory.new', { ns: 'clips' })}
          </Link>
        }
      />

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('directory.loadFailed', { ns: 'clips' })}</span>
          {/* `isFetching`, not `isPending`: react-query keeps `status: 'error'` across a refetch,
              so without it the notice does not move for the seconds a retry takes on cellular. */}
          <Button
            variant="ghost"
            onClick={() => void refetch()}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}
      {!isError && isPending && (
        <LoadingState
          variant="cards"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8 [&>.loading-reveal]:gap-3 sm:[&>.loading-reveal]:grid-cols-1 lg:[&>.loading-reveal]:grid-cols-2"
        />
      )}

      {!isError && !isPending && (
        <>
          {summaries.isPending && (
            <LoadingState
              variant="inline"
              label={t('directory.checkingWork', { ns: 'clips' })}
              className="mt-6"
            />
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('directory.workLoadFailed', { ns: 'clips' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="video-templates-heading" className="mt-6">
              <Typography variant="title" id="video-templates-heading">
                {t('directory.saved', { ns: 'clips' })}
              </Typography>
              {/* Saved resources share one card composition with the writing catalog. */}
              <ul className="mt-3 grid gap-3 lg:grid-cols-2 lg:gap-4">
                {templates.map((template, index) => (
                  <TemplateRow
                    key={template.id}
                    ownerId={ownerId}
                    template={template}
                    deleteButtonRef={(button) => {
                      if (button) deleteButtons.current.set(template.id, button)
                      else deleteButtons.current.delete(template.id)
                    }}
                    onDeleted={() => requestRemoval(template.id, index)}
                    summary={summaries.data?.find(
                      (row) =>
                        row.targetId === template.id || row.lastPublication?.id === template.id,
                    )}
                  />
                ))}
              </ul>
            </section>
          )}
          {summaries.data?.some((row) => !row.targetId && !row.lastPublication) && (
            <section className="mt-8" aria-labelledby="video-templates-unsaved-heading">
              <Typography variant="title" as="h2" id="video-templates-unsaved-heading">
                {t('directory.unsavedWork', { ns: 'clips' })}
              </Typography>
              <ul className="mt-3 grid gap-3 lg:grid-cols-2 lg:gap-4">
                {summaries.data
                  .filter((row) => !row.targetId && !row.lastPublication)
                  .map((row) => (
                    <DirectoryItem
                      key={row.sessionId}
                      interactive
                      icon={<Clapperboard className="size-6" />}
                      title={
                        <Link
                          to="/video-templates/new"
                          search={(previous) => ({ ...previous, session: row.sessionId })}
                          className={typographyStyles({
                            variant: 'fieldTitle',
                            className:
                              'text-content-primary break-words after:absolute after:inset-0',
                          })}
                        >
                          {t('directory.resumeNew', {
                            ns: 'clips',
                            name: row.displayName || t('directory.create', { ns: 'clips' }),
                          })}
                        </Link>
                      }
                      description={
                        <Typography variant="meta" as="p">
                          {t('directory.unsaved', { ns: 'clips' })}
                        </Typography>
                      }
                      status={<EditingStatus summary={row} />}
                    />
                  ))}
              </ul>
            </section>
          )}
        </>
      )}
    </main>
  )
}

/** Plain language and no example: the empty state says what a video template is FOR and lets the
 *  editor teach its own vocabulary. It creates nothing — there are no shipped presets. */
function EmptyState() {
  const { t } = useTranslation('clips')
  return (
    <section aria-labelledby="video-templates-empty-heading" className="mt-8">
      <Typography variant="title" id="video-templates-empty-heading">
        {t('directory.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('directory.emptyHelp')}
      </Typography>
    </section>
  )
}

/** One template, one target. The link stretches over the whole card through its `::after`, so the
 *  padding and the empty space navigate too, while the delete paints above that layer and acts
 *  without navigating — a row is one target, not a row with a button inside it (THEME-23). */
function TemplateRow({
  ownerId,
  template,
  summary,
  deleteButtonRef,
  onDeleted,
}: {
  ownerId: string
  template: ClipTemplate
  summary?: AuthoringSummary
  deleteButtonRef: Ref<HTMLButtonElement>
  onDeleted: () => void
}) {
  const { t } = useTranslation('clips')
  return (
    <DirectoryItem
      interactive
      icon={<Clapperboard className="size-6" />}
      title={
        <Link
          to="/video-templates/$templateId"
          params={{ templateId: template.id }}
          className={typographyStyles({
            variant: 'fieldTitle',
            className: 'text-content-primary line-clamp-2 break-words after:absolute after:inset-0',
          })}
        >
          {template.name}
        </Link>
      }
      description={<ClipTemplateDescription template={template} className="line-clamp-2" />}
      status={summary && <EditingStatus summary={summary} />}
      metadata={
        <>
          <Badge tone="neutral">
            {t('directory.projectCount', { count: template.projectCount })}
          </Badge>
          <Typography variant="meta" as="span">
            {t('directory.savedAvailable')}
          </Typography>
        </>
      }
      actions={
        <DeleteClipTemplateButton
          ownerId={ownerId}
          template={template}
          buttonRef={deleteButtonRef}
          onDeleted={onDeleted}
        />
      }
    />
  )
}
function EditingStatus({ summary }: { summary?: AuthoringSummary }) {
  const { t } = useTranslation(['clips', 'authoring'])
  if (!summary) return null
  const named = {
    kind: t('kinds.video-template', { ns: 'authoring' }),
    name: summary.displayName || t('directory.create', { ns: 'clips' }),
  }
  return (
    <div role="status" className="mt-1 space-y-1">
      {summary.lastPublication?.outcome && (
        <Typography variant="meta" as="p">
          {t('directory.lastPublication', {
            ns: 'clips',
            result: t(
              summary.lastPublication.outcome === 'created' ? 'confirmedCreate' : 'confirmedUpdate',
              { ns: 'authoring', kind: named.kind, name: summary.lastPublication.name },
            ),
          })}
        </Typography>
      )}
      {summary.hasUnpublishedChanges && (
        <Typography variant="meta" as="p">
          {t('unpublished', { ns: 'authoring', ...named })}
        </Typography>
      )}
      {summary.activeJobId && (
        <Typography variant="meta" as="p">
          {t('directory.activeAI', { ns: 'clips', name: named.name })}
        </Typography>
      )}
      {summary.publicationPending && (
        <Typography variant="meta" as="p">
          {t('directory.publicationPending', { ns: 'clips', name: named.name })}
        </Typography>
      )}
      {summary.targetConflict && (
        <Typography variant="meta" as="p">
          {t('directory.conflict', { ns: 'clips', name: named.name })}
        </Typography>
      )}
    </div>
  )
}
