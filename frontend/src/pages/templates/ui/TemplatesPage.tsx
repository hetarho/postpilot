import { useTranslation } from 'react-i18next'
import { useRef, type Ref } from 'react'
import { Link } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { DeleteTemplateButton } from '@/features/delete-template'
import { ChannelActions } from '@/widgets/channel-actions'
import { MessagesSquare } from 'lucide-react'
import { templateDraftIllustration } from '@/shared/assets'
import {
  Badge,
  Button,
  Notice,
  LoadingState,
  Typography,
  pageStyles,
  typographyStyles,
  useListRemovalFocus,
} from '@/shared/ui'

/** Guide: 템플릿 — what a template IS: the shape a post takes. Each saved template is shown as the
 *  post it makes (the same preview the template's own screen draws), under its name, so the owner
 *  finds one by its shape rather than by a label. A new one is made by talking to the AI (새 템플릿);
 *  a conversation not yet saved waits under 이어서 대화하기 (TMPL-24).
 *
 *  Editing lives on the template's own screen, so this list carries nothing but the list (change
 *  30). Nothing here calls a model or enqueues a job: a template is authored text, and reading or
 *  deleting one is a plain CRUD round trip ([I5]). */
export function TemplatesPage() {
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const page = useRef<HTMLElement>(null)
  const deleteButtons = useRef(new Map<string, HTMLButtonElement>())
  const requestRemoval = useListRemovalFocus({
    keys: templates.map((template) => template.id),
    getTarget: (id) => deleteButtons.current.get(id),
    getFallback: () => page.current?.querySelector<HTMLAnchorElement>('a[href="/templates/new"]'),
  })

  return (
    <main
      ref={page}
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}
    >
      {/* Guide: the hub's title, the 템플릿 chip and the kit's head (what a template does to a post)
          say where this is; the page is the templates themselves. */}
      <h1 className="sr-only">{t('title', { ns: 'templates' })}</h1>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed', { ns: 'templates' })}</span>
          <Button
            variant="ghost"
            onClick={refetch}
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
              label={t('screen.checkingWork', { ns: 'templates' })}
              className="mt-6"
            />
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('screen.workLoadFailed', { ns: 'templates' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {/* Work still in progress leads, as 작성 중 leads 글: a conversation not yet saved is
              picked up here before the saved shapes. */}
          {summaries.data?.some((row) => !row.targetId && !row.lastPublication) && (
            <section className="mt-6" aria-labelledby="templates-unsaved-heading">
              <Typography variant="title" as="h2" id="templates-unsaved-heading">
                {t('screen.unsavedWork', { ns: 'templates' })}
              </Typography>
              <ul className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
                {summaries.data
                  .filter((row) => !row.targetId && !row.lastPublication)
                  .map((row) => (
                    <li
                      key={row.sessionId}
                      className="bg-card-bg hover:bg-card-bg-hover relative flex min-h-16 items-center gap-3 rounded-2xl px-4 py-3 shadow-sm"
                    >
                      <span
                        aria-hidden="true"
                        className="bg-kit-template/15 text-kit-template flex size-10 shrink-0 items-center justify-center rounded-xl"
                      >
                        <MessagesSquare className="size-5" />
                      </span>
                      <span className="flex min-w-0 flex-1 flex-col">
                        <Link
                          to="/templates/new"
                          search={{ session: row.sessionId }}
                          className={typographyStyles({
                            variant: 'fieldTitle',
                            className: 'text-content-primary truncate after:absolute after:inset-0',
                          })}
                        >
                          {t('screen.resumeNew', {
                            ns: 'templates',
                            name: row.displayName || t('page.new', { ns: 'templates' }),
                          })}
                        </Link>
                        <EditingStatus summary={row} />
                      </span>
                    </li>
                  ))}
              </ul>
            </section>
          )}
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section
              aria-labelledby="templates-heading"
              className={
                summaries.data?.some((row) => !row.targetId && !row.lastPublication)
                  ? 'mt-8'
                  : 'mt-6'
              }
            >
              <Typography variant="title" id="templates-heading">
                {t('page.saved', { ns: 'templates' })}
              </Typography>
              {/* Guide: each template is the shape of the post it makes; the card opens its
                  conversation. */}
              <ul className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
        </>
      )}
      {/* The section's creation button is the page's one dock (THEME-67). */}
      <ChannelActions section="templates" />
    </main>
  )
}

