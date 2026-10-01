import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import { Typography } from '@/shared/ui'
import { TEMPLATE_ASK_MAX_PER_BODY, TEMPLATE_PHOTO_ROW_MAX } from '../config'
import {
  decode,
  parse,
  parseTemplate,
  type ParseFailure,
  type TemplateArea,
  type TemplateNode,
} from '../lib/grammar'
import { TEMPLATE_PARSE_OPTIONS } from '../model/types'

/** How many passes a 사진마다 반복 is drawn with: two show that it repeats, and nothing on a
 *  hardcoded preview asks to be operated (TMPL-66). */
const REPEAT_PASSES = 2

interface TemplatePreviewProps {
  titleArea: string
  body: string
  className?: string
}

/** The post a template makes, drawn the way a post's reading view draws one, from placeholder
 *  stand-ins and nothing else (TMPL-65, TMPL-66).
 *
 *  It is client work over the one draft: no model call, no request, no credit, no grammar
 *  syntax and neither generation number. While an area does not parse it keeps showing that
 *  area's last parsable state, with the failure noted, so a half-typed tag in 원문 does not make
 *  the post the author is shaping disappear. */
export function TemplatePreview({ titleArea, body, className }: TemplatePreviewProps) {
  const { t } = useTranslation('templates')
  const title = useLastParsed(titleArea, true)
  const nodes = useLastParsed(body, false)
  const whole = parseTemplate(titleArea, body, TEMPLATE_PARSE_OPTIONS)
  const failure = whole.ok ? null : whole.failure
  const empty = (title ?? []).length === 0 && !hasContent(nodes ?? [])

  return (
    <article aria-label={t('preview.heading')} className={twMerge('min-w-0', className)}>
      <Typography variant="eyebrow" as="p">
        {t('preview.heading')}
      </Typography>
      <Typography variant="meta" as="p" className="text-content-tertiary mt-1">
        {t('preview.note')}
      </Typography>
      {failure && <ParseNotice failure={failure} />}
      {empty && !failure ? (
        <Typography variant="body" as="p" className="text-content-secondary mt-8">
          {t('preview.empty')}
        </Typography>
      ) : (
        <>
          {title && title.length > 0 && (
            <Typography variant="title" as="h3" className="mt-8 break-words">
              <InlineNodes nodes={title} />
            </Typography>
          )}
          {nodes && (
            <div className="mt-8 space-y-5">
              <BlockNodes nodes={nodes} />
            </div>
          )}
        </>
      )}
    </article>
  )
}

/** The last nodes an area parsed to, or null before it ever has. Kept as state updated while
 *  rendering — React's own pattern for a value derived from the previous render — so nothing
 *  outside this component has to remember it. */
function useLastParsed(text: string, titleArea: boolean): TemplateNode[] | null {
  const [last, setLast] = useState<{ text: string; nodes: TemplateNode[] } | null>(null)
  const result = parse(text, { ...TEMPLATE_PARSE_OPTIONS, titleArea })
  if (result.ok && last?.text !== text) {
    setLast({ text, nodes: result.nodes })
    return result.nodes
  }
  return result.ok ? result.nodes : (last?.nodes ?? null)
}

function hasContent(nodes: readonly TemplateNode[]): boolean {
  return nodes.some((node) => node.kind !== 'literal' || decode(node.text ?? '').trim() !== '')
}

function ParseNotice({ failure }: { failure: ParseFailure }) {
  const { t } = useTranslation('templates')
  const reason = t(`builder.reasons.${failure.reason}`, {
    max: TEMPLATE_PHOTO_ROW_MAX,
    askMax: TEMPLATE_ASK_MAX_PER_BODY,
  })
  return (
    <Typography variant="meta" as="p" role="status" className="text-content-secondary mt-3">
      {t('preview.unparsed', {
        area: t(`source.area.${failure.area satisfies TemplateArea}`),
        line: failure.line,
        reason,
      })}
    </Typography>
  )
}

