import { useTranslation } from 'react-i18next'
import { preferredClipRenderKind, type ClipRenderKind } from '@/entities/clip-project'
import type { MyPlan } from '@/entities/plan'
import { Button, Popover, Typography } from '@/shared/ui'

/** ②'s render control (CLIP-40, CLIP-153): ONE trigger that opens the choice of kind — a browser
 *  render or a server one — rather than a button naming a kind beside a switch to the other. The
 *  supported browser leads independently of the last successful kind, and a browser the device
 *  cannot render on keeps its option, refused, with the reason beside it (CLIP-155). */
export function ClipRenderAction({
  lastKind,
  browserAvailable = false,
  browserRefusal,
  serverWindow,
  serverPlan,
  serverEntitled = false,
  currentRender = false,
  pending,
  disabled,
  variant = 'secondary',
  onRender,
}: {
  lastKind?: ClipRenderKind
  browserAvailable?: boolean
  browserRefusal?: 'capability' | 'memory'
  serverWindow?: MyPlan['serverExportWindow']
  serverPlan?: MyPlan['plan']
  serverEntitled?: boolean
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
  const existingServerResult = currentRender && lastKind === 'server'
  const serverNeedsPlan = !serverEntitled && !existingServerResult
  const serverExhausted =
    serverEntitled &&
    serverPlan !== 'master' &&
    (!serverWindow || serverWindow.remaining <= 0) &&
    !existingServerResult
  const renewal = serverWindow?.endsAt ? new Date(serverWindow.endsAt).toLocaleString() : ''
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
            const refused =
              kind === 'browser' ? !browserAvailable : serverNeedsPlan || serverExhausted
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
                  {kind === 'server' && existingServerResult
                    ? t('render.serverExisting')
                    : kind === 'server' && serverNeedsPlan
                      ? t(
                          serverPlan && serverPlan !== 'max'
                            ? 'render.serverMaxRequired'
                            : 'render.serverAccessUnknown',
                        )
                      : kind === 'server' && serverWindow
                        ? t('render.serverBalance', {
                            used: serverWindow.used,
                            reserved: serverWindow.reserved,
                            remaining: serverWindow.remaining,
                            allowance: serverWindow.allowance,
                          })
                        : refused && kind === 'browser' && browserRefusal
                          ? t(`render.refusal.${browserRefusal}`)
                          : t(`render.kindHelp.${kind}`)}
                </Typography>
                {kind === 'server' && (serverExhausted || serverNeedsPlan) && (
                  <Typography variant="meta" as="p" role="status">
                    {serverExhausted &&
                      (renewal
                        ? t('render.serverRenewal', { at: renewal })
                        : t('render.serverNoAllowance'))}
                    {serverNeedsPlan && serverPlan && (
                      <>
                        {' '}
                        <a href="/plans">{t('render.serverUpgrade')}</a>
                      </>
                    )}
                    {browserAvailable && <> {t('render.serverBrowserOption')}</>}
                  </Typography>
                )}
              </div>
            )
          })}
        </div>
      )}
    </Popover>
  )
}
