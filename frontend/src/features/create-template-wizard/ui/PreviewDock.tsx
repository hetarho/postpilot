import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'
import { Button, Sheet, Typography } from '@/shared/ui'

/** The wizard's preview below `lg` (TMPL-75): a `미리보기` button that floats at the viewport's
 *  foot while the step scrolls and settles above Back and next, opening the preview in a bottom
 *  sheet. It glows once each time an answer changes the preview — `changes` counts them — and the
 *  global reduced-motion rule ends that glow before it is seen. */
export function PreviewDock({ changes, children }: { changes: number; children: ReactNode }) {
  const { t } = useTranslation('templates')
  const id = useId()
  const [open, setOpen] = useState(false)
  const close = () => setOpen(false)
  return (
    <>
      <div className="pointer-events-none sticky bottom-4 z-10 mt-6 flex justify-end lg:hidden">
        <span className="pointer-events-auto relative inline-flex">
          {changes > 0 && (
            <span
              key={changes}
              aria-hidden
              data-testid="preview-glow"
              className="bg-promo-stroke animate-preview-glow absolute -inset-1.5 rounded-full opacity-0 blur-md"
            />
          )}
          <Button
            variant="secondary"
            onClick={() => setOpen(true)}
            className="relative rounded-full"
          >
            <Sparkles aria-hidden className="size-4" />
            {t('wizard.preview.open')}
          </Button>
        </span>
      </div>
      <Sheet
        open={open}
        labelledBy={`${id}-title`}
        size="wide"
        header={
          <Typography variant="title" id={`${id}-title`}>
            {t('wizard.preview.open')}
          </Typography>
        }
        footer={
          <Button variant="secondary" onClick={close}>
            {t('wizard.preview.close')}
          </Button>
        }
        onClose={close}
      >
        {children}
      </Sheet>
    </>
  )
}
