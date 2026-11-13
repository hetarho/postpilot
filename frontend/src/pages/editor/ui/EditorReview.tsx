import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useGuidelines } from '@/entities/guideline'
import { Thumbnail } from '@/entities/image'
import type { PostDraft } from '@/entities/post'
import { Button, Typography, cardStyles, typographyStyles } from '@/shared/ui'
import type { GuideStep } from '../model/guide'

/** Guide: 확인 — the short summary 글 생성 ends on. Each part of what the run will be given is one
 *  row: its name, what it holds now, and one way to change it. The story, photos and details jump
 *  back to their own screen; 말투 is its own picker in place; 지침 opens its page; the writing
 *  options open the brief. Nothing here is a second copy of a field. */
export function EditorReview({
  post,
  ownerId,
  sponsored,
  memo,
  title,
  templateName,
  voice,
  voiceWarning,
  brief,
  onGo,
  onOpenGuidelines,
}: {
  post?: PostDraft
  ownerId: string
  sponsored: boolean
  memo: string
  title: string
  templateName: string
  /** The 말투 picker, label beside it (useEditorChoices). */
  voice: ReactNode
  voiceWarning?: ReactNode
  /** The writing brief's trigger, drawn as this row's 바꾸기. */
  brief: ReactNode
  onGo: (step: GuideStep) => void
  onOpenGuidelines: () => void
}) {
  const { t } = useTranslation('posts')
  const images = post?.images ?? []
  const videos = post?.videos ?? []
  const media = [
    images.length > 0 ? t('guide.photos', { count: images.length }) : '',
    videos.length > 0 ? t('guide.videos', { count: videos.length }) : '',
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <div className="mt-6 flex flex-col gap-4">
      <ul className={cardStyles({ padding: 'none', className: 'divide-divider divide-y' })}>
        <SummaryRow
          label={t('guide.summary.kind')}
          change={t('guide.change')}
          onChange={() => onGo('kind')}
        >
          {t(sponsored ? 'guide.sponsored' : 'guide.selfPaid')}
        </SummaryRow>
        <SummaryRow
          label={t('guide.summary.photos')}
          change={t('guide.change')}
          onChange={() => onGo('photos')}
        >
          {images.length > 0 ? (
            <span className="flex items-center gap-2">
              <span aria-hidden="true" className="flex -space-x-3">
                {images.slice(0, 4).map((image) => (
                  <span
                    key={image.id}
                    className="ring-card-bg inline-block overflow-hidden rounded-md ring-2"
                  >
                    <Thumbnail
                      src={image.viewUrl || undefined}
                      alt=""
                      size="small"
                      rotation={image.rotation}
                    />
                  </span>
                ))}
              </span>
              <span>{media}</span>
            </span>
          ) : (
            media || t('guide.noPhotos')
          )}
        </SummaryRow>
        <SummaryRow
          label={t('guide.summary.title')}
          change={t('guide.change')}
          onChange={() => onGo('details')}
        >
          <span className="line-clamp-2">{title.trim() || t('guide.titleEmpty')}</span>
        </SummaryRow>
        <SummaryRow
          label={t('guide.summary.template')}
          change={t('guide.change')}
          onChange={() => onGo('template')}
        >
          {templateName}
        </SummaryRow>
        <SummaryRow
          label={t('guide.summary.story')}
          change={t('guide.change')}
          onChange={() => onGo('story')}
        >
          <span className="line-clamp-2 whitespace-pre-line">
            {memo.trim() || t('guide.storyEmpty')}
          </span>
        </SummaryRow>
      </ul>
      <div className={cardStyles({ padding: 'none', className: 'divide-divider divide-y' })}>
        <div className="px-4 py-3 sm:px-5">
          {voice}
          {voiceWarning}
        </div>
        <GuidelinesRow ownerId={ownerId} onOpen={onOpenGuidelines} />
        <div className="flex min-h-14 items-center gap-3 px-4 py-2 sm:px-5">
          <Typography variant="label" as="span" className="w-20 shrink-0">
            {t('guide.summary.options')}
          </Typography>
          <span className="min-w-0 flex-1" />
          {brief}
        </div>
      </div>
    </div>
  )
}

function SummaryRow({
  label,
  change,
  onChange,
  children,
}: {
  label: string
  change: string
  onChange: () => void
  children: ReactNode
}) {
  return (
    <li className="flex min-h-14 items-center gap-3 px-4 py-3 sm:px-5">
      <Typography variant="label" as="span" className="w-20 shrink-0 self-start pt-0.5">
        {label}
      </Typography>
      <span
        className={typographyStyles({
          variant: 'body',
          className: 'text-content-primary min-w-0 flex-1',
        })}
      >
        {children}
      </span>
      {/* The row's name rides the button, so a screen reader hears "이야기 바꾸기" rather than three
          identical 바꾸기. */}
      <Button
        variant="ghost"
        onClick={onChange}
        aria-label={`${label} ${change}`}
        className="text-link-fg-current shrink-0 self-start"
      >
        {change}
      </Button>
    </li>
  )
}

/** The guidelines the next run follows, counted: the owner's own and the 기본 지침 switched on. */
function GuidelinesRow({ ownerId, onOpen }: { ownerId: string; onOpen: () => void }) {
  const { t } = useTranslation('posts')
  const { guidelines, defaults, isPending, isError } = useGuidelines(ownerId, 'post')
  const label = t('guide.summary.guidelines')
  return (
    <div className="flex min-h-14 items-center gap-3 px-4 py-2 sm:px-5">
      <Typography variant="label" as="span" className="w-20 shrink-0">
        {label}
      </Typography>
      <span
        className={typographyStyles({
          variant: 'body',
          className: 'text-content-primary min-w-0 flex-1 truncate',
        })}
      >
        {isPending || isError
          ? ''
          : t('guide.guidelines', {
              mine: guidelines.length,
              defaults: defaults.filter((entry) => entry.enabled).length,
            })}
      </span>
      <Button
        variant="ghost"
        onClick={onOpen}
        aria-label={`${label} ${t('guide.change')}`}
        className="text-link-fg-current shrink-0"
      >
        {t('guide.change')}
      </Button>
    </div>
  )
}
