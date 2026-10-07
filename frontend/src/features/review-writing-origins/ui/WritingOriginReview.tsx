import { useEffect, useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  originContentMatches,
  originFieldSegments,
  originFieldText,
  originReadableFields,
  SEMANTIC_ORIGIN_CATEGORIES,
  type OriginFieldLocator,
  type OriginFieldSegment,
  type OriginReview,
  type OriginSource,
  type OriginTextRenderer,
  type PostDraft,
  type SemanticOriginCategory,
} from '@/entities/post'
import type { PostContent } from '@/shared/api'
import { Button, Checkbox, SelectableText, Sheet, Typography } from '@/shared/ui'
import { copyOriginSelection } from '../lib/copy-origin-selection'

const CATEGORY_STYLES: Record<SemanticOriginCategory, string> = {
  owner_input: 'text-origin-owner-foreground bg-origin-owner-highlight decoration-solid',
  photo_interpretation:
    'text-origin-visual-foreground bg-origin-visual-highlight decoration-dashed',
  ai_added: 'text-origin-ai-foreground bg-origin-ai-highlight decoration-dotted',
}
const UNCONFIRMED_STYLE =
  'text-origin-unconfirmed-foreground bg-origin-unconfirmed-highlight decoration-wavy'

interface WritingOriginReviewProps {
  ownerId: string
  post: PostDraft
  content: PostContent
  pending?: boolean
  children: (renderers: {
    renderTextField: OriginTextRenderer | undefined
    renderAltField: OriginTextRenderer | undefined
  }) => ReactNode
}

interface Selection {
  fence: string
  field: OriginFieldLocator
  segment: OriginFieldSegment
  returnId?: string
}

/** Owner/post identity resets local disclosure state before another account's text is rendered.
 *  Evidence is already owner-scoped by the post query; this feature makes no provider calls. */
export function WritingOriginReview(props: WritingOriginReviewProps) {
  return <PostOriginReview key={JSON.stringify([props.ownerId, props.post.slug])} {...props} />
}

