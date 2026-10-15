import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { CircleCheck, CircleDashed } from 'lucide-react'
import type { AdvertiserGuide } from '@/entities/advertiser-guide'
import type { PostContent } from '@/shared/api'
import { Checkbox, Typography, typographyStyles } from '@/shared/ui'
import { countedItems, tickedLines, toggleTick, type CountedItem } from '../lib/count'

/** ③'s pre-submission checklist (POST-119): what the guide asks for, counted against the post as
 *  it stands, and every free-text line as a box the owner ticks. Advisory — it gates nothing —
 *  and it makes no request but the tick save the host performs. Each mark says its state in
 *  words; the icon only repeats it. */
export function AdvertiserChecklist({
  content,
  guide,
  ticks,
  readOnly,
  onTicksChange,
}: {
  content: Pick<PostContent, 'title' | 'tags' | 'blocks'>
  guide: AdvertiserGuide
  ticks: readonly string[]
  readOnly: boolean
  onTicksChange: (ticks: string[]) => void
}) {
  const { t } = useTranslation('posts')
  const id = useId()
  const items = countedItems(content, guide)
  const lines = tickedLines(guide, ticks)
  return (
    <section aria-labelledby={`${id}-title`} className="mt-10">
      <Typography variant="fieldTitle" as="h3" id={`${id}-title`}>
        {t('checklist.title')}
      </Typography>
      <Typography variant="meta" as="p" className="mt-1">
        {t('checklist.help')}
      </Typography>
      {items.length > 0 && (
        <ul aria-label={t('checklist.counted')} className="mt-3 space-y-2">
          {items.map((item) => (
            <CountedRow key={itemKey(item)} item={item} />
          ))}
        </ul>
      )}
      {lines.length > 0 && (
        <fieldset className="mt-4">
          <legend className={typographyStyles({ variant: 'label' })}>{t('checklist.lines')}</legend>
          <Typography variant="meta" as="p" className="mt-1">
            {t('checklist.linesHelp')}
          </Typography>
          <ul className="mt-2">
            {lines.map(({ line, ticked }) => (
              <li key={line}>
                <label
                  className={typographyStyles({
                    variant: 'body',
                    className: 'flex min-h-11 items-start gap-3 py-2',
                  })}
                >
                  <span className="mt-1">
                    <Checkbox
                      checked={ticked}
                      disabled={readOnly}
                      onChange={(event) =>
                        onTicksChange(toggleTick(guide, ticks, line, event.target.checked))
                      }
                    />
                  </span>
                  <span className="min-w-0 break-words">{line}</span>
                </label>
              </li>
            ))}
          </ul>
        </fieldset>
      )}
    </section>
  )
}

function itemKey(item: CountedItem): string {
  switch (item.kind) {
    case 'keyword':
      return `keyword:${item.keyword}`
    case 'hashtag':
      return `hashtag:${item.hashtag}`
    default:
      return item.kind
  }
}

function CountedRow({ item }: { item: CountedItem }) {
  const { t } = useTranslation('posts')
  const Icon = item.met ? CircleCheck : CircleDashed
  return (
    <Typography variant="body" as="li" className="flex items-start gap-3">
      <Icon
        aria-hidden
        className={`mt-1 size-4 shrink-0 ${item.met ? 'text-content-primary' : 'text-content-secondary'}`}
      />
      <span className="min-w-0">
        <span className="break-words">{label(item, t)}</span>
        <span className="text-content-secondary">{` · ${status(item, t)}`}</span>
      </span>
    </Typography>
  )
}

type Translate = ReturnType<typeof useTranslation<'posts'>>['t']

function label(item: CountedItem, t: Translate): string {
  switch (item.kind) {
    case 'keyword':
      return t('checklist.keyword', { keyword: item.keyword })
    case 'hashtag':
      return t('checklist.hashtag', { hashtag: item.hashtag })
    case 'characters':
      return t('checklist.characters', {
        withSpaces: item.count.withSpaces,
        withoutSpaces: item.count.withoutSpaces,
        minimum: item.minimum,
      })
    case 'photos':
      return t('checklist.photos', { count: item.count, minimum: item.minimum })
  }
}

function status(item: CountedItem, t: Translate): string {
  switch (item.kind) {
    case 'keyword':
      return t(item.met ? 'checklist.keywordMet' : 'checklist.keywordMissing')
    case 'hashtag':
      return t(item.met ? 'checklist.hashtagMet' : 'checklist.hashtagMissing')
    case 'characters':
      return item.met
        ? t('checklist.charactersMet')
        : t('checklist.charactersShort', { short: item.minimum - item.count.withoutSpaces })
    case 'photos':
      return item.met
        ? t('checklist.photosMet')
        : t('checklist.photosShort', { short: item.minimum - item.count })
  }
}
