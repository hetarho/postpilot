import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { MessagesSquare, Sparkles } from 'lucide-react'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { KitScreenHeader } from '@/widgets/writing-kit'
import {
  Badge,
  Button,
  Notice,
  LoadingState,
  Typography,
  buttonStyles,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** Guide: 템플릿 — the shapes the owner's posts flow in. Each template is shown AS its shape: a
 *  miniature of the post it makes (the same preview its own screen draws, clipped to a page),
 *  under its name and how many posts use it — so the owner finds one by what it looks like, not
 *  by reading a list of names. A tile opens the template, where it is reviewed, changed by
 *  talking, or deleted. A new template is never assembled here: `AI와 새 템플릿` opens a
 *  conversation that drafts one from what the owner says they post (TMPL departure: no wizard).
 *  A conversation left unsaved waits at the top to be picked up.
 *
 *  Nothing here calls a model or enqueues a job ([I5]). */
export function TemplatesPage() {
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const unsaved = summaries.data?.filter((row) => !row.targetId && !row.lastPublication) ?? []

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-4 sm:pt-8' })}>
      <KitScreenHeader
        kit="template"
        title={t('title')}
        statement={t('page.statement')}
        actions={
          <Link
            to="/templates/new"
            className={buttonStyles({ variant: 'cta', className: 'gap-2' })}
          >
            <Sparkles aria-hidden="true" className="size-5 shrink-0" />
            {t('page.create')}
          </Link>
        }
      />

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed')}</span>
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
          className="mt-8 sm:[&>.loading-reveal]:grid-cols-2 lg:[&>.loading-reveal]:grid-cols-3"
        />
      )}
      {summaries.isError && (
        <Notice tone="danger" role="alert" className="mt-6">
          {t('screen.workLoadFailed')}
          <Button variant="ghost" onClick={() => void summaries.refetch()}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}

      {unsaved.length > 0 && (
        <section aria-labelledby="templates-unsaved" className="mt-8">
          <Typography variant="eyebrow" as="h2" id="templates-unsaved">
            {t('page.resumeHeading')}
          </Typography>
          <ul className="mt-2 flex flex-col gap-2">
            {unsaved.map((row) => (
              <li key={row.sessionId}>
                <Link
                  to="/templates/new"
                  search={{ session: row.sessionId }}
                  className="bg-badge-accent-bg hover:bg-card-bg-hover active:bg-card-bg-active flex min-h-14 items-center gap-3 rounded-2xl px-4 py-2"
                >
                  <MessagesSquare
                    aria-hidden="true"
                    className="text-badge-accent-fg size-5 shrink-0"
                  />
                  <span
                    className={typographyStyles({
                      variant: 'label',
                      className: 'text-content-primary min-w-0 flex-1 truncate',
                    })}
                  >
                    {t('screen.resumeNew', {
                      name: row.displayName || t('page.new'),
                    })}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {!isError && !isPending && templates.length === 0 && (
        <section
          aria-labelledby="templates-empty"
          className="border-divider mt-8 flex flex-col items-start gap-3 rounded-3xl border border-dashed px-6 py-8"
        >
          <Typography variant="title" as="h2" id="templates-empty">
            {t('page.emptyTitle')}
          </Typography>
          {/* The way to make one is the heading row's; the empty space only says what to bring. */}
          <Typography variant="body" as="p" className="text-content-secondary max-w-measure">
            {t('page.emptyBody')}
          </Typography>
        </section>
      )}

      {!isError && !isPending && templates.length > 0 && (
        <section aria-labelledby="templates-saved" className="mt-8">
          <h2 id="templates-saved" className="sr-only">
            {t('page.saved')}
          </h2>
          <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {templates.map((template, index) => (
              <TemplateTile
                key={template.id}
                template={template}
                index={index}
                summary={summaries.data?.find(
                  (row) => row.targetId === template.id || row.lastPublication?.id === template.id,
                )}
              />
            ))}
          </ul>
        </section>
      )}
    </main>
  )
}

/** One template as a page: its shape in miniature, faded where the tile ends, then its name, its
 *  description and how many posts use it. The whole tile is the one link. */
function TemplateTile({
  template,
  index,
  summary,
}: {
  template: Template
  index: number
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation('templates')
  return (
    <li className="animate-rise" style={{ animationDelay: `${index * 50}ms` }}>
      <Link
        to="/templates/$templateId"
        params={{ templateId: template.id }}
        className={cardStyles({
          interactive: true,
          padding: 'none',
          className: 'group flex h-full flex-col overflow-hidden',
        })}
      >
        <div className="bg-card-well relative h-52 overflow-hidden px-5 pt-5">
          <div className="bg-card-bg duration-slow ease-emphasized origin-top rounded-t-xl px-4 pt-4 shadow-sm transition-transform group-hover:-translate-y-1">
            <TemplatePreview
              variant="thumbnail"
              titleArea={template.titleArea}
              body={template.body}
            />
          </div>
          <div
            aria-hidden="true"
            className="from-card-well pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-linear-to-t to-transparent"
          />
        </div>
        <div className="flex flex-1 flex-col gap-1 p-5">
          <span
            className={typographyStyles({
              variant: 'fieldTitle',
              className: 'text-content-primary line-clamp-2 break-words',
            })}
          >
            {template.name}
          </span>
          {template.description && (
            <span
              className={typographyStyles({
                variant: 'body',
                className: 'text-content-secondary line-clamp-2',
              })}
            >
              {template.description}
            </span>
          )}
          <span className="mt-auto flex flex-wrap items-center gap-2 pt-2">
            <Badge tone="neutral">{t('postCount', { count: template.postCount })}</Badge>
            <WorkState summary={summary} />
          </span>
        </div>
      </Link>
    </li>
  )
}

/** A conversation still running, waiting to be saved or holding unsaved changes: one quiet word on
 *  the tile, the detail on the template's own screen. */
function WorkState({ summary }: { summary?: AuthoringSummary }) {
  const { t } = useTranslation(['templates', 'authoring'])
  if (!summary) return null
  const named = {
    kind: t('kinds.post-template', { ns: 'authoring' }),
    name: summary.displayName || t('page.new'),
  }
  const text = summary.activeJobId
    ? t('screen.activeAI', named)
    : summary.publicationPending
      ? t('screen.publicationPending', named)
      : summary.targetConflict
        ? t('screen.conflict', named)
        : summary.hasUnpublishedChanges
          ? t('unpublished', { ns: 'authoring', ...named })
          : ''
  if (!text) return null
  return (
    <span role="status" className={typographyStyles({ variant: 'meta', className: 'min-w-0' })}>
      {text}
    </span>
  )
}