function PostOriginReview({ post, content, pending, children }: WritingOriginReviewProps) {
  const { t } = useTranslation(['posts', 'common'])
  const id = useId()
  const [visible, setVisible] = useState(true)
  const [selection, setSelection] = useState<Selection | null>(null)
  const [pickerFence, setPickerFence] = useState<string | null>(null)
  const waiting = Boolean(pending) || !originContentMatches(content, post.content)
  const review = availableReview(post)
  // Content edits, current-result changes and withdrawal of a frozen source all dismiss an
  // open explanation synchronously. It can never retarget itself to a later source/result.
  const fence = JSON.stringify(
    [post.contentRevision, post.contentHash, content, waiting, review],
    (_key, value: unknown) => (typeof value === 'bigint' ? value.toString() : value),
  )
  const selected = selection?.fence === fence && visible ? selection : null
  const pickerOpen = pickerFence === fence && visible
  if (selection && !selected) setSelection(null)
  if (pickerFence && !pickerOpen) setPickerFence(null)

  useEffect(() => {
    if (pickerOpen && selected) document.getElementById(`${id}-title`)?.focus()
  }, [id, pickerOpen, selected])

  const close = () => {
    setSelection(null)
    setPickerFence(null)
  }

  const fieldSegments = (field: OriginFieldLocator, text: string): OriginFieldSegment[] => {
    const projected = originFieldSegments(
      content,
      { contentRevision: post.contentRevision, contentHash: post.contentHash ?? '' },
      review,
      field,
      { pending: waiting, canonicalContent: post.content },
    )
    return projected.map((segment) => segment.text).join('') === text
      ? projected
      : [
          {
            text,
            start: 0,
            end: Array.from(text).length,
            state: 'unconfirmed',
            sourceRefs: [],
            reviewState: 'unconfirmed',
            reason: 'quote_mismatch',
          },
        ]
  }
  const renderTextField: OriginTextRenderer = (field, text) => {
    if (!text) return text
    const segments = fieldSegments(field, text)
    return segments.map((segment) => {
      if (!segment.text.trim()) return segment.text
      const category =
        segment.state === 'supported' && segment.category
          ? t(`originReview.category.${segment.category}`, { ns: 'posts' })
          : t('originReview.unconfirmed', { ns: 'posts' })
      return (
        <SelectableText
          key={`${segment.start}:${segment.end}`}
          data-writing-origin-phrase
          className={`underline underline-offset-4 ${
            segment.state === 'supported' && segment.category
              ? CATEGORY_STYLES[segment.category]
              : UNCONFIRMED_STYLE
          }`}
          aria-label={t('originReview.phrase', { ns: 'posts', category, phrase: segment.text })}
          aria-haspopup="dialog"
          aria-expanded={Boolean(
            selected &&
            JSON.stringify(selected.field) === JSON.stringify(field) &&
            selected.segment.start === segment.start,
          )}
          onActivate={() => setSelection({ fence, field, segment })}
        >
          {segment.text}
        </SelectableText>
      )
    })
  }
  const renderAltField: OriginTextRenderer = (field, text) =>
    text ? (
      <div role="group" aria-label={t('originReview.alt', { ns: 'posts' })} className="mt-2">
        <Typography variant="label" as="p" data-writing-origin-review-ui>
          {t('originReview.alt', { ns: 'posts' })}
        </Typography>
        <Typography variant="body" as="p" className="mt-1 break-words whitespace-pre-wrap">
          {renderTextField(field, text)}
        </Typography>
      </div>
    ) : null

  return (
    <>
      <div className="my-4 space-y-3">
        <div className="flex flex-wrap items-center gap-4">
          <label className="inline-flex cursor-pointer items-center gap-3">
            <Checkbox checked={visible} onChange={(event) => setVisible(event.target.checked)} />
            <Typography variant="body" as="span">
              {t('originReview.toggle', { ns: 'posts' })}
            </Typography>
          </label>
          {visible && (
            <Button
              variant="ghost"
              aria-haspopup="dialog"
              aria-expanded={pickerOpen}
              onClick={() => {
                setSelection(null)
                setPickerFence(fence)
              }}
            >
              {t('originReview.find', { ns: 'posts' })}
            </Button>
          )}
        </div>
        {visible && (
          <>
            <ul
              aria-label={t('originReview.legend', { ns: 'posts' })}
              className="flex flex-wrap gap-3"
            >
              {SEMANTIC_ORIGIN_CATEGORIES.map((category) => (
                <Typography variant="body" as="li" key={category}>
                  <span
                    className={`rounded-sm underline underline-offset-4 ${CATEGORY_STYLES[category]}`}
                  >
                    {t(`originReview.category.${category}`, { ns: 'posts' })}
                  </span>
                </Typography>
              ))}
            </ul>
            <Typography variant="body" className="text-content-secondary">
              {t('originReview.help', { ns: 'posts' })}
            </Typography>
            <Typography variant="body" className="text-content-secondary">
              {t('originReview.unconfirmedHelp', { ns: 'posts' })}
            </Typography>
          </>
        )}
      </div>
      <div className="contents" onCopy={copyOriginSelection}>
        {children({
          renderTextField: visible ? renderTextField : undefined,
          renderAltField: visible ? renderAltField : undefined,
        })}
      </div>
      {(selected || pickerOpen) && (
        <Sheet
          open
          labelledBy={`${id}-title`}
          onClose={close}
          header={
            <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
              <Typography
                variant="title"
                id={`${id}-title`}
                tabIndex={pickerOpen && selected ? -1 : undefined}
                className="min-w-0 flex-1"
              >
                {t(selected ? 'originReview.detail' : 'originReview.find', { ns: 'posts' })}
              </Typography>
              {pickerOpen && selected && (
                <Button
                  variant="ghost"
                  onClick={() => {
                    const returnId = selected.returnId
                    setSelection(null)
                    if (returnId) queueMicrotask(() => document.getElementById(returnId)?.focus())
                  }}
                >
                  {t('originReview.back', { ns: 'posts' })}
                </Button>
              )}
              <Button variant="ghost" onClick={close}>
                {t('action.close', { ns: 'common' })}
              </Button>
            </div>
          }
        >
          {selected ? (
            <OriginDetail selection={selected} review={review} />
          ) : (
            <>
              <Typography variant="body" className="mb-4">
                {t('originReview.pickerHelp', { ns: 'posts' })}
              </Typography>
              <ul className="space-y-3">
                {originReadableFields(content).flatMap((field, fieldIndex) =>
                  fieldSegments(field, originFieldText(content, field) ?? '')
                    .filter((segment) => segment.text.trim())
                    .map((segment) => {
                      const returnId = `${id}-choice-${fieldIndex}-${segment.start}`
                      const category =
                        segment.state === 'supported' && segment.category
                          ? t(`originReview.category.${segment.category}`, { ns: 'posts' })
                          : t('originReview.unconfirmed', { ns: 'posts' })
                      const fieldLabel = t(`originReview.field.${field.kind}`, fieldNumbers(field))
                      return (
                        <li key={returnId}>
                          <Button
                            id={returnId}
                            className="w-full justify-start text-left"
                            aria-label={t('originReview.pickerPhrase', {
                              ns: 'posts',
                              field: fieldLabel,
                              category,
                              phrase: segment.text,
                            })}
                            onClick={() => setSelection({ fence, field, segment, returnId })}
                          >
                            <span className="flex min-w-0 flex-col gap-1">
                              <Typography variant="body" as="span">
                                {fieldLabel} · {category}
                              </Typography>
                              <Typography
                                variant="body"
                                as="span"
                                className="break-words whitespace-pre-wrap"
                              >
                                {segment.text}
                              </Typography>
                            </span>
                          </Button>
                        </li>
                      )
                    }),
                )}
              </ul>
            </>
          )}
        </Sheet>
      )}
    </>
  )
}

