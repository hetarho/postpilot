import { useTranslation } from 'react-i18next'
import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import {
  clipSeconds,
  cutOutputMs,
  textInterval,
  timelineCuts,
  type ClipEditCut,
  type ClipEditPlan,
  type ClipEditableText,
} from '@/entities/clip-plan'
import { Typography } from '@/shared/ui'

/** One line of the reading: what it is, and the value the clip was made with. */
function Line({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-3">
      <Typography variant="meta" as="dt" className="shrink-0">
        {label}
      </Typography>
      <Typography variant="body" as="dd" className="min-w-0 break-words">
        {value}
      </Typography>
    </div>
  )
}

/** The same sheet a correction opens, with values where its controls were: a finalized project is
 *  read, never edited, and its originals are gone (CLIP-160, CLIP-76). */
export function ClipCutReading({
  plan,
  cut,
  filename,
  notices,
  language,
}: {
  plan: ClipEditPlan
  cut: ClipEditCut
  filename?: string
  notices: readonly ClipNotice[]
  language?: 'ko' | 'en'
}) {
  const { t } = useTranslation('clips')
  const index = plan.cuts.indexOf(cut)
  const time = timelineCuts(plan)[index]
  return (
    <div className="space-y-4">
      <dl className="space-y-2">
        <Line label={t('reading.source')} value={filename ?? cut.sourceId} />
        {time && (
          <Line
            label={t('reading.outputRange')}
            value={t('timeline.outputRange', {
              start: clipSeconds(time.startMs),
              end: clipSeconds(time.endMs),
            })}
          />
        )}
        <Line
          label={t('reading.sourceRange')}
          value={`${clipSeconds(cut.startMs)}–${clipSeconds(cut.endMs)} s`}
        />
        <Line
          label={t('assembly.rate')}
          value={`${cut.playbackRatePermille / 1000}x · ${clipSeconds(cutOutputMs(cut))} s`}
        />
        <Line
          label={t('correction.transition')}
          value={
            cut.transitionMs === 0
              ? t('correction.transitions.cut')
              : cut.transitionMs === 300
                ? t('timeline.fadeBlack')
                : t('correction.transitions.fade', { ms: cut.transitionMs })
          }
        />
        <Line label={t('correction.volume')} value={`${cut.volumePermille / 10}%`} />
      </dl>
      {!plan.nativeComposition && cut.copies.length > 0 && (
        <div className="space-y-2">
          <Typography variant="fieldTitle" as="h3">
            {t('correction.copy')}
          </Typography>
          {cut.copies.map((copy, i) => (
            <div key={i}>
              <Typography variant="body" className="break-words">
                {copy.text}
              </Typography>
              <Typography variant="meta">
                {t('timeline.outputRange', {
                  start: clipSeconds(copy.startMs),
                  end: clipSeconds(copy.endMs),
                })}
              </Typography>
            </div>
          ))}
        </div>
      )}
      <ClipNoticeList
        notices={notices.filter((n) => n.cutId === cut.id && !n.elementId)}
        language={language}
      />
    </div>
  )
}

export function ClipTextReading({
  plan,
  text,
  notices,
  language,
}: {
  plan: ClipEditPlan
  text: ClipEditableText
  notices: readonly ClipNotice[]
  language?: 'ko' | 'en'
}) {
  const { t } = useTranslation('clips')
  const interval = textInterval(plan, text)
  const style = text.ownerStyle || text.style
  return (
    <div className="space-y-4">
      <Typography variant="body" className="break-words">
        {text.text}
      </Typography>
      <dl className="space-y-2">
        <Line
          label={t('reading.interval')}
          value={t('timeline.outputRange', {
            start: clipSeconds(interval.startMs),
            end: clipSeconds(interval.endMs),
          })}
        />
        {style && <Line label={t('reading.style')} value={style} />}
        <Line
          label={t('reading.pace')}
          value={t(text.pace === 'rapid' ? 'reading.paceRapid' : 'reading.paceSteady')}
        />
        {text.keyword && <Line label={t('reading.keyword')} value={text.keyword} />}
        {text.ownerPosition && (
          <Line
            label={t('reading.placement')}
            value={`${Math.round(text.ownerPosition.x)}, ${Math.round(text.ownerPosition.y)}`}
          />
        )}
        {text.ownerSizePx && (
          <Line label={t('reading.size')} value={`${Math.round(text.ownerSizePx)} px`} />
        )}
      </dl>
      {text.phrases && text.phrases.length > 0 && (
        <div className="space-y-1">
          {text.phrases.map((phrase, i) => (
            <Typography key={i} variant="meta" className="break-words">
              {`${phrase.text} · ${clipSeconds(phrase.startMs)}–${clipSeconds(phrase.endMs)} s`}
            </Typography>
          ))}
        </div>
      )}
      <ClipNoticeList
        notices={notices.filter((n) => n.elementId === text.elementId && n.cutId === text.cutId)}
        language={language}
      />
    </div>
  )
}
