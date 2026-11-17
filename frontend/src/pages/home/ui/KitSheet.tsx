import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import { TemplateOutline } from '@/entities/template'
import { Typography, typographyStyles } from '@/shared/ui'
import type { KitElement, KitReadiness } from '../model/readiness'
import type { WritingKit } from '../model/useWritingKit'
import { KIT_TONES } from '../model/kit-tones'
import { KitMarker } from './kit-marks'

/** Guide: the readiness graphic — one sheet of paper that IS the post the kit writes, drawn from
 *  the owner's own data. The template is its skeleton (the template's own headings, through the
 *  shared `TemplateOutline`); under the headings the voice writes the sentence (its own sample,
 *  habits marked), the memories are the facts highlighted in the text, and the rules are the
 *  stamps in the margin. Each part carries its mark in the left gutter, in the element's ink; a
 *  part that is not ready is drawn as its dashed hole, so the owner sees what their post lacks
 *  rather than a percentage. A second sheet lies under it, and the halo behind it warms as the kit
 *  fills — still shapes, with motion only in the halos.
 *
 *  The drawing is one image to assistive technology (its label lists every part's state); the
 *  legend beside it (`KitLegend`) is where each part is opened. */
export function KitSheet({ data }: { data: WritingKit }) {
  const { t } = useTranslation('creation')
  const settled = data.summary.settled
  const next = settled ? data.summary.next : undefined
  const item = (element: KitElement) =>
    data.kit.find((entry) => entry.element === element) as KitReadiness
  const states = data.kit
    .map((entry) => `${t(`home.elements.${entry.element}.name`)} ${t(`home.level.${entry.level}`)}`)
    .join(', ')
  const parts: { element: Exclude<KitElement, 'template'>; body: ReactNode }[] = [
    { element: 'voice', body: <VoiceBody item={item('voice')} data={data} /> },
    { element: 'memory', body: <MemoryBody item={item('memory')} data={data} /> },
    { element: 'guideline', body: <GuidelineBody item={item('guideline')} data={data} /> },
  ]
  const template = item('template')
  return (
    <div
      role="img"
      aria-label={t('home.sheetLabel', { states })}
      className="relative isolate mx-auto w-full max-w-lg"
    >
      {/* The halo: the kit's warmth grows with what is ready. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -inset-6 -z-20 sm:-inset-10"
        style={{ opacity: 0.2 + 0.12 * data.summary.ready }}
      >
        <div className="bg-promo-aurora-1 animate-promo-glow absolute top-0 left-0 size-2/3 rounded-full blur-3xl" />
        <div className="bg-promo-aurora-2 animate-promo-glow absolute right-0 bottom-0 size-2/3 rounded-full blur-3xl [animation-delay:1.6s]" />
      </div>
      {/* The post under this one: the kit writes every post, not one. */}
      <div
        aria-hidden="true"
        className="bg-surface-raised ring-divider absolute inset-x-5 top-4 -bottom-3 -z-10 rotate-[2.5deg] rounded-[1.75rem] shadow-md ring-1"
      />
      <div
        aria-hidden="true"
        className="bg-surface-raised ring-divider relative isolate overflow-hidden rounded-[1.75rem] px-5 pt-5 pb-6 shadow-xl ring-1 sm:px-7 sm:pt-6 sm:pb-8"
      >
        <PostHead caption={t('home.sheet')} />
        <div className="relative mt-5 pl-10">
          {/* The template shapes the post from its title down: its mark stands at the title. */}
          <KitMarker
            item={template}
            next={next === 'template'}
            className="absolute -top-1.5 left-0"
          />
          <span
            className={clsx(
              'block h-4 w-4/5 rounded-full',
              template.level === 'ready' || template.level === 'loading'
                ? 'bg-content-tertiary/25'
                : DASHED,
            )}
          />
          <span className="mt-6 block" />
          {!settled ? (
            <LoadingBody />
          ) : (
            <TemplateOutline
              template={template.level === 'ready' ? data.template?.first : undefined}
              max={parts.length}
              fill
            >
              {(index) => (
                <Part item={item(parts[index].element)} next={next === parts[index].element}>
                  {parts[index].body}
                </Part>
              )}
            </TemplateOutline>
          )}
        </div>
      </div>
    </div>
  )
}