/** A body's nodes as the blocks of a post. */
function BlockNodes({ nodes }: { nodes: readonly TemplateNode[] }) {
  const { t } = useTranslation('templates')
  const blocks: ReactNode[] = []
  nodes.forEach((node, index) => {
    const key = `${index}-${node.line}`
    switch (node.kind) {
      case 'literal': {
        const text = decode(node.text ?? '').trim()
        if (text !== '') blocks.push(<FixedText key={key} text={text} />)
        break
      }
      case 'write':
        blocks.push(<WriteBox key={key} topic={decode(node.text ?? '')} />)
        break
      case 'ask':
        blocks.push(<AskStandIn key={key} node={node} />)
        break
      case 'slot':
        blocks.push(
          node.slotKind === 'photo' ? (
            <PhotoRow key={key} count={node.count ?? 1} />
          ) : (
            <FixedText
              key={key}
              text={legacyLabel(node, {
                place: t('builder.legacy.place'),
                link: t('builder.legacy.link'),
              })}
            />
          ),
        )
        break
      case 'repeat':
        blocks.push(
          <section
            key={key}
            aria-label={t('preview.repeat')}
            className="border-divider space-y-5 border-l-2 pl-3"
          >
            <Typography variant="meta" as="p" className="text-content-tertiary">
              {t('preview.repeat')}
            </Typography>
            {Array.from({ length: REPEAT_PASSES }, (_, pass) => (
              <div key={pass} className="space-y-5">
                <BlockNodes nodes={node.children ?? []} />
              </div>
            ))}
          </section>,
        )
        break
    }
  })
  return <>{blocks}</>
}

/** The title area's nodes on one title line: the same stand-ins, inline. */
function InlineNodes({ nodes }: { nodes: readonly TemplateNode[] }) {
  const { t } = useTranslation('templates')
  return (
    <>
      {nodes.map((node, index) => {
        const key = `${index}-${node.line}`
        switch (node.kind) {
          case 'literal':
            return <span key={key}>{decode(node.text ?? '')}</span>
          case 'write':
            return (
              <span
                key={key}
                className="bg-surface-recessed text-content-secondary rounded-sm px-1.5"
              >
                {decode(node.text ?? '')}
              </span>
            )
          case 'ask': {
            const topic = decode(node.text ?? '')
            return (
              <span
                key={key}
                className="bg-surface-recessed text-content-secondary rounded-sm px-1.5"
              >
                {topic === ''
                  ? t('preview.answerOf', { label: decode(node.label ?? '') })
                  : `${topic} · ${t('preview.answerOf', { label: decode(node.label ?? '') })}`}
              </span>
            )
          }
          default:
            return null
        }
      })}
    </>
  )
}

function FixedText({ text }: { text: string }) {
  return (
    <Typography variant="body" as="p" className="break-words whitespace-pre-wrap">
      {text}
    </Typography>
  )
}

/** What the AI writes: a grey paragraph naming what its place is about, never sample prose. */
function WriteBox({ topic, children }: { topic: string; children?: ReactNode }) {
  const { t } = useTranslation('templates')
  return (
    <div className="bg-surface-recessed rounded-lg px-4 py-3">
      <Typography variant="meta" as="p" className="text-content-tertiary">
        {t('preview.aiWrites')}
      </Typography>
      <Typography variant="body" as="p" className="text-content-secondary mt-1 break-words">
        {topic}
      </Typography>
      {children}
    </div>
  )
}

/** A data field: its 제목 over an 입력한 내용 placeholder, plain for the verbatim flavor and inside
 *  the grey box for the one the AI writes from (TMPL-43). */
function AskStandIn({ node }: { node: TemplateNode }) {
  const { t } = useTranslation('templates')
  const label = decode(node.label ?? '')
  const topic = decode(node.text ?? '')
  const answer = (
    <Typography variant="label" as="p" className="mt-1 break-words">
      {label}
      <span className="text-content-tertiary"> · {t('preview.answer')}</span>
    </Typography>
  )
  if (topic === '') return <div>{answer}</div>
  return <WriteBox topic={topic}>{answer}</WriteBox>
}

function PhotoRow({ count }: { count: number }) {
  const { t } = useTranslation('templates')
  return (
    <div
      role="img"
      aria-label={t('preview.photos', { count })}
      className="grid gap-2"
      style={{ gridTemplateColumns: `repeat(${count}, minmax(0, 1fr))` }}
    >
      {Array.from({ length: count }, (_, cell) => (
        <div key={cell} className="bg-surface-recessed aspect-square rounded-lg" />
      ))}
    </div>
  )
}

/** A stored place/link position reads as the 고정 문구 it is in the builder (TMPL-37). */
function legacyLabel(node: TemplateNode, names: { place: string; link: string }): string {
  const label = decode(node.label ?? '').trim()
  if (label !== '') return label
  return node.slotKind === 'link' ? names.link : names.place
}
