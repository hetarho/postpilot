import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { ReplacementSpan } from '@/entities/post'
import { Button, InlinePopover, Typography } from '@/shared/ui'

/** One replacement span in ②'s read view (POST-79): the text the write offered phrases for,
 *  pressable, opening those phrases. Taking one is an ordinary edit of that occurrence (POST-80);
 *  opening, closing or ignoring the mark sends nothing.
 *
 *  Inside a sentence the mark is sized by the line it sits in, which is WCAG 2.5.8's inline
 *  exception: growing it to 44 px would break the line apart. A tag chip stands alone, so there
 *  the pressed box itself reaches THEME-23's floor under a coarse pointer. */
export function ReplacementMark({
  text,
  span,
  onTake,
}: {
  text: string
  span: ReplacementSpan
  onTake: (phrase: string) => void
}) {
  const { t } = useTranslation('posts')
  return (
    <InlinePopover
      label={t('replacements.panel', { source: span.source })}
      className={
        span.at.surface === 'tag'
          ? 'inline-flex items-center justify-center pointer-coarse:min-h-11 pointer-coarse:min-w-11'
          : undefined
      }
      panel={(close) => (
        <div>
          <Typography variant="label" as="p">
            {t('replacements.source', { source: span.source })}
          </Typography>
          <div className="mt-2 flex flex-wrap gap-2">
            {span.phrases.map((phrase) => (
              <Button
                key={phrase}
                variant="secondary"
                aria-label={t('replacements.take', { phrase })}
                onClick={() => {
                  close()
                  onTake(phrase)
                }}
              >
                {phrase}
              </Button>
            ))}
          </div>
          {/* What was observed about the phrases, and nothing about what they gain (QUAL-21). */}
          <Typography variant="meta" as="p" className="mt-3">
            {t('replacements.observed')}
          </Typography>
        </div>
      )}
    >
      {text}
    </InlinePopover>
  )
}

/** One piece of text split into plain runs and marks, each mark bound to its own span. The spans
 *  arrive in reading order and never overlap (`visibleSpans`). */
export function MarkedText({
  text,
  spans,
  onTake,
}: {
  text: string
  spans: readonly ReplacementSpan[]
  onTake: (span: ReplacementSpan, phrase: string) => void
}) {
  const parts: ReactNode[] = []
  let at = 0
  for (const span of spans) {
    if (span.start > at) parts.push(text.slice(at, span.start))
    parts.push(
      <ReplacementMark
        key={span.start}
        text={text.slice(span.start, span.end)}
        span={span}
        onTake={(phrase) => onTake(span, phrase)}
      />,
    )
    at = span.end
  }
  if (at < text.length) parts.push(text.slice(at))
  return <>{parts}</>
}