/** Plain language and no example. The grammar is what the write prompt reads, not something the
 *  user authors in — so the empty state says what a template is FOR and lets the composition
 *  editor teach its own vocabulary (TMPL-24). It creates nothing: there are no shipped presets. */
function EmptyState() {
  const { t } = useTranslation('templates')
  return (
    <section
      aria-labelledby="templates-empty-heading"
      className="mt-6 flex flex-col items-center gap-2 text-center sm:flex-row sm:text-left"
    >
      <img
        src={templateDraftIllustration}
        alt=""
        width={440}
        height={440}
        className="size-44 shrink-0 object-contain"
        draggable={false}
      />
      <div>
        <Typography variant="title" id="templates-empty-heading">
          {t('page.empty')}
        </Typography>
        <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
          {t('page.emptyTalk')}
        </Typography>
      </div>
    </section>
  )
}

/** One template, one target. The link stretches over the whole row through its `::after`, so the
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
  template: Template
  summary?: AuthoringSummary
  deleteButtonRef: Ref<HTMLButtonElement>
  onDeleted: () => void
}) {
  const { t } = useTranslation('templates')
  return (
    <li className="group bg-surface-recessed hover:bg-card-bg-hover relative flex min-w-0 flex-col overflow-hidden rounded-3xl transition-colors">
      <div className="flex items-start gap-2 px-5 pt-4">
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <Link
            to="/templates/$templateId"
            params={{ templateId: template.id }}
            className={typographyStyles({
              variant: 'fieldTitle',
              className:
                'text-content-primary line-clamp-2 break-words after:absolute after:inset-0',
            })}
          >
            {template.name}
          </Link>
          {template.description && (
            <Typography variant="meta" as="span" className="line-clamp-2">
              {template.description}
            </Typography>
          )}
        </span>
        <span className="relative z-10 -mt-1 -mr-2 shrink-0">
          <DeleteTemplateButton
            quiet
            ownerId={ownerId}
            template={template}
            buttonRef={deleteButtonRef}
            onDeleted={onDeleted}
          />
        </span>
      </div>
      {/* The post this template makes, as its own screen draws it, cut to the card. A picture of
          the shape for the eye; the template's screen reads it in full. */}
      <div
        aria-hidden="true"
        inert
        className="pointer-events-none relative mt-3 max-h-64 [zoom:0.86] overflow-hidden [mask-image:linear-gradient(to_bottom,black_65%,transparent)] px-4"
      >
        <TemplatePreview bare titleArea={template.titleArea} body={template.body} />
      </div>
      <div className="relative mt-auto flex flex-wrap items-center gap-2 px-5 pt-1 pb-4">
        <Badge tone="neutral">{t('postCount', { count: template.postCount })}</Badge>
        {summary && <EditingStatus summary={summary} />}
      </div>
    </li>
  )
}

function EditingStatus({ summary }: { summary?: AuthoringSummary }) {
  const { t } = useTranslation(['templates', 'authoring'])
  if (!summary) return null
  const named = {
    kind: t('kinds.post-template', { ns: 'authoring' }),
    name: summary.displayName || t('page.new', { ns: 'templates' }),
  }
  return (
    <div className="mt-1 space-y-1" role="status">
      {summary.lastPublication?.outcome && (
        <Typography variant="meta" as="p">
          {t('screen.lastPublication', {
            ns: 'templates',
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
          {t('screen.activeAI', { ns: 'templates', name: named.name })}
        </Typography>
      )}
      {summary.publicationPending && (
        <Typography variant="meta" as="p">
          {t('screen.publicationPending', { ns: 'templates', name: named.name })}
        </Typography>
      )}
      {summary.targetConflict && (
        <Typography variant="meta" as="p">
          {t('screen.conflict', { ns: 'templates', name: named.name })}
        </Typography>
      )}
    </div>
  )
}
