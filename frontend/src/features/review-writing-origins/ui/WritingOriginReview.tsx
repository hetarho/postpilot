import { ReturnIcon } from '@/shared/ui'
import { useEffect, useId, useState, type ReactNode } from 'react'
import { clsx } from 'clsx'
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
import type { VoiceHabitPhrase } from '@/entities/voice'
import type { PostContent } from '@/shared/api'
import { Button, Checkbox, SegmentedControl, SelectableText, Sheet, Typography } from '@/shared/ui'
import { copyOriginSelection } from '../lib/copy-origin-selection'
import { habitSegments } from '../lib/habit-segments'

const CATEGORY_STYLES: Record<SemanticOriginCategory, string> = {
  owner_input: 'text-origin-owner-foreground bg-origin-owner-highlight decoration-solid',
  photo_interpretation:
    'text-origin-visual-foreground bg-origin-visual-highlight decoration-dashed',
  advertiser_request:
    'text-origin-advertiser-foreground bg-origin-advertiser-highlight decoration-double',
  ai_added: 'text-origin-ai-foreground bg-origin-ai-highlight decoration-dotted',
}
const UNCONFIRMED_STYLE =
  'text-origin-unconfirmed-foreground bg-origin-unconfirmed-highlight decoration-wavy'
/** The legend while 출처 보기 is off: every category in the same quiet grey, told apart only by
 *  its line, so the key stays readable without claiming the colours the draft is not wearing. */
const CATEGORY_MUTED: Record<SemanticOriginCategory, string> = {
  owner_input: 'text-content-tertiary decoration-solid',
  photo_interpretation: 'text-content-tertiary decoration-dashed',
  advertiser_request: 'text-content-tertiary decoration-double',
  ai_added: 'text-content-tertiary decoration-dotted',
}
const UNCONFIRMED_MUTED = 'text-content-tertiary decoration-wavy'
/** The 말투 view's one colour: a thick solid line, so the habit never rests on colour alone. */
const VOICE_HABIT_STYLE =
  'text-voice-habit-foreground bg-voice-habit-highlight decoration-solid decoration-2'

type OriginView = 'meaning' | 'voice'
/** Why a post has no reading to show in 말투 (POST-128). */
export type VoiceViewUnavailableReason = 'none' | 'deleted' | 'unmade'

/** The post's voice reading as the page reads it (POST-102); `reading` is the widget-rendered
 *  reading, passed in because a feature never imports a widget (ARCH-13). */
export interface WritingOriginVoice {
  applicable: boolean
  reason?: VoiceViewUnavailableReason
  /** The content revision the habits were found in. */
  revision?: bigint
  habits: VoiceHabitPhrase[]
  reading?: ReactNode
}

