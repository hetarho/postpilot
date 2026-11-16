import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { NotebookPen, PenLine, Sparkles } from 'lucide-react'
import { Button, Checkbox, Sheet, Typography, typographyStyles } from '@/shared/ui'
import { contentKindHelpHidden, rememberContentKindHelpHidden } from '../lib/content-kind-help'

const KINDS = [
  { key: 'write', icon: Sparkles },
  { key: 'ask', icon: NotebookPen },
  { key: 'literal', icon: PenLine },
] as const

/** What 이 자리에 들어갈 것's three choices mean, in more words than the radios have room for:
 *  what the post's author does when writing, what the finished post shows, and when each fits.
 *  It opens by itself on the first part's screen and from the ⓘ beside the choice. */
export function ContentKindHelp({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation('templates')
  const id = useId()
  const [hidden, setHidden] = useState(contentKindHelpHidden)
  return (
    <Sheet
      open={open}
      labelledBy={`${id}-title`}
      onClose={onClose}
      header={
        <Typography variant="title" as="h2" id={`${id}-title`}>
          {t('wizard.section.help.title')}
        </Typography>
      }
      footer={
        <div className="mt-6 flex flex-col gap-3">
          <label
            className={typographyStyles({
              variant: 'body',
              className: 'flex min-h-11 items-center gap-3',
            })}
          >
            <Checkbox
              checked={hidden}
              onChange={(event) => {
                setHidden(event.target.checked)
                rememberContentKindHelpHidden(event.target.checked)
              }}
            />
            {t('wizard.section.help.hide')}
          </label>
          <Button variant="cta" className="w-full" onClick={onClose}>
            {t('wizard.section.help.close')}
          </Button>
        </div>
      }
    >
      <Typography variant="body" className="text-content-secondary">
        {t('wizard.section.help.intro')}
      </Typography>
      <ul className="mt-5 space-y-5">
        {KINDS.map(({ key, icon: Icon }) => (
          <li key={key} className="flex gap-3">
            <span className="bg-card-well text-content-secondary flex size-10 shrink-0 items-center justify-center rounded-full">
              <Icon aria-hidden className="size-5" />
            </span>
            <span className="min-w-0 space-y-1">
              <Typography variant="fieldTitle" as="span" className="block">
                {t(`wizard.section.help.${key}.name`)}
              </Typography>
              <Typography variant="body" as="span" className="block">
                {t(`wizard.section.help.${key}.what`)}
              </Typography>
              <Typography variant="meta" as="span" className="text-content-secondary block">
                {t(`wizard.section.help.${key}.when`)}
              </Typography>
            </span>
          </li>
        ))}
      </ul>
    </Sheet>
  )
}