/** A local attachment removal can precede the server's sidecar response. Only withdraw known
 *  evidence here; never mark it available or replace its frozen identity with a newer filename. */
function availableReview(post: PostDraft): OriginReview | undefined {
  const review = post.contentOrigins
  if (!review || !Array.isArray(review.sources)) return review
  const attachments = [...post.images, ...post.videos]
  return {
    ...review,
    sources: review.sources.map((source) => {
      if (!source || typeof source !== 'object') return source
      if (source.kind !== 'visual_observation' || !source.available) return source
      const present =
        Boolean(source.attachmentId) &&
        attachments.some(
          (attachment) =>
            attachment.id === source.attachmentId &&
            attachment.filename === source.attachmentFilename,
        )
      return present ? source : { ...source, available: false }
    }),
  }
}

function OriginDetail({ selection, review }: { selection: Selection; review?: OriginReview }) {
  const { t } = useTranslation('posts')
  const { field, segment } = selection
  const sources =
    segment.state === 'supported'
      ? (review?.sources.filter(
          (source) => source.available && segment.sourceRefs.includes(source.id),
        ) ?? [])
      : []
  const explanation =
    segment.reason === 'pending'
      ? 'originReview.pending'
      : segment.reason === 'stale_result'
        ? 'originReview.stale'
        : segment.reason === 'unavailable_source'
          ? 'originReview.unavailable'
          : 'originReview.missing'
  return (
    <div className="space-y-4">
      <Typography variant="body">
        {t(`originReview.field.${field.kind}`, fieldNumbers(field))}
      </Typography>
      <Typography variant="body" className="break-words whitespace-pre-wrap">
        {segment.text}
      </Typography>
      <Typography variant="body">
        {segment.state === 'supported' && segment.category
          ? t(`originReview.category.${segment.category}`)
          : t('originReview.unconfirmed')}
      </Typography>
      {segment.state === 'unconfirmed' && <Typography variant="body">{t(explanation)}</Typography>}
      {sources.map((source) => (
        <SourceDetail key={source.id} source={source} />
      ))}
      {segment.state === 'supported' && segment.category === 'ai_added' && sources.length === 0 && (
        <Typography variant="body">{t('originReview.aiWithoutBasis')}</Typography>
      )}
    </div>
  )
}

function fieldNumbers(field: OriginFieldLocator) {
  return {
    number:
      'blockIndex' in field ? field.blockIndex + 1 : 'tagIndex' in field ? field.tagIndex + 1 : 1,
    item: 'itemIndex' in field ? field.itemIndex + 1 : 1,
  }
}

function SourceDetail({ source }: { source: OriginSource }) {
  const { t } = useTranslation('posts')
  return (
    <div className="space-y-2">
      <Typography variant="body">{t(`originReview.source.${source.kind}`)}</Typography>
      {source.kind === 'visual_observation' && source.attachmentFilename && (
        <Typography variant="body" className="break-words">
          {source.attachmentFilename}
        </Typography>
      )}
      <Typography variant="body" className="break-words whitespace-pre-wrap">
        {source.text}
      </Typography>
    </div>
  )
}
