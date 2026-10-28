import { useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Image as ImageIcon, NotebookPen, Sparkles } from 'lucide-react'
import { AnswerChip, PreviewLines, type SectionContent } from '@/entities/template'
import { Typography } from '@/shared/ui'
import type { SectionDraft } from '../model/wizard-machine'

const KINDS = ['write', 'ask', 'literal'] as const satisfies readonly SectionContent[]

/** What stands in a section (TMPL-74), offered as three cards that each draw this section at the
 *  two moments an owner who does not know the workflow cannot picture: 글 쓸 때, what the post's
 *  author meets on ①, and 완성된 글, what the post shows. The drawings use the section's own
 *  heading, question and text; the plain sentence each card stands for is its accessible
 *  description. Native radios inside the cards give the arrow keys for free. */
export function ContentKindCards({
  section,
  onChange,
}: {
  section: SectionDraft
  onChange: (content: SectionContent) => void
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  return (
    <fieldset className="min-w-0">
      <legend>
        <Typography variant="label" as="span">
          {t('wizard.section.content')}
        </Typography>
      </legend>
      <div className="mt-2 grid gap-3">
        {KINDS.map((kind) => (
          <label
            key={kind}
            className="bg-surface-raised border-divider has-[:checked]:border-stroke-accent has-[:checked]:ring-stroke-accent has-[:focus-visible]:outline-focus-ring duration-base ease-standard flex min-w-0 cursor-pointer flex-col gap-3 rounded-xl border p-4 transition-colors has-[:checked]:ring-1 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2"
          >
            <span className="flex items-center gap-3">
              <input
                type="radio"
                name={`${id}-content`}
                value={kind}
                checked={section.content === kind}
                aria-labelledby={`${id}-${kind}-name`}
                aria-describedby={`${id}-${kind}-help`}
                onChange={() => onChange(kind)}
                className="accent-button-cta-bg size-4 shrink-0 focus-visible:outline-none"
              />
              <Typography variant="fieldTitle" as="span" id={`${id}-${kind}-name`}>
                {t(`wizard.section.${KIND_LABEL[kind]}`)}
              </Typography>
            </span>
            <span id={`${id}-${kind}-help`} className="sr-only">
              {t(`wizard.section.${KIND_HELP[kind]}`)}
            </span>
            <span aria-hidden className="grid grid-cols-2 gap-2">
              <Moment caption={t('wizard.section.kinds.writing')}>
                <Writing kind={kind} section={section} />
              </Moment>
              <Moment caption={t('wizard.section.kinds.result')}>
                <Typography variant="label" as="span" className="text-content-primary truncate">
                  {section.heading.trim() || section.name}
                </Typography>
                <Result kind={kind} section={section} />
              </Moment>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}

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

function Moment({ caption, children }: { caption: string; children: ReactNode }) {
  return (
    <span className="bg-surface-base border-divider flex min-w-0 flex-col gap-2 rounded-lg border p-2.5">
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
        <span className="border-stroke-accent bg-surface-raised flex items-center gap-1 rounded-md border px-2 py-1.5">
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
      return <AnswerChip>{t('wizard.section.kinds.askAnswer')}</AnswerChip>
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
