import { useTranslation } from 'react-i18next'
import {
  ClipNoticeList,
  type ClipNotice,
  clipSeconds,
  textInterval,
  splitTextPhrases,
  type ClipEditPlan,
  type ClipEditableText,
  type TimelineEdit,
} from '@/entities/clip-project'
import { CLIP_ACCENTS } from '@/entities/clip-template'
import { CLIP_RAPID } from '@/shared/config'
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
}: {
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  plan: ClipEditPlan
  text: ClipEditableText
  change: (edit: TimelineEdit, group?: string) => void
  invalid: boolean
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
          <Typography variant="meta">{t('editor.accentHelp')}</Typography>
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
        <div className="space-y-3">
          {!narration && (
            <>
              <FieldLabel id="clip-text-pace-label">{t('pace.label')}</FieldLabel>
              <Listbox
                aria-labelledby="clip-text-pace-label"
                value={text.pace || 'steady'}
                options={[
                  { value: 'steady', label: t('pace.steady') },
                  {
                    value: 'rapid',
                    label: t('pace.rapid'),
                    disabled: !splitTextPhrases(plan, text),
                  },
                ]}
                onChange={(pace) =>
                  patch({
                    pace,
                    phrases: pace === 'rapid' ? (splitTextPhrases(plan, text) ?? []) : [],
                  })
                }
              />
            </>
          )}
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
      <Button variant="danger" onClick={() => change({ type: 'removeText', id: text.instanceId })}>
        {t('timeline.deleteText')}
      </Button>
    </div>
  )
}
