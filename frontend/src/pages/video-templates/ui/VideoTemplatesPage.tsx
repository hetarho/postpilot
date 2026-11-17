import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ClipTemplateSummary, useClipTemplates, type ClipTemplate } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { ChannelActions } from '@/widgets/channel-actions'
import {
  KitArt,
  KitEmpty,
  KitHeader,
  TemplateCard,
  UnsavedTemplateCard,
} from '@/widgets/writing-kit'
import { Badge, Button, Notice, LoadingState, Typography, pageStyles } from '@/shared/ui'

/** Guide: 영상 템플릿 — the shapes the owner's videos take, the 영상 side of 템플릿. Each template is
 *  drawn as the video it makes (its opening and closing design around one example line, and its
 *  stages in order) on the same card 글 템플릿 uses; the card is the way in, where it is changed by
 *  talking and deleted. A conversation not yet saved waits at the top to continue. The one action
 *  is a new template, which opens the conversation. Nothing here calls a model. */
export function VideoTemplatesPage() {
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isFetching, isError, refetch } = useClipTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'video-template' })
  const unsaved = summaries.data?.filter((row) => !row.targetId && !row.lastPublication) ?? []

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}>
      <KitHeader part="template" video />

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
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('directory.workLoadFailed', { ns: 'clips' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {unsaved.length > 0 && (
            <section className="mt-8" aria-labelledby="video-templates-unsaved-heading">
              <Typography variant="title" as="h2" id="video-templates-unsaved-heading">
                {t('directory.unsavedWork', { ns: 'clips' })}
              </Typography>
              <ul className="mt-3 grid gap-3 lg:grid-cols-2 lg:gap-4">
                {unsaved.map((row) => {
                  const name = row.displayName || t('directory.create', { ns: 'clips' })
                  return (
                    <UnsavedTemplateCard
                      key={row.sessionId}
                      title={t('directory.resumeNew', { ns: 'clips', name })}
                      status={
                        row.activeJobId
                          ? t('directory.activeAI', { ns: 'clips', name })
                          : t('directory.unsaved', { ns: 'clips' })
                      }
                      renderLink={(link) => (
                        <Link
                          to="/video-templates/new"
                          search={{ session: row.sessionId }}
                          {...link}
                        />
                      )}
                    />
                  )
                })}
              </ul>
            </section>
          )}
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="video-templates-heading" className="mt-8">
              <Typography variant="title" as="h2" id="video-templates-heading">
                {t('directory.saved', { ns: 'clips' })}
              </Typography>
              <ul className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {templates.map((template, index) => (
                  <VideoTemplateShape
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
      <ChannelActions video section="templates" />
    </main>
  )
}

/** No video template yet: the empty slot a first conversation will fill. It creates nothing —
 *  there are no shipped presets. */
function EmptyState() {
  const { t } = useTranslation('clips')
  return (
    <KitEmpty className="mt-8" figure={<KitArt part="template" />} title={t('directory.empty')}>
      {t('directory.emptyTalk')}
    </KitEmpty>
  )
}

/** One video template, drawn as the video it makes. */
function VideoTemplateShape({
  template,
  index,
  summary,
}: {
  template: ClipTemplate
  index: number
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation(['clips', 'authoring'])
  const named = { kind: t('kinds.video-template', { ns: 'authoring' }), name: template.name }
  // What is waiting on this template's conversation, in the order it matters.
  const work = summary?.targetConflict
    ? t('directory.conflict', { ns: 'clips', name: template.name })
    : summary?.activeJobId
      ? t('directory.activeAI', { ns: 'clips', name: template.name })
      : summary?.publicationPending
        ? t('directory.publicationPending', { ns: 'clips', name: template.name })
        : summary?.hasUnpublishedChanges
          ? t('unpublished', { ns: 'authoring', ...named })
          : ''
  return (
    <TemplateCard
      index={index}
      name={template.name}
      outline={<ClipTemplateSummary template={template} />}
      meta={
        <Badge tone="neutral">
          {t('directory.projectCount', { ns: 'clips', count: template.projectCount })}
        </Badge>
      }
      status={work || undefined}
      renderLink={(link) => (
        <Link to="/video-templates/$templateId" params={{ templateId: template.id }} {...link} />
      )}
    />
  )
}
