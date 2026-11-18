import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { TemplatePreview, useTemplates, type Template } from '@/entities/template'
import { ChannelActions } from '@/widgets/channel-actions'
import { KitScreenHeader } from '@/widgets/writing-kit'
import {
  ResumeConversations,
  TemplateDirectoryEmpty,
  TemplateTile,
  TemplateTileGrid,
  templateWorkState,
} from '@/widgets/template-directory'
import { Button, Notice, LoadingState, pageStyles } from '@/shared/ui'

/** Guide: 템플릿 — the shapes the owner's posts flow in. Each template is shown AS its shape: a
 *  miniature of the post it makes (the same preview its own screen draws, clipped to a page),
 *  under its name and how many posts use it — so the owner finds one by what it looks like, not
 *  by reading a list of names. A tile opens the template, where it is reviewed, changed by
 *  talking, or deleted. A new template is never assembled here: `새 템플릿` floats over the list
 *  as 새 글 floats over 글 (the section's one creation, THEME-67) and opens a conversation that
 *  drafts one from what the owner says they post (TMPL departure: no wizard). A conversation left
 *  unsaved waits at the top to be picked up.
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
      <KitScreenHeader kit="template" title={t('title')} statement={t('page.statement')} />

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

      <ResumeConversations
        heading={t('page.resumeHeading')}
        rows={unsaved.map((row) => ({
          key: row.sessionId,
          label: t('screen.resumeNew', { name: row.displayName || t('page.new') }),
          link: (props) => (
            <Link to="/templates/new" search={{ session: row.sessionId }} {...props} />
          ),
        }))}
      />

      {!isError && !isPending && templates.length === 0 && (
        // The way to make one floats at the bottom; the empty space only says what to bring.
        <TemplateDirectoryEmpty title={t('page.emptyTitle')} body={t('page.emptyBody')} />
      )}

      {!isError && !isPending && templates.length > 0 && (
        <TemplateTileGrid label={t('page.saved')}>
          {templates.map((template) => (
            <PostTemplateTile
              key={template.id}
              template={template}
              summary={summaries.data?.find(
                (row) => row.targetId === template.id || row.lastPublication?.id === template.id,
              )}
            />
          ))}
        </TemplateTileGrid>
      )}
      {/* Docked over the list at every width and set after it, so the last tile scrolls clear. */}
      <ChannelActions section="templates" icon={false} />
    </main>
  )
}

/** One post template on the shared tile: the post it makes in miniature, its name, its
 *  description, how many posts use it and at most one work state. */
function PostTemplateTile({
  template,
  summary,
}: {
  template: Template
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation(['templates', 'authoring'])
  const state = templateWorkState(summary)
  const named = {
    kind: t('kinds.post-template', { ns: 'authoring' }),
    name: summary?.displayName || t('page.new'),
  }
  return (
    <TemplateTile
      link={(props) => (
        <Link to="/templates/$templateId" params={{ templateId: template.id }} {...props} />
      )}
      miniature={
        <div className="bg-card-bg duration-slow ease-emphasized origin-top rounded-t-xl px-4 pt-4 shadow-sm transition-transform group-hover:-translate-y-1">
          <TemplatePreview
            variant="thumbnail"
            titleArea={template.titleArea}
            body={template.body}
          />
        </div>
      }
      name={template.name}
      description={template.description || undefined}
      count={t('postCount', { count: template.postCount })}
      workState={
        state === 'activeAI'
          ? t('screen.activeAI', named)
          : state === 'publicationPending'
            ? t('screen.publicationPending', named)
            : state === 'conflict'
              ? t('screen.conflict', named)
              : state === 'unpublished'
                ? t('unpublished', { ns: 'authoring', ...named })
                : undefined
      }
    />
  )
}
