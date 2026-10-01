import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import type { PostDraft } from '@/entities/post'
import type { PostContent } from '@/shared/api'
import { ExtractMemoriesButton } from '@/features/extract-memories'
import { PublishedUrlField } from '@/features/record-published-url'
import { Notice, buttonStyles } from '@/shared/ui'
import { ExportPanel } from '@/widgets/export-panel'
import { EmptyStep } from './EmptyStep'

/** ③'s panel: what the finished post is for — 기억으로 저장, the manual per-platform export and
 *  the address it was published at (POST-54). */
export function EditorFinishPanel({
  post,
  ownerId,
  result,
  liveContent,
  onPhotoUrlsStale,
  onGoGenerate,
}: {
  post: PostDraft
  ownerId: string
  /** The finalized content this step is about; absent until a run has produced one. */
  result?: PostContent
  liveContent?: PostContent
  onPhotoUrlsStale: () => void
  onGoGenerate: () => void
}) {
  const { t } = useTranslation('posts')
  if (!result)
    return (
      <EmptyStep goTo={onGoGenerate} goToLabel={t('editor.goGenerate')}>
        {t('editor.finishEmpty')}
      </EmptyStep>
    )
  return (
    <>
      {/* Enabled by the same thing that makes this step exist: canonical content (POST-72). It
          proposes; nothing is stored until the user checks a row. */}
      {/* The two things a finished post can be turned into for later, side by side (POST-72,
          POST-103). The template link starts nothing: the owner sends the request themselves. */}
      <div className="mt-10 flex flex-wrap items-start gap-2">
        {ownerId && (
          <ExtractMemoriesButton
            ownerId={ownerId}
            postSlug={post.slug}
            hasContent={Boolean(post.content)}
          />
        )}
        {post.content && (
          <Link
            to="/templates/new"
            search={{ from: post.slug }}
            className={buttonStyles({ variant: 'secondary' })}
          >
            {t('editor.templateFromPost')}
          </Link>
        )}
      </div>
      {post.contentLanguage ? (
        <ExportPanel
          videos={post.videos}
          content={liveContent ?? result}
          images={post.images}
          createdAt={post.createdAt}
          contentLanguage={post.contentLanguage}
          onPhotoUrlsStale={onPhotoUrlsStale}
        />
      ) : (
        <Notice tone="danger" role="alert" className="mt-10">
          {t('export.languageMissing')}
        </Notice>
      )}
      {/* The panel's foot (POST-54): what the finished post ends with is where it was published. */}
      <PublishedUrlField post={post} className="mt-10" />
    </>
  )
}
