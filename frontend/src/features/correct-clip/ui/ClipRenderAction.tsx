import { useState } from 'react'
import { ArrowLeftRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { preferredClipRenderKind, type ClipRenderKind } from '@/entities/clip-project'
import { Button, Typography } from '@/shared/ui'

export function ClipRenderAction({
  lastKind,
  browserAvailable = false,
  browserRefusal,
  currentRender = false,
  pending,
  disabled,
  onRender,
}: {
  lastKind?: ClipRenderKind
  browserAvailable?: boolean
  browserRefusal?: 'capability' | 'memory'
  currentRender?: boolean
  pending: boolean
  disabled: boolean
  onRender: (kind: ClipRenderKind) => void
}) {
  const { t } = useTranslation('clips')
  const [chosen, setChosen] = useState<ClipRenderKind>()
  const kind = chosen ?? preferredClipRenderKind(lastKind, browserAvailable)
  const other = kind === 'browser' ? 'server' : 'browser'
  return (
    <>
      <Button
        variant="secondary"
        className="min-w-0 flex-1 sm:flex-none"
        pending={pending}
        disabled={disabled || (kind === 'browser' && !browserAvailable)}
        onClick={() => onRender(kind)}
      >
        {t(currentRender ? 'timeline.rerender' : 'timeline.render', {
          kind: t(`timeline.renderKinds.${kind}`),
        })}
      </Button>
      {(browserAvailable || kind === 'browser') && (
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('timeline.switchRenderKind', { kind: t(`timeline.renderKinds.${other}`) })}
          title={t('timeline.switchRenderKind', { kind: t(`timeline.renderKinds.${other}`) })}
          disabled={disabled || pending}
          onClick={() => setChosen(other)}
        >
          <ArrowLeftRight className="size-5" aria-hidden="true" />
        </Button>
      )}
      {browserRefusal && (
        <Typography variant="meta" as="p" className="order-last w-full" role="status">
          {t(`render.refusal.${browserRefusal}`)}
        </Typography>
      )}
    </>
  )
}