interface WritingOriginReviewProps {
  ownerId: string
  post: PostDraft
  content: PostContent
  pending?: boolean
  voice?: WritingOriginVoice
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

function PostOriginReview({ post, content, pending, voice, children }: WritingOriginReviewProps) {
  const { t } = useTranslation(['posts', 'common'])
  const id = useId()
  const [visible, setVisible] = useState(true)
  const [chosenView, setView] = useState<OriginView>('meaning')
  const [selection, setSelection] = useState<Selection | null>(null)
  const [habitSelection, setHabitSelection] = useState<{
    fence: string
    habit: VoiceHabitPhrase
  } | null>(null)
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
  const voiceAvailable = Boolean(voice?.applicable)
  const view: OriginView = chosenView === 'voice' && voiceAvailable ? 'voice' : 'meaning'
  const voiceReason = voiceAvailable ? undefined : (voice?.reason ?? postVoiceReason(post))
  // Habits are found in one revision's text: drawn only over that very text, never over a newer
  // revision or unsaved local edits whose offsets they no longer describe.
  const habits =
    voice && voiceAvailable && !waiting && voice.revision === post.contentRevision
      ? voice.habits
      : []
  const selectedHabit =
    habitSelection?.fence === fence && visible && view === 'voice' ? habitSelection.habit : null
  if (habitSelection && !selectedHabit) setHabitSelection(null)

  useEffect(() => {
    if (pickerOpen && selected) document.getElementById(`${id}-title`)?.focus()
  }, [id, pickerOpen, selected])

  const close = () => {
    setSelection(null)
    setPickerFence(null)
    setHabitSelection(null)
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
          className={`duration-base underline underline-offset-4 transition-opacity starting:opacity-0 ${
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

  const renderHabitField: OriginTextRenderer = (field, text) => {
    if (!text) return text
    return habitSegments(text, field, habits).map((segment) =>
      segment.habit ? (
        <SelectableText
          key={`${segment.start}:${segment.end}`}
          data-voice-habit-phrase
          className={`duration-base underline underline-offset-4 transition-opacity starting:opacity-0 ${VOICE_HABIT_STYLE}`}
          aria-label={t('originReview.voice.phrase', {
            ns: 'posts',
            kind: t(`originReview.voice.kind.${segment.habit.kind}`, { ns: 'posts' }),
            phrase: segment.text,
          })}
          aria-haspopup="dialog"
          aria-expanded={selectedHabit === segment.habit}
          onActivate={() => setHabitSelection({ fence, habit: segment.habit! })}
        >
          {segment.text}
        </SelectableText>
      ) : (
        segment.text
      ),
    )
  }
  // Captions are not searched for habits, so their alt text stays plain in 말투.
  const renderPlainAltField: OriginTextRenderer = (_field, text) =>
    text ? (
      <div role="group" aria-label={t('originReview.voice.alt', { ns: 'posts' })} className="mt-2">
        <Typography variant="label" as="p" data-writing-origin-review-ui>
          {t('originReview.voice.alt', { ns: 'posts' })}
        </Typography>
        <Typography variant="body" as="p" className="mt-1 break-words whitespace-pre-wrap">
          {text}
        </Typography>
      </div>
    ) : null
  const renderers = !visible
    ? { renderTextField: undefined, renderAltField: undefined }
    : view === 'voice'
      ? { renderTextField: renderHabitField, renderAltField: renderPlainAltField }
      : { renderTextField, renderAltField }

  return (
    <>
      {/* One steady bar: the toggle only shows or hides the colours. Nothing in it appears or
          leaves — 문구 출처 찾기 stays and is disabled while the colours are off — and the legend
          is always there, coloured while the draft wears its colours and grey while it does not,
          so a press never moves the draft under the owner's thumb. The two longer notes live in
          문구 출처 찾기's sheet, where the owner goes to read about a phrase. */}
      <div className="my-4 space-y-2">
        <div className="flex min-h-11 flex-wrap items-center justify-between gap-2">
          <label className="inline-flex min-h-11 cursor-pointer items-center gap-3">
            <Checkbox checked={visible} onChange={(event) => setVisible(event.target.checked)} />
            <Typography variant="body" as="span">
              {t('originReview.toggle', { ns: 'posts' })}
            </Typography>
          </label>
          <Button
            variant="ghost"
            disabled={!visible || view === 'voice'}
            aria-haspopup="dialog"
            aria-expanded={pickerOpen}
            onClick={() => {
              setSelection(null)
              setPickerFence(fence)
            }}
          >
            {t('originReview.find', { ns: 'posts' })}
          </Button>
        </div>
        {/* 의미 출처 first: two colour systems on one phrase cannot both be read, so one view
            shows at a time (POST-128). */}
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <SegmentedControl<OriginView>
            ariaLabel={t('originReview.voice.views', { ns: 'posts' })}
            value={view}
            disabled={!visible}
            onChange={(next) => {
              setSelection(null)
              setPickerFence(null)
              setHabitSelection(null)
              setView(next)
            }}
            options={[
              { value: 'meaning', label: t('originReview.voice.meaning', { ns: 'posts' }) },
              {
                value: 'voice',
                label: t('originReview.voice.voice', { ns: 'posts' }),
                disabled: !voiceAvailable,
              },
            ]}
          />
          {voiceReason && (
            <Typography variant="meta" as="p" data-voice-view-reason>
              {t(`originReview.voice.reason.${voiceReason}`, { ns: 'posts' })}
            </Typography>
          )}
        </div>
        {view === 'meaning' ? (
          <ul
            aria-label={t('originReview.legend', { ns: 'posts' })}
            className="flex flex-wrap gap-3"
          >
            {SEMANTIC_ORIGIN_CATEGORIES.map((category) => (
              <Typography variant="body" as="li" key={category}>
                <span
                  data-origin-legend={visible ? 'on' : 'off'}
                  className={clsx(
                    'duration-base rounded-sm underline underline-offset-4 transition-colors',
                    visible ? CATEGORY_STYLES[category] : CATEGORY_MUTED[category],
                  )}
                >
                  {t(`originReview.category.${category}`, { ns: 'posts' })}
                </span>
              </Typography>
            ))}
            {/* A coloured phrase the legend does not name reads as an unexplained category. */}
            <Typography variant="body" as="li">
              <span
                data-origin-legend={visible ? 'on' : 'off'}
                title={t('originReview.unconfirmedLegend', { ns: 'posts' })}
                className={clsx(
                  'duration-base rounded-sm underline underline-offset-4 transition-colors',
                  visible ? UNCONFIRMED_STYLE : UNCONFIRMED_MUTED,
                )}
              >
                {t('originReview.unconfirmed', { ns: 'posts' })}
              </span>
            </Typography>
          </ul>
        ) : (
          <ul
            aria-label={t('originReview.voice.legend', { ns: 'posts' })}
            className="flex flex-wrap gap-3"
          >
            <Typography variant="body" as="li">
              <span
                className={clsx(
                  'duration-base rounded-sm underline underline-offset-4 transition-colors',
                  visible
                    ? VOICE_HABIT_STYLE
                    : 'text-content-tertiary decoration-solid decoration-2',
                )}
              >
                {t('originReview.voice.habit', { ns: 'posts' })}
              </span>
            </Typography>
          </ul>
        )}
      </div>
      {visible && view === 'voice' && voice?.reading}
      <div className="contents" onCopy={copyOriginSelection}>
        {children(renderers)}
      </div>
      {selectedHabit && (
        <Sheet
          open
          labelledBy={`${id}-habit-title`}
          onClose={close}
          header={
            <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
              <Typography variant="title" id={`${id}-habit-title`} className="min-w-0 flex-1">
                {t('originReview.voice.detail', { ns: 'posts' })}
              </Typography>
              <Button variant="ghost" onClick={close}>
                {t('action.close', { ns: 'common' })}
              </Button>
            </div>
          }
        >
          <HabitDetail habit={selectedHabit} />
        </Sheet>
      )}
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
                  variant="secondary"
                  size="icon"
                  aria-label={t('originReview.back', { ns: 'posts' })}
                  title={t('originReview.back', { ns: 'posts' })}
                  onClick={() => {
                    const returnId = selected.returnId
                    setSelection(null)
                    if (returnId) queueMicrotask(() => document.getElementById(returnId)?.focus())
                  }}
                >
                  <ReturnIcon />
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
              <div className="mb-4 space-y-3">
                <Typography variant="body" className="text-content-secondary">
                  {t('originReview.help', { ns: 'posts' })}
                </Typography>
                <Typography variant="body" className="text-content-secondary">
                  {t('originReview.unconfirmedHelp', { ns: 'posts' })}
                </Typography>
              </div>
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

/** The reason 말투 has nothing to read when the page did not name one: the post's own voice. */
function postVoiceReason(post: PostDraft): VoiceViewUnavailableReason | undefined {
  if (!post.voice) return 'none'
  if (post.voice.deleted) return 'deleted'
  if (!post.voice.made) return 'unmade'
  return undefined
}

function HabitDetail({ habit }: { habit: VoiceHabitPhrase }) {
  const { t } = useTranslation('posts')
  const rate = Math.round(habit.voicePerHundred * 10) / 10
  return (
    <div className="space-y-4">
      <Typography variant="body">
        {t(`originReview.voice.kind.${habit.kind}`)}
        {habit.term ? ` · ${habit.term}` : ''}
      </Typography>
      <Typography variant="body">
        {rate > 0
          ? t('originReview.voice.rate', { rate, times: habit.textCount })
          : t('originReview.voice.count', { times: habit.textCount })}
      </Typography>
      {habit.example && (
        <div className="space-y-1">
          <Typography variant="body" className="break-words whitespace-pre-wrap">
            {t('originReview.voice.example', { example: habit.example })}
          </Typography>
          {habit.exampleFictional && (
            <Typography variant="meta" as="p">
              {t('originReview.voice.fictional')}
            </Typography>
          )}
        </div>
      )}
    </div>
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