/** The top of a post: who wrote it, and its title — the parts no kit element decides. */
function PostHead({ caption }: { caption: string }) {
  return (
    <span className="flex flex-col">
      <span className="flex items-center gap-3">
        <span className="bg-badge-accent-bg block size-8 shrink-0 rounded-full" />
        <span className="flex min-w-0 flex-1 flex-col gap-1.5">
          <span className="bg-divider block h-2 w-20 rounded-full" />
          <span className="bg-divider block h-2 w-12 rounded-full opacity-70" />
        </span>
        <Typography variant="eyebrow" as="span" className="shrink-0">
          {caption}
        </Typography>
      </span>
    </span>
  )
}

/** One part of the post a kit element writes: its mark in the gutter, a bracket in its ink down
 *  the part, and what it puts on the page. */
function Part({
  item,
  next,
  children,
}: {
  item: KitReadiness
  next: boolean
  children: ReactNode
}) {
  return (
    <span className="relative block">
      <KitMarker item={item} next={next} className="absolute top-0 -left-10" />
      <span
        className={clsx(
          'absolute top-9 bottom-0 -left-[1.6875rem] w-0.5 rounded-full',
          item.level === 'ready' || item.level === 'partial'
            ? [KIT_TONES[item.element].line, item.level === 'ready' ? 'opacity-40' : 'opacity-20']
            : 'bg-divider',
        )}
      />
      <span className="block min-h-7">{children}</span>
    </span>
  )
}

function Bars({ widths, className }: { widths: number[]; className?: string }) {
  return (
    <span className="flex flex-col gap-2">
      {widths.map((width, index) => (
        <span
          key={index}
          className={clsx('block h-2.5 rounded-full', className ?? 'bg-divider')}
          style={{ width: `${width}%` }}
        />
      ))}
    </span>
  )
}

const DASHED = 'border-content-tertiary/50 border border-dashed bg-transparent'

function LoadingBody() {
  return (
    <span className="loading-reveal flex flex-col gap-5">
      {[0, 1, 2].map((index) => (
        <span key={index} className="flex flex-col gap-2">
          <span className="bg-divider block h-3 w-2/5 rounded-full" />
          <Bars widths={[94, 72]} />
        </span>
      ))}
    </span>
  )
}

/** The voice writes the sentence: its own sample with its habits marked, or the lines it will
 *  write once it is made. */
function VoiceBody({ item, data }: { item: KitReadiness; data: WritingKit }) {
  const voice = data.voice
  if (item.level === 'ready' && voice?.sample)
    return (
      <span className="flex flex-col gap-2">
        <span
          className={typographyStyles({
            variant: 'body',
            className: 'text-content-primary line-clamp-3 break-words',
          })}
        >
          <Marked text={voice.sample} phrases={voice.phrases} />
        </span>
        {!voice.phrases.some((phrase) => voice.sample?.includes(phrase)) &&
          voice.phrases.length > 0 && (
            <span className="flex flex-wrap gap-1.5">
              {voice.phrases.slice(0, 2).map((phrase) => (
                <span
                  key={phrase}
                  className={typographyStyles({
                    variant: 'meta',
                    className:
                      'bg-voice-habit-highlight text-voice-habit-foreground max-w-full truncate rounded-md px-2 py-0.5',
                  })}
                >
                  {phrase}
                </span>
              ))}
            </span>
          )}
      </span>
    )
  if (item.level === 'ready')
    return <Bars widths={[96, 90, 58]} className="bg-voice-habit-highlight" />
  if (item.level === 'partial')
    return (
      <span className="flex flex-col gap-3">
        <Bars widths={[96, 90, 58]} />
        {item.percent !== undefined && (
          <span className="bg-meter-track block h-1 w-full overflow-hidden rounded-full">
            <span
              className="bg-kit-ink-voice block h-full rounded-full"
              style={{ width: `${Math.min(100, item.percent)}%` }}
            />
          </span>
        )}
      </span>
    )
  return <Bars widths={[96, 90, 58]} className={DASHED} />
}

