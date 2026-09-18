import { useTranslation } from 'react-i18next'
import { preferredClipRenderKind, type ClipRenderKind } from '@/entities/clip-project'
import { Button, Popover, Typography } from '@/shared/ui'

/** ②'s render control (CLIP-40, CLIP-153): ONE trigger that opens the choice of kind — a browser
 *  render or a server one — rather than a button naming a kind beside a switch to the other. The
 *  last successful kind leads (browser on a project that has none yet), and a browser the device
 *  cannot render on keeps its option, refused, with the reason beside it (CLIP-155). */
export function ClipRenderAction({
  lastKind,
  browserAvailable = false,
  browserRefusal,
  currentRender = false,
  pending,
  disabled,
  variant = 'secondary',
  onRender,
}: {
  lastKind?: ClipRenderKind
  browserAvailable?: boolean
  browserRefusal?: 'capability' | 'memory'
  currentRender?: boolean
  pending: boolean
  disabled: boolean
  /** `cta` while a render is the step's next action — before the project has one — and
   *  `secondary` once 확정하기 stands beside it. */
  variant?: 'cta' | 'secondary'
  onRender: (kind: ClipRenderKind) => void
}) {
  const { t } = useTranslation('clips')
  const preferred = preferredClipRenderKind(lastKind, browserAvailable)
  const kinds: ClipRenderKind[] =
    preferred === 'browser' ? ['browser', 'server'] : ['server', 'browser']
  const label = t(currentRender ? 'timeline.rerender' : 'timeline.render')
  return (
    <Popover
      label={label}
      triggerLabel={label}
      triggerVariant={variant}
      triggerPending={pending}
      disabled={disabled}
      placement="above"
      align="end"
      phone="sheet"
    >
      {(close) => (
        <div className="grid gap-4">
          <Typography variant="body">{t('render.choose')}</Typography>
          {kinds.map((kind) => {
            const refused = kind === 'browser' && !browserAvailable
            return (
              <div key={kind} className="grid gap-1">
                <Button
                  variant={kind === preferred ? 'cta' : 'secondary'}
                  className="w-full"
                  disabled={refused}
                  onClick={() => {
                    onRender(kind)
                    close()
                  }}
                >
                  {t(`render.kind.${kind}`)}
                </Button>
                <Typography variant="meta" as="p" role={refused ? 'status' : undefined}>
                  {refused && browserRefusal
                    ? t(`render.refusal.${browserRefusal}`)
                    : t(`render.kindHelp.${kind}`)}
                </Typography>
              </div>
            )
          })}
        </div>
      )}
    </Popover>
  )
}
