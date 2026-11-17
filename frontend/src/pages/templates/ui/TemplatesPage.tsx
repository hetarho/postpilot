import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { clsx } from 'clsx'
import { CircleHelp, Image, MessagesSquare, PenLine, Repeat } from 'lucide-react'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { templateOutline, useTemplates, type OutlineRow, type Template } from '@/entities/template'
import { ChannelActions } from '@/widgets/channel-actions'
import { KitHeader } from '@/widgets/writing-kit'
import {
  Badge,
  Button,
  Notice,
  LoadingState,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** How many outline rows a card draws before it says how many more there are. */
const OUTLINE_ROWS = 6

/** Guide: 템플릿 — the shapes the owner's posts take. Each template is drawn as the outline of the
 *  post it makes (its headings in its own words, the photo positions, what the AI writes and what
 *  the writer is asked), so the list is read as shapes, not as names. A card is the way into the
 *  template, where it is changed by talking and deleted; a conversation not yet saved waits at the
 *  top as one to continue. The one action is a new template, which opens the conversation.
 *
 *  Nothing on this screen calls a model or enqueues a job ([I5]). */
export function TemplatesPage() {
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const unsaved = summaries.data?.filter((row) => !row.targetId && !row.lastPublication) ?? []

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}>
      <KitHeader part="template" />

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
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('screen.workLoadFailed', { ns: 'templates' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {unsaved.length > 0 && (
            <section className="mt-8" aria-labelledby="templates-unsaved-heading">
              <Typography variant="title" as="h2" id="templates-unsaved-heading">
                {t('shape.talking', { ns: 'templates' })}
              </Typography>
              <ul className="mt-3 grid gap-3 lg:grid-cols-2 lg:gap-4">
                {unsaved.map((row) => (
                  <UnsavedConversation key={row.sessionId} summary={row} />
                ))}
              </ul>
            </section>
          )}
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-label={t('page.saved', { ns: 'templates' })} className="mt-8">
              <h2 className="flex items-baseline gap-2">
                <span className={typographyStyles({ variant: 'title' })}>
                  {t('page.saved', { ns: 'templates' })}
                </span>
                <span className={typographyStyles({ variant: 'label' })}>{templates.length}</span>
              </h2>
              <ul className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {templates.map((template, index) => (
                  <TemplateShape
                    key={template.id}
                    template={template}
                    index={index}
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

/** No template yet: the empty sheet a first conversation will fill, and nothing else. */
function EmptyState() {
  const { t } = useTranslation('templates')
  return (
    <section
      aria-labelledby="templates-empty-heading"
      className="border-content-tertiary/40 mt-8 flex flex-col items-center gap-3 rounded-2xl border-2 border-dashed px-6 py-10 text-center"
    >
      <span
        aria-hidden="true"
        className="bg-badge-accent-bg text-badge-accent-fg flex size-14 items-center justify-center rounded-full"
      >
        <MessagesSquare className="size-7" />
      </span>
      <Typography variant="title" as="h2" id="templates-empty-heading">
        {t('page.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure text-balance">
        {t('page.emptyTalk')}
      </Typography>
    </section>
  )
}

/** One template, drawn as the outline of the post it makes. The whole card is the link. */
function TemplateShape({
  template,
  index,
  summary,
}: {
  template: Template
  index: number
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation(['templates', 'authoring'])
  const outline = templateOutline(template.body)
  const shown = outline.slice(0, OUTLINE_ROWS)
  const nameId = `template-${template.id}-name`
  const named = { kind: t('kinds.post-template', { ns: 'authoring' }), name: template.name }
  // What is waiting on this template's conversation, in the order it matters.
  const work = summary?.targetConflict
    ? t('screen.conflict', { ns: 'templates', name: template.name })
    : summary?.activeJobId
      ? t('screen.activeAI', { ns: 'templates', name: template.name })
      : summary?.publicationPending
        ? t('screen.publicationPending', { ns: 'templates', name: template.name })
        : summary?.hasUnpublishedChanges
          ? t('unpublished', { ns: 'authoring', ...named })
          : ''
  return (
    <li className="animate-rise" style={{ animationDelay: `${index * 50}ms` }}>
      <Link
        to="/templates/$templateId"
        params={{ templateId: template.id }}
        aria-labelledby={nameId}
        className={cardStyles({
          interactive: true,
          className: 'flex h-full min-w-0 flex-col gap-3',
        })}
      >
        <span className="flex min-w-0 flex-col gap-1">
          <span
            id={nameId}
            className={typographyStyles({
              variant: 'fieldTitle',
              className: 'text-content-primary line-clamp-2 break-words',
            })}
          >
            {template.name}
          </span>
          {template.description && (
            <span className={typographyStyles({ variant: 'meta', className: 'line-clamp-2' })}>
              {template.description}
            </span>
          )}
        </span>
        {shown.length > 0 && (
          <ol
            aria-label={t('shape.outline', { ns: 'templates', name: template.name })}
            className="bg-card-well border-stroke-accent flex flex-col gap-1.5 rounded-lg border-l-2 px-3 py-3"
          >
            {shown.map((row, rowIndex) => (
              <OutlineLine key={rowIndex} row={row} />
            ))}
            {outline.length > OUTLINE_ROWS && (
              <li className={typographyStyles({ variant: 'meta' })}>
                {t('shape.more', { ns: 'templates', count: outline.length - OUTLINE_ROWS })}
              </li>
            )}
          </ol>
        )}
        <span className="mt-auto flex flex-wrap items-center gap-2">
          <Badge tone="neutral">
            {t('postCount', { ns: 'templates', count: template.postCount })}
          </Badge>
          {work && (
            <span
              role="status"
              className={typographyStyles({ variant: 'meta', className: 'text-link-fg' })}
            >
              {work}
            </span>
          )}
        </span>
      </Link>
    </li>
  )
}

function OutlineLine({ row }: { row: OutlineRow }) {
  const { t } = useTranslation('templates')
  const Icon =
    row.kind === 'photo'
      ? Image
      : row.kind === 'write'
        ? PenLine
        : row.kind === 'ask'
          ? CircleHelp
          : row.kind === 'repeat'
            ? Repeat
            : null
  const text =
    row.kind === 'heading'
      ? row.text
      : row.kind === 'photo'
        ? t('shape.photo', { count: row.count })
        : row.kind === 'write'
          ? t('shape.write')
          : row.kind === 'ask'
            ? t('shape.ask', { label: row.text })
            : t('shape.repeat', { each: row.text })
  return (
    <li className="flex min-w-0 items-center gap-2">
      {Icon ? (
        <Icon aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
      ) : (
        <span aria-hidden="true" className="bg-meter-fill size-1.5 shrink-0 rounded-full" />
      )}
      <span
        className={typographyStyles({
          variant: row.kind === 'heading' ? 'label' : 'meta',
          className: clsx('min-w-0 truncate', row.kind === 'heading' && 'text-content-primary'),
        })}
      >
        {text}
      </span>
    </li>
  )
}

/** A conversation that has not been saved yet: where it stands, and the way back into it. */
function UnsavedConversation({ summary }: { summary: AuthoringSummary }) {
  const { t } = useTranslation(['templates', 'authoring'])
  const name = summary.displayName || t('page.new', { ns: 'templates' })
  return (
    <li>
      <Link
        to="/templates/new"
        search={{ session: summary.sessionId }}
        className={cardStyles({
          interactive: true,
          className: 'flex min-w-0 items-center gap-3',
        })}
      >
        <span
          aria-hidden="true"
          className="bg-badge-accent-bg text-badge-accent-fg flex size-11 shrink-0 items-center justify-center rounded-full"
        >
          <MessagesSquare className="size-5" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col">
          <span
            className={typographyStyles({
              variant: 'fieldTitle',
              className: 'text-content-primary truncate',
            })}
          >
            {t('screen.resumeNew', { ns: 'templates', name })}
          </span>
          <span className={typographyStyles({ variant: 'meta' })}>
            {summary.activeJobId
              ? t('screen.activeAI', { ns: 'templates', name })
              : t('screen.unsaved', { ns: 'templates' })}
          </span>
        </span>
      </Link>
    </li>
  )
}
