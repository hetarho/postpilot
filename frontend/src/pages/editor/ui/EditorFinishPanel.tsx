import { useTranslation } from 'react-i18next'
import type { PostDraft } from '@/entities/post'
import { BlockType, type PostContent } from '@/shared/api'
import { flushContentQueue } from '@/features/edit-post-content'
import { VoiceLearningPanel, type useVoiceLearning } from '@/features/finalize-post'
import { ExtractMemoriesButton } from '@/features/extract-memories'
import { PublishedUrlField } from '@/features/record-published-url'
import { SentenceFeedback } from '@/features/give-voice-feedback'
import { Notice } from '@/shared/ui'
import { ExportPanel } from '@/widgets/export-panel'
import { EmptyStep } from './EmptyStep'

/** The finalized text 문장 의견 chooses a sentence from.
 *
 *  Every block whose text the reader sees, in reading order, projected by type: TEXT/HEADING/
 *  QUOTE contribute their content and LIST contributes its items, while an image block has no
 *  sentence in it at all. A projection that omitted list items would make the control disappear
 *  on a list-only post and, worse, offer a sentence the server cannot find. */
function sentenceSource(content: PostContent): string {
  return content.blocks
    .flatMap((block) =>
      block.type === BlockType.LIST
        ? block.items
        : block.type === BlockType.IMAGE
          ? []
          : [block.content],
    )
    .filter((value) => value.trim() !== '')
    .join('\n')
}

/** ③'s panel: what the finished post is for — the learning report, the feedback it earns and
 *  the manual per-platform export. */
export function EditorFinishPanel({
  post,
  ownerId,
  result,
  liveContent,
  learning,
  languageMismatch,
  onPhotoUrlsStale,
  onGoGenerate,
  onGoRefine,
}: {
  post: PostDraft
  ownerId: string
  /** The finalized content this step is about; absent until a run has produced one. */
  result?: PostContent
  liveContent?: PostContent
  learning: ReturnType<typeof useVoiceLearning>
  languageMismatch: boolean
  onPhotoUrlsStale: () => void
  onGoGenerate: () => void
  onGoRefine: () => void
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
      {ownerId && <VoiceLearningPanel learning={learning} onBackToRefine={onGoRefine} />}
      {/* Offered exactly where it works: `learning.learned` is "this post's learning run
          completed", which is the precondition the server checks. Feedback is evidence for the
          voice, and a deleted voice cannot take any, nor can a post whose language differs from
          the voice's source — both are server refusals, so the control is not offered rather
          than offered to fail. The text is the FINALIZED text the user is looking at, and the
          flush goes through the slug's content queue because the block editor that owned the
          live ref is mounted on the previous step. */}
      {learning.learned && !post.voice.deleted && !languageMismatch && (
        <SentenceFeedback
          postSlug={post.slug}
          text={sentenceSource(liveContent ?? result)}
          beforeSubmit={() =>
            (flushContentQueue(post.slug) ?? Promise.resolve(0n)).then(() => undefined)
          }
        />
      )}
      {/* Beside 말투 학습, and enabled by the same thing that makes this step exist: canonical
          content (POST-72). It proposes; nothing is stored until the user checks a row. */}
      {ownerId && (
        <ExtractMemoriesButton
          ownerId={ownerId}
          postSlug={post.slug}
          hasContent={Boolean(post.content)}
          className="mt-10"
        />
      )}
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
