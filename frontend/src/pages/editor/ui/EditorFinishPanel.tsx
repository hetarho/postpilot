import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import type { PostDraft } from '@/entities/post'
import { isEmptyGuide, type AdvertiserGuide } from '@/entities/advertiser-guide'
import type { PostContent } from '@/shared/api'
import { AdvertiserChecklist } from '@/features/check-advertiser-guide'
import { ExtractMemoriesButton } from '@/features/extract-memories'
import { PublishedUrlField } from '@/features/record-published-url'
import { Notice, buttonStyles } from '@/shared/ui'
import { ExportPanel } from '@/widgets/export-panel'
import { EmptyStep } from './EmptyStep'

/** The 체험단 parts ③ reads: the switch, the guide and the owner's ticks, with how a tick saves. */
export interface FinishCampaign {
  on: boolean
  guide: AdvertiserGuide
  ticks: readonly string[]
  readOnly: boolean
  onTicksChange: (ticks: string[]) => void
}

/** ③'s panel: what the finished post is for — 기억으로 저장, the 체험단 checklist, the manual
 *  per-platform export and the address it was published at (POST-54, POST-119). */
export function EditorFinishPanel({
  post,
  ownerId,
  result,
  liveContent,
  campaign,
  onPhotoUrlsStale,
  onGoGenerate,
}: {
  post: PostDraft
  ownerId: string
  /** Absent or off, or with an empty guide, shows no checklist. */
  campaign?: FinishCampaign
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
      {/* Above the export: what to check before the copy goes to the campaign (POST-119). */}
      {campaign?.on && !isEmptyGuide(campaign.guide) && (
        <AdvertiserChecklist
          content={liveContent ?? result}
          guide={campaign.guide}
          ticks={campaign.ticks}
          readOnly={campaign.readOnly}
          onTicksChange={campaign.onTicksChange}
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
