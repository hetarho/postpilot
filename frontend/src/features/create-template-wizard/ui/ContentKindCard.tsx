import { useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Image as ImageIcon, NotebookPen, Sparkles } from 'lucide-react'
import { AnswerChip, PreviewLines, type SectionContent } from '@/entities/template'
import { RadioTile, Typography, cardStyles, radioTiles } from '@/shared/ui'
import type { SectionDraft } from '../model/wizard-machine'

const KINDS = ['write', 'ask', 'literal'] as const satisfies readonly SectionContent[]

const KIND_LABEL = {
  write: 'contentWrite',
  ask: 'contentAsk',
  literal: 'contentLiteral',
} as const
const KIND_HELP = {
  write: 'contentWriteHelp',
  ask: 'contentAskHelp',
  literal: 'contentLiteralHelp',
} as const

/** What stands in a section (TMPL-74), as one card: three radios across its top choose the kind,
 *  and the card below them changes with the choice. It draws this section at the two moments an
 *  owner who does not know the workflow cannot picture — 글 쓸 때, what the post's author meets on
 *  ①, and 완성된 글, what the post shows — from the section's own heading, question and text, and
 *  then holds what the chosen kind asks the owner for. Shared radio tiles preserve one roving tab stop and
 *  arrow-key choice; the chosen kind's plain sentence describes the group. */
export function ContentKindCard({
  section,
  onChange,
  inputs = {},
}: {
  section: SectionDraft
  onChange: (content: SectionContent) => void
  /** What a kind asks the owner for, shown inside the card while that kind is chosen. */
  inputs?: Partial<Record<SectionContent, ReactNode>>
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  const kind = section.content
  const choices = radioTiles(KINDS.length, KINDS.indexOf(kind), (index) => onChange(KINDS[index]))
  return (
    <fieldset className="min-w-0" aria-describedby={`${id}-help`}>
      <legend>
        <Typography variant="label" as="span">
          {t('wizard.section.content')}
        </Typography>
      </legend>
      <div className={cardStyles({ padding: 'sm', className: 'mt-2 min-w-0' })}>
        <div
          {...choices.group}
          aria-label={t('wizard.section.content')}
          className="grid grid-cols-3 gap-2"
        >
          {KINDS.map((option, index) => (
            <RadioTile
              value={option}
              key={option}
              {...choices.tile(index, 'items-center justify-center gap-2')}
              aria-describedby={`${id}-help`}
            >
              <Typography
                variant="label"
                as="span"
                className={
                  option === kind
                    ? 'text-content-primary min-w-0 text-center'
                    : 'min-w-0 text-center'
                }
              >
                {t(`wizard.section.${KIND_LABEL[option]}`)}
              </Typography>
            </RadioTile>
          ))}
        </div>
        <span id={`${id}-help`} className="sr-only">
          {t(`wizard.section.${KIND_HELP[kind]}`)}
        </span>
        <div aria-hidden className="mt-4 grid grid-cols-2 gap-2">
          <Moment caption={t('wizard.section.kinds.writing')}>
            <Writing kind={kind} section={section} />
          </Moment>
          <Moment caption={t('wizard.section.kinds.result')}>
            <Typography variant="label" as="span" className="text-content-primary truncate">
              {section.heading.trim() || section.name}
            </Typography>
            <Result kind={kind} section={section} />
          </Moment>
        </div>
        {inputs[kind] && <div className="mt-4">{inputs[kind]}</div>}
      </div>
    </fieldset>
  )
}

function Moment({ caption, children }: { caption: string; children: ReactNode }) {
  return (
    <span className="bg-card-well flex min-w-0 flex-col gap-2 rounded-lg p-3">
      <Typography variant="eyebrow" as="span">
        {caption}
      </Typography>
      {children}
    </span>
  )
}

/** What the post's author meets on ① for this kind. */
function Writing({ kind, section }: { kind: SectionContent; section: SectionDraft }) {
  const { t } = useTranslation('templates')
  if (kind === 'ask')
    return (
      <span className="flex min-w-0 flex-col gap-1">
        <Typography variant="meta" as="span" className="text-content-secondary truncate">
          {section.question.trim() || section.name}
        </Typography>
        <span className="bg-field-bg flex min-w-0 items-center gap-1 rounded-md px-3 py-2">
          <span className="bg-button-cta-bg h-3 w-px shrink-0" />
          <Typography variant="meta" as="span" className="truncate">
            {t('wizard.section.kinds.askField')}
          </Typography>
        </span>
      </span>
    )
  return (
    <span className="flex min-w-0 flex-col gap-1.5">
      {kind === 'write' && (
        <span className="text-content-tertiary flex items-center gap-1.5">
          <NotebookPen className="size-4" />
          <ImageIcon className="size-4" />
        </span>
      )}
      <Typography variant="meta" as="span">
        {t(kind === 'write' ? 'wizard.section.kinds.writeMemo' : 'wizard.section.kinds.nothing')}
      </Typography>
    </span>
  )
}

/** What the finished post shows in this kind's place. */
function Result({ kind, section }: { kind: SectionContent; section: SectionDraft }) {
  const { t } = useTranslation('templates')
  switch (kind) {
    case 'write':
      return (
        <span className="flex min-w-0 flex-col gap-1.5">
          <Sparkles className="text-content-tertiary size-3.5" />
          <PreviewLines lines={2} />
        </span>
      )
    case 'ask':
      // The AI writes the part, from what the author typed (TMPL-74).
      return (
        <span className="flex min-w-0 flex-col gap-1.5">
          <span className="flex min-w-0 items-center gap-1.5">
            <Sparkles className="text-content-tertiary size-3.5 shrink-0" />
            <AnswerChip>{t('wizard.section.kinds.askAnswer')}</AnswerChip>
          </span>
          <PreviewLines lines={2} />
        </span>
      )
    case 'literal':
      return section.literal.trim() !== '' ? (
        <Typography variant="meta" as="span" className="text-content-primary line-clamp-2">
          {section.literal.trim()}
        </Typography>
      ) : (
        <AnswerChip>{t('wizard.section.kinds.literalText')}</AnswerChip>
      )
  }
}
