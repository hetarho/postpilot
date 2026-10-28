import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import { Image as ImageIcon, Repeat2, Sparkles } from 'lucide-react'
import { Typography } from '@/shared/ui'

/** The stand-ins a template preview draws a post from (TMPL-66): each keeps what is fixed and
 *  what the AI writes visibly apart, and none is larger than the text around it, so the card
 *  reads as the post a template makes rather than as a layout of grey boxes. They carry no
 *  sample prose and no grammar syntax. */

/** How many passes a 사진마다 반복 is drawn with: two show that it repeats, and nothing on a
 *  hardcoded preview asks to be operated (TMPL-66). */
export const PREVIEW_REPEAT_PASSES = 2

/** One post at reading width, no wider than a phone's measure. */
export function PreviewPostCard({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div
      className={twMerge(
        'bg-surface-raised border-divider mx-auto w-full max-w-sm min-w-0 rounded-2xl border px-4 py-5',
        className,
      )}
    >
      {children}
    </div>
  )
}

/** A few grey text lines, the last one short: the shape of a paragraph, never its words. */
export function PreviewLines({ lines = 3 }: { lines?: number }) {
  return (
    <div aria-hidden className="space-y-1.5">
      {Array.from({ length: lines }, (_, line) => (
        <div
          key={line}
          className={twMerge(
            'bg-surface-recessed h-2 rounded-full',
            line === lines - 1 ? 'w-3/5' : line % 2 === 1 ? 'w-11/12' : 'w-full',
          )}
        />
      ))}
    </div>
  )
}

/** What the AI writes: a small mark naming what its place is about over line bars. */
export function WriteStandIn({ topic, children }: { topic: string; children?: ReactNode }) {
  const { t } = useTranslation('templates')
  return (
    <div className="min-w-0 space-y-2">
      <Typography
        variant="meta"
        as="p"
        className="text-content-tertiary flex min-w-0 items-center gap-1.5"
      >
        <Sparkles aria-hidden className="size-3.5 shrink-0" />
        <span className="shrink-0">{t('preview.aiWrites')}</span>
        {topic !== '' && (
          <>
            <span aria-hidden>·</span>
            <span className="text-content-secondary min-w-0 truncate">{topic}</span>
          </>
        )}
      </Typography>
      <PreviewLines />
      {children}
    </div>
  )
}

/** A photo place: one short strip of small tiles, never full-width squares (TMPL-66). */
export function PhotoStandIn({ count }: { count: number }) {
  const { t } = useTranslation('templates')
  const label = t('preview.photos', { count })
  return (
    <div className="flex items-center gap-2">
      <div role="img" aria-label={label} className="flex gap-1">
        {Array.from({ length: count }, (_, cell) => (
          <div
            key={cell}
            className="bg-surface-recessed text-content-tertiary flex size-11 items-center justify-center rounded-md"
          >
            <ImageIcon aria-hidden className="size-4" />
          </div>
        ))}
      </div>
      <Typography variant="meta" as="span" aria-hidden>
        {label}
      </Typography>
    </div>
  )
}

/** What the post's author types: a highlighted placeholder, never a sample answer. */
export function AnswerChip({ children }: { children?: ReactNode }) {
  const { t } = useTranslation('templates')
  return (
    <Typography
      variant="label"
      as="span"
      className="bg-badge-accent-bg text-badge-accent-fg inline-flex max-w-full rounded-md px-2 py-0.5 break-words"
    >
      {children ?? t('preview.answer')}
    </Typography>
  )
}

/** A data field: its 제목 over the answer placeholder, plain for the verbatim flavor and under the
 *  AI mark for the one the AI writes from (TMPL-43). */
export function AnswerStandIn({ label, topic }: { label: string; topic: string }) {
  const answer = (
    <div className="min-w-0 space-y-1">
      <Typography variant="label" as="p" className="break-words">
        {label}
      </Typography>
      <AnswerChip />
    </div>
  )
  if (topic === '') return answer
  return <WriteStandIn topic={topic}>{answer}</WriteStandIn>
}

export function FixedTextStandIn({ text }: { text: string }) {
  return (
    <Typography variant="body" as="p" className="break-words whitespace-pre-wrap">
      {text}
    </Typography>
  )
}

/** A 사진마다 반복: its passes under one mark, drawn twice by the caller to show that it repeats. */
export function RepeatStandIn({ children }: { children: ReactNode }) {
  const { t } = useTranslation('templates')
  return (
    <section
      aria-label={t('preview.repeat')}
      className="border-stroke-accent space-y-4 border-l-2 pl-3"
    >
      <Typography variant="meta" as="p" className="flex items-center gap-1.5">
        <Repeat2 aria-hidden className="size-3.5" />
        {t('preview.repeat')}
      </Typography>
      {children}
    </section>
  )
}
