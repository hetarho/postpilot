import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { PartyPopper } from 'lucide-react'
import type { PostDraft } from '@/entities/post'
import { isEmptyGuide, type AdvertiserGuide } from '@/entities/advertiser-guide'
import type { PostContent } from '@/shared/api'
import { AdvertiserChecklist } from '@/features/check-advertiser-guide'
import { ExtractMemoriesButton } from '@/features/extract-memories'
import { PublishedUrlField } from '@/features/record-published-url'
import { Notice, Typography, buttonStyles, cardStyles } from '@/shared/ui'
import { ExportPanel } from '@/widgets/export-panel'
import { EmptyStep } from './EmptyStep'
import { CreateClipFromPostButton } from './CreateClipFromPostButton'

/** The 체험단 parts ③ reads: the switch, the guide and the owner's ticks, with how a tick saves. */
export interface FinishCampaign {
  on: boolean
  guide: AdvertiserGuide
  ticks: readonly string[]
  readOnly: boolean
  onTicksChange: (ticks: string[]) => void
}

/** ③'s panel — Guide: a 완성 screen about one thing, getting the post out. It opens on a short
 *  celebration, then the 체험단 checklist when there is one (what to check before the copy goes
 *  to the campaign, POST-119), then the per-platform copy and export as the screen's main body.
 *  The things a finished post can ALSO become — 기억, a template, a clip — wait in one card under
 *  it, and the address it was published at closes the screen (POST-54). */
export function EditorFinishPanel({
  post,
  ownerId,
  result,
  liveContent,
  campaign,
  clipMaterial,
  beforeCreateClip,
  onPhotoUrlsStale,
  onGoGenerate,
}: {
  post: PostDraft
  ownerId: string
  /** Absent or off, or with an empty guide, shows no checklist. */
  campaign?: FinishCampaign
  clipMaterial?: Pick<PostDraft, 'title' | 'memo'>
  beforeCreateClip?: () => Promise<void>
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
      <div className="mt-8 flex items-center gap-4">
        <span
          aria-hidden="true"
          className="bg-badge-accent-bg text-badge-accent-fg flex size-14 shrink-0 items-center justify-center rounded-full"
        >
          <PartyPopper className="size-7" />
        </span>
        <Typography variant="stepTitle">{t('guide.finishTitle')}</Typography>
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
          contentLanguage={post.contentLanguage}
          // The copied body reserves the disclosure's place while 체험단 글 is on (EXPORT-28).
          disclosure={Boolean(campaign?.on)}
          onPhotoUrlsStale={onPhotoUrlsStale}
        />
      ) : (
        <Notice tone="danger" role="alert" className="mt-10">
          {t('export.languageMissing')}
        </Notice>
      )}
      {/* What else the finished post can be turned into (POST-72, POST-103): one card, three
          quiet actions. The template link starts nothing: the owner sends the request themselves. */}
      <section aria-labelledby="finish-more" className={cardStyles({ className: 'mt-10' })}>
        <Typography variant="fieldTitle" as="h2" id="finish-more">
          {t('guide.more')}
        </Typography>
        <div className="mt-4 flex flex-wrap items-start gap-2">
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
          {post.content && ownerId && (
            <CreateClipFromPostButton
              key={`${ownerId}-${post.slug}`}
              ownerId={ownerId}
              post={{
                ...post,
                ...clipMaterial,
                campaign: campaign?.on ?? post.campaign,
                advertiserGuide: campaign?.guide ?? post.advertiserGuide,
              }}
              beforeCreate={beforeCreateClip}
            />
          )}
        </div>
      </section>
      {/* The panel's foot (POST-54): what the finished post ends with is where it was published. */}
      <PublishedUrlField post={post} className="mt-10" />
    </>
  )
}
