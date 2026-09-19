import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  splitTextPhrases,
  textInterval,
  type ClipEditPlan,
  type ClipEditableText,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import { CLIP_ACCENTS } from '@/entities/clip-template'
import {
  CLIP_CAPTION_STYLES,
  CLIP_DEFAULT_CAPTION_STYLE,
  CLIP_RAPID,
  clipCaptionSizes,
} from '@/entities/clip-design'
import {
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import { ClipTimeField } from './ClipTimeField'

export function ClipTextControls({
  plan,
  text,
  change,
  invalid,
  notices = [],
  language,
  captionStyles = [],
}: {
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  plan: ClipEditPlan
  text: ClipEditableText
  change: (edit: TimelineEdit, group?: string) => void
  invalid: boolean
  /** The styles THIS project allows a caption to take (CLIP-142). Empty is a
   *  project that selected none, which is the default style alone. */
  captionStyles?: readonly string[]
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
  // The project's own selection, narrowed to styles this build actually knows:
  // an id it does not carry is one the server would refuse anyway (CLIP-142).
  const allowed = CLIP_CAPTION_STYLES.filter((id) =>
    captionStyles.length ? captionStyles.includes(id) : id === CLIP_DEFAULT_CAPTION_STYLE,
  )
  const sizes = clipCaptionSizes(text.ownerStyle)
  const [size, setSize] = useState(text.ownerSizePx ? String(text.ownerSizePx) : '')
  const typed = Number(size)
  const sizeRefused =
    size.trim() !== '' && (!Number.isFinite(typed) || typed < sizes.min || typed > sizes.max)
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
      {invalid && <FieldMessage>{t('timeline.textInvalid')}</FieldMessage>}
      {text.staleEvidence && !text.evidenceReviewed && (
        <div role="alert" className="space-y-2">
          <FieldMessage>{t('timeline.stale')}</FieldMessage>
          <Button variant="secondary" onClick={() => patch({ evidenceReviewed: true })}>
            {t('timeline.reviewed')}
          </Button>
        </div>
      )}
      <ClipNoticeList notices={textNotices} language={language} />
      <div>
        <FieldLabel htmlFor="clip-selected-text">{t('correction.copy')}</FieldLabel>
        <Textarea
          id="clip-selected-text"
          autoGrow
          value={text.text}
          onChange={(e) => patch({ text: e.target.value }, 'text')}
        />
      </div>
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
          <div>
            <FieldLabel id="clip-caption-style-label">{t('placement.style')}</FieldLabel>
            <Listbox
              aria-labelledby="clip-caption-style-label"
              value={text.ownerStyle ?? ''}
              options={[
                { value: '', label: t('placement.styleDefault') },
                ...allowed.map((value) => ({ value, label: t(`captionStyles.${value}`) })),
              ]}
              onChange={(ownerStyle) =>
                patch({ ownerStyle: ownerStyle || undefined, ownerSizePx: undefined })
              }
            />
          </div>
          <div>
            <FieldLabel htmlFor="clip-caption-size">{t('placement.size')}</FieldLabel>
            <TextField
              id="clip-caption-size"
              inputMode="numeric"
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
              <FieldMessage>
                {t('placement.sizeRange', { min: sizes.min, max: sizes.max })}
              </FieldMessage>
            ) : (
              <Typography variant="meta">
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
