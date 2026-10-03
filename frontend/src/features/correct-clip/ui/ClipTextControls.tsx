import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipCaptionSizeRange,
  clipCaptionStyleOf,
  clipOwnerSizeFits,
  clipSeconds,
  splitTextPhrases,
  textInterval,
  type ClipCaptionFragment,
  type ClipEditPlan,
  type ClipEditableText,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import { CLIP_ACCENTS } from '@/entities/clip-template'
import { CLIP_RAPID } from '@/entities/clip-design'
import type { AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import { ClipCaptionStylePicker } from './ClipCaptionStylePicker'
import { ClipTimeField } from './ClipTimeField'

const regionRole = (role: string) => role === 'hook' || role === 'ending'

export function ClipTextControls({
  plan,
  text,
  change,
  invalid,
  regionOverlap,
  notices = [],
  language,
  captionStyles = [],
  styleSamples,
  samplesUnavailable,
  failure,
}: {
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  plan: ClipEditPlan
  text: ClipEditableText
  change: (edit: TimelineEdit, group?: string) => void
  invalid: boolean
  regionOverlap?: boolean
  /** The project's AI caption set (CLIP-142). It bounds what a writer picks, not the owner:
   *  here it only says what a caption naming no style is drawn in. */
  captionStyles?: readonly string[]
  /** The renderer's drawing of every approved style, once it has arrived (CDS-83). */
  styleSamples?: readonly ClipCaptionFragment[]
  samplesUnavailable?: boolean
  /** The last save's refusal; shown here when it names this caption. */
  failure?: AppFailure
}) {
  const { t } = useTranslation('clips')
  const textNotices = notices.filter(
    (n) => n.elementId === text.elementId && n.cutId === text.cutId,
  )
  if (text.fallbackReason && !textNotices.some((n) => n.code === text.fallbackReason))
    textNotices.push({
      code: text.fallbackReason,
      elementId: text.elementId,
      cutId: text.cutId,
      action: 'repair',
    })
  const interval = textInterval(plan, text)
  // A caption of the narration carries its own absolute window and no placement
  // of any kind: the server chooses the anchor, the project the pace and accent
  // (CLIP-134, CDS-60).
  const narration = !!text.narration
  const patch = (value: Partial<ClipEditableText>, group?: string) =>
    change(
      { type: 'text', id: text.instanceId, patch: value },
      group ? `${text.instanceId}:${group}` : undefined,
    )
  // The size is bounded by the role of the style the caption is DRAWN in — the owner's
  // choice, else the one its plan names (CDS-82, CDS-100).
  const drawn = clipCaptionStyleOf({ style: text.style }, captionStyles)
  const sizes = clipCaptionSizeRange(text, captionStyles)
  const [size, setSize] = useState(text.ownerSizePx ? String(text.ownerSizePx) : '')
  // The field follows the caption's own size when something other than typing moves it — an
  // undo, a redo, another caption selected — so it never shows a size the draft does not hold.
  const held = `${text.instanceId}:${text.ownerSizePx ?? ''}`
  const [shown, setShown] = useState(held)
  if (shown !== held) {
    setShown(held)
    setSize(text.ownerSizePx ? String(text.ownerSizePx) : '')
  }
  const typed = Number(size)
  const sizeRefused =
    (size.trim() !== '' && (!Number.isFinite(typed) || typed < sizes.min || typed > sizes.max)) ||
    !clipOwnerSizeFits(text, captionStyles)
  const refusal =
    failure?.reason === 'CLIP_COMPOSITION_INVALID' && failure.params.element_id === text.elementId
      ? failure
      : undefined
  const phrases = text.phrases?.length
    ? text.phrases
    : text.pace === 'rapid'
      ? (splitTextPhrases(plan, text) ?? [])
      : []
  const phraseChange = (index: number, value: Partial<(typeof phrases)[number]>, group: string) =>
    patch({ phrases: phrases.map((p, i) => (i === index ? { ...p, ...value } : p)) }, group)
  return (
    <div className="space-y-4" aria-label={t('timeline.textControls')}>
      <Typography variant="fieldTitle">
        {t(text.kind === 'ai' ? 'timeline.generated' : 'timeline.fixed')} · {text.elementId}
      </Typography>
      <Typography variant="meta">
        {t('timeline.outputRange', {
          start: clipSeconds(interval.startMs),
          end: clipSeconds(interval.endMs),
        })}
      </Typography>
      {invalid && (
        <FieldMessage>
          {t(regionOverlap ? 'timeline.captionRegionOverlap' : 'timeline.textInvalid')}
        </FieldMessage>
      )}
      {refusal && (
        <div role="alert">
          <AppFailureMessage failure={refusal} />
        </div>
      )}
      {text.staleEvidence && !text.evidenceReviewed && (
        <div role="alert" className="space-y-2">
          <FieldMessage>{t('timeline.stale')}</FieldMessage>
          <Button variant="secondary" onClick={() => patch({ evidenceReviewed: true })}>
            {t('timeline.reviewed')}
          </Button>
        </div>
      )}
      <ClipNoticeList notices={textNotices} language={language} />
      {/* An intro or outro draws its slots, one row each, and no text of its
          own beside them (CLIP-188). */}
      {!(regionRole(text.role) && text.rows.length > 0) && (
        <div>
          <FieldLabel htmlFor="clip-selected-text">{t('correction.copy')}</FieldLabel>
          <Textarea
            id="clip-selected-text"
            autoGrow
            value={text.text}
            onChange={(e) => patch({ text: e.target.value }, 'text')}
          />
        </div>
      )}
      {text.rows.map((row, index) => (
        <div key={index}>
          <FieldLabel htmlFor={`clip-text-row-${index}`}>
            {t('timeline.row', { number: index + 1 })}
          </FieldLabel>
          <Textarea
            id={`clip-text-row-${index}`}
            autoGrow
            value={row.text}
            onChange={(e) =>
              patch(
                {
                  rows: text.rows.map((r, i) => (i === index ? { ...r, text: e.target.value } : r)),
                },
                `row-${index}`,
              )
            }
          />
        </div>
      ))}
      {narration && (
        <div className="grid grid-cols-2 gap-3">
          <ClipTimeField
            id="clip-text-start"
            label={t('timeline.narrationStart')}
            value={text.startMs ?? interval.startMs}
            onChange={(startMs) => patch({ startMs, endMs: text.endMs ?? interval.endMs }, 'start')}
          />
          <ClipTimeField
            id="clip-text-end"
            label={t('timeline.narrationEnd')}
            value={text.endMs ?? interval.endMs}
            onChange={(endMs) => patch({ startMs: text.startMs ?? interval.startMs, endMs }, 'end')}
          />
        </div>
      )}
      {!narration && (
        <div>
          <FieldLabel id="clip-basis-label">{t('timeline.basis')}</FieldLabel>
          <Listbox
            aria-labelledby="clip-basis-label"
            value={text.basis}
            options={(['whole', 'output-start', 'output-end', 'cut'] as const).map((value) => ({
              value,
              label: t(`timeline.bases.${value}`),
              disabled: value === 'cut' && !plan.cuts.some((c) => c.id === text.cutId),
            }))}
            onChange={(basis) => {
              const offset =
                basis === 'output-end'
                  ? plan.durationMs
                  : basis === 'cut'
                    ? interval.cutOffsetMs
                    : 0
              patch({
                basis,
                startMs: basis === 'whole' ? undefined : interval.startMs - offset,
                endMs: basis === 'whole' ? undefined : interval.endMs - offset,
              })
            }}
          />
        </div>
      )}
      {!narration && text.basis !== 'whole' && (
        <div className="grid grid-cols-2 gap-3">
          <ClipTimeField
            id="clip-text-start"
            label={t('timeline.textStart')}
            value={text.startMs ?? interval.startMs - interval.cutOffsetMs}
            onChange={(startMs) =>
              patch(
                { startMs, endMs: text.endMs ?? interval.endMs - interval.cutOffsetMs },
                'start',
              )
            }
          />
          <ClipTimeField
            id="clip-text-end"
            label={t('timeline.textEnd')}
            value={text.endMs ?? interval.endMs - interval.cutOffsetMs}
            onChange={(endMs) =>
              patch(
                { startMs: text.startMs ?? interval.startMs - interval.cutOffsetMs, endMs },
                'end',
              )
            }
          />
        </div>
      )}
      {!narration && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <FieldLabel id="clip-text-position-label">{t('correction.position')}</FieldLabel>
            <Listbox
              aria-labelledby="clip-text-position-label"
              value={text.position}
              options={(
                ['auto', 'header', 'top', 'upper_mid', 'lower_mid', 'bottom', 'center'] as const
              ).map((value) => ({ value, label: t(`timeline.positions.${value}`) }))}
              onChange={(position) => patch({ position })}
            />
          </div>
          <div>
            <FieldLabel id="clip-text-align-label">{t('correction.align')}</FieldLabel>
            <Listbox
              aria-labelledby="clip-text-align-label"
              value={text.align}
              options={(['left', 'center', 'right'] as const).map((value) => ({
                value,
                label: t(`aligns.${value}`),
              }))}
              onChange={(align) => patch({ align })}
            />
          </div>
          {text.role === 'caption' && (
            <div>
              <FieldLabel id="clip-text-accent-label">{t('editor.accent')}</FieldLabel>
              <Listbox
                aria-labelledby="clip-text-accent-label"
                value={text.accent}
                options={CLIP_ACCENTS.map((value) => ({
                  value,
                  label: t(`accent.${value || 'none'}`),
                }))}
                onChange={(accent) => patch({ accent })}
              />
            </div>
          )}
        </div>
      )}
      {!narration && text.role === 'caption' && (
        <>
          <div>
            <FieldLabel htmlFor="clip-text-keyword">{t('correction.keyword')}</FieldLabel>
            <TextField
              id="clip-text-keyword"
              value={text.keyword}
              onChange={(e) => patch({ keyword: e.target.value }, 'keyword')}
            />
          </div>
        </>
      )}
      {text.role === 'caption' && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div className="sm:col-span-2">
            {/* A new style keeps the owner's size and place: a size it cannot take is reported
                on the size field below and holds the save, never quietly cleared (CDS-100). */}
            <ClipCaptionStylePicker
              value={text.ownerStyle}
              drawn={drawn}
              samples={styleSamples}
              unavailable={samplesUnavailable}
              onChange={(ownerStyle) => patch({ ownerStyle })}
            />
          </div>
          <div>
            <FieldLabel htmlFor="clip-caption-size">{t('placement.size')}</FieldLabel>
            <TextField
              id="clip-caption-size"
              inputMode="numeric"
              aria-invalid={sizeRefused || undefined}
              aria-describedby="clip-caption-size-range"
              value={size}
              onChange={(e) => setSize(e.target.value)}
              onBlur={() => {
                const value = Number(size)
                if (!size.trim()) return patch({ ownerSizePx: undefined })
                // CDS-3's floor is refused AT THE CONTROL: the owner keeps what
                // they typed and the caption keeps the size it had.
                if (!Number.isFinite(value) || value < sizes.min || value > sizes.max) return
                patch({ ownerSizePx: Math.round(value) })
              }}
            />
            {sizeRefused ? (
              <FieldMessage id="clip-caption-size-range">
                {t('placement.sizeRange', { min: sizes.min, max: sizes.max })}
              </FieldMessage>
            ) : (
              <Typography variant="meta" id="clip-caption-size-range">
                {t('placement.sizeRange', { min: sizes.min, max: sizes.max })}
              </Typography>
            )}
          </div>
          {text.ownerPosition && (
            <Button variant="ghost" onClick={() => patch({ ownerPosition: undefined })}>
              {t('placement.reset')}
            </Button>
          )}
        </div>
      )}
      {text.role === 'caption' && (
        <div className="space-y-3">
          {text.pace === 'rapid' &&
            phrases.map((phrase, index) => (
              <div key={index} className="space-y-2">
                <FieldLabel htmlFor={`clip-phrase-${index}`}>
                  {t('pace.phrase', { number: index + 1 })}
                </FieldLabel>
                <TextField
                  id={`clip-phrase-${index}`}
                  value={phrase.text}
                  onChange={(e) =>
                    phraseChange(index, { text: e.target.value }, `phrase-${index}-text`)
                  }
                />
                <div className="grid grid-cols-2 gap-3">
                  <ClipTimeField
                    id={`clip-phrase-${index}-start`}
                    label={t('timeline.phraseStart')}
                    value={phrase.startMs}
                    onChange={(startMs) =>
                      phraseChange(index, { startMs }, `phrase-${index}-start`)
                    }
                  />
                  <ClipTimeField
                    id={`clip-phrase-${index}-end`}
                    label={t('timeline.phraseEnd')}
                    value={phrase.endMs}
                    onChange={(endMs) => phraseChange(index, { endMs }, `phrase-${index}-end`)}
                  />
                </div>
                <Button
                  variant="ghost"
                  disabled={phrases.length < 2}
                  onClick={() => patch({ phrases: phrases.filter((_, i) => i !== index) })}
                >
                  {t('pace.removePhrase', { number: index + 1 })}
                </Button>
              </div>
            ))}
          {text.pace === 'rapid' && (
            <Button
              variant="secondary"
              disabled={phrases.length >= CLIP_RAPID.max_per_cut}
              onClick={() => {
                const startMs = phrases.at(-1)?.endMs ?? interval.startMs - interval.cutOffsetMs
                patch({
                  phrases: [...phrases, { text: '', startMs, endMs: startMs + CLIP_RAPID.min_ms }],
                })
              }}
            >
              {t('pace.addPhrase')}
            </Button>
          )}
        </div>
      )}
      {text.evidence?.map((e, index) => (
        <Typography key={index} variant="meta">
          {t('timeline.evidence', {
            source: e.sourceId,
            start: clipSeconds(e.startMs),
            end: clipSeconds(e.endMs),
          })}
        </Typography>
      ))}
      {/* No delete here: it is the item sheet's pinned footer that carries it,
          beside the cut's own, so a destructive control is never at the bottom
          of a scroller (CLIP-53). */}
    </div>
  )
}