/** The voice's habit phrases, marked where they occur in its sample. */
function Marked({ text, phrases }: { text: string; phrases: string[] }) {
  const found = phrases.filter((phrase) => phrase && text.includes(phrase))
  if (!found.length) return <>{text}</>
  const pattern = new RegExp(
    `(${found.map((phrase) => phrase.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`,
  )
  return (
    <>
      {text.split(pattern).map((piece, index) =>
        found.includes(piece) ? (
          <mark
            key={index}
            className="bg-voice-habit-highlight text-voice-habit-foreground rounded px-0.5"
          >
            {piece}
          </mark>
        ) : (
          piece
        ),
      )}
    </>
  )
}

/** The memories are the facts the post is told: highlighted in the running text. */
function MemoryBody({ item, data }: { item: KitReadiness; data: WritingKit }) {
  if (item.level === 'ready' && data.memories.shown.length > 0)
    return (
      <span className="flex flex-col gap-2">
        <Bars widths={[88]} />
        <span className="flex min-w-0 flex-wrap items-center gap-1.5">
          {data.memories.shown.slice(0, 2).map((memory) => (
            <span
              key={memory.id}
              className={typographyStyles({
                variant: 'body',
                className:
                  'bg-origin-owner-highlight text-origin-owner-foreground max-w-full truncate rounded-md px-2',
              })}
            >
              {memory.text}
            </span>
          ))}
          {data.memories.count > 2 && (
            <span className={typographyStyles({ variant: 'meta' })}>
              +{data.memories.count - 2}
            </span>
          )}
        </span>
        <Bars widths={[64]} />
      </span>
    )
  return (
    <span className="flex flex-col gap-2">
      <Bars widths={[88]} />
      <span className="flex items-center gap-2">
        <span className="bg-divider block h-2.5 w-1/5 rounded-full" />
        <span className={clsx('block h-6 w-2/5 rounded-md', DASHED)} />
        <span className="bg-divider block h-2.5 w-1/4 rounded-full" />
      </span>
    </span>
  )
}

/** The rules are what the post was checked against: stamps under its last lines. */
function GuidelineBody({ item, data }: { item: KitReadiness; data: WritingKit }) {
  const shown = data.guidelines.shown.slice(0, 2)
  return (
    <span className="flex flex-col gap-2.5">
      <Bars widths={[92, 70]} />
      {(item.level === 'ready' || item.level === 'partial') && shown.length > 0 ? (
        <span className="flex min-w-0 flex-wrap gap-1.5">
          {shown.map((rule) => (
            <span
              key={rule.key}
              className={typographyStyles({
                variant: 'meta',
                className: clsx(
                  'inline-flex max-w-full items-center gap-1 rounded-full px-2.5 py-1',
                  rule.builtIn
                    ? 'ring-origin-ai-foreground/40 text-content-secondary ring-1 ring-inset'
                    : 'bg-origin-ai-highlight text-origin-ai-foreground',
                ),
              })}
            >
              <Check className="size-3.5 shrink-0" />
              <span className="truncate">{rule.text}</span>
            </span>
          ))}
        </span>
      ) : (
        <span className="flex gap-1.5">
          <span className={clsx('block h-7 w-24 rounded-full', DASHED)} />
          <span className={clsx('block h-7 w-16 rounded-full', DASHED)} />
        </span>
      )}
    </span>
  )
}
