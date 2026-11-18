import { Plus } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  ClipTemplateDescription,
  ClipTemplateSummary,
  useClipTemplates,
  type ClipTemplate,
} from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { KitScreenHeader } from '@/widgets/writing-kit'
import {
  ResumeConversations,
  TemplateDirectoryEmpty,
  TemplateTile,
  TemplateTileGrid,
  templateWorkState,
} from '@/widgets/template-directory'
import { videoTemplateIllustration } from '@/shared/assets'
import { Button, Notice, LoadingState, buttonStyles, pageStyles } from '@/shared/ui'

/** Guide: 영상 템플릿 is 템플릿's directory (THEME-76, CLIP-42) — the same resume rows and the same
 *  tiles, bound to video templates. Unsaved conversations lead; then each saved template is a tile
 *  leading with its 9:16 composition example, then its name, its stages in order, how many videos
 *  use it and at most one work state. A tile opens the template's reading, where it is changed or
 *  deleted; no tile carries a delete. The section's one creation, `+ 새 템플릿`, stands in the
 *  header's action row (THEME-67).
 *
 *  Nothing here calls a model, renders or enqueues a job. */
export function VideoTemplatesPage() {
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isFetching, isError, refetch } = useClipTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'video-template' })
  const unsaved = summaries.data?.filter((row) => !row.targetId && !row.lastPublication) ?? []

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}>
      {/* The same opening as its siblings under 영상 — 영상 지침 and 목소리 — with the one way to
          make a template in the heading row. */}
      <KitScreenHeader
        size="section"
        title={t('directory.title', { ns: 'clips' })}
        statement={t('directory.statement', { ns: 'clips' })}
        illustration={videoTemplateIllustration}
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
          className="mt-8 sm:[&>.loading-reveal]:grid-cols-2 lg:[&>.loading-reveal]:grid-cols-3"
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

      <ResumeConversations
        heading={t('directory.resumeHeading', { ns: 'clips' })}
        rows={unsaved.map((row) => ({
          key: row.sessionId,
          label: t('directory.resumeNew', {
            ns: 'clips',
            name: row.displayName || t('directory.create', { ns: 'clips' }),
          }),
          link: (props) => (
            <Link to="/video-templates/new" search={{ session: row.sessionId }} {...props} />
          ),
        }))}
      />

      {!isError && !isPending && templates.length === 0 && (
        <TemplateDirectoryEmpty
          title={t('directory.emptyTitle', { ns: 'clips' })}
          body={t('directory.emptyBody', { ns: 'clips' })}
        />
      )}

      {!isError && !isPending && templates.length > 0 && (
        <TemplateTileGrid label={t('directory.saved', { ns: 'clips' })}>
          {templates.map((template) => (
            <VideoTemplateTile
              key={template.id}
              template={template}
              summary={summaries.data?.find(
                (row) => row.targetId === template.id || row.lastPublication?.id === template.id,
              )}
            />
          ))}
        </TemplateTileGrid>
      )}
    </main>
  )
}

/** One video template on the shared tile: its static 9:16 example (CLIP-236), its name, the
 *  stages it runs through, how many videos use it and at most one work state. */
function VideoTemplateTile({
  template,
  summary,
}: {
  template: ClipTemplate
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation(['clips', 'authoring'])
  const state = templateWorkState(summary)
  const name = summary?.displayName || template.name
  return (
    <TemplateTile
      link={(props) => (
        <Link to="/video-templates/$templateId" params={{ templateId: template.id }} {...props} />
      )}
      miniature={
        <div className="duration-slow ease-emphasized flex origin-top justify-center transition-transform group-hover:-translate-y-1">
          <ClipTemplateSummary template={template} variant="miniature" />
        </div>
      }
      name={template.name}
      description={<ClipTemplateDescription template={template} />}
      count={t('directory.projectCount', { ns: 'clips', count: template.projectCount })}
      workState={
        state === 'activeAI'
          ? t('directory.activeAI', { ns: 'clips', name })
          : state === 'publicationPending'
            ? t('directory.publicationPending', { ns: 'clips', name })
            : state === 'conflict'
              ? t('directory.conflict', { ns: 'clips', name })
              : state === 'unpublished'
                ? t('unpublished', {
                    ns: 'authoring',
                    kind: t('kinds.video-template', { ns: 'authoring' }),
                    name,
                  })
                : undefined
      }
    />
  )
}
