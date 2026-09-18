import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { preferredClipRenderKind, type ClipRenderKind } from '@/entities/clip-project'
import { Button } from '@/shared/ui'

export function ClipRenderAction({
  lastKind,
  browserAvailable = false,
  currentRender = false,
  pending,
  disabled,
  onRender,
}: {
  lastKind?: ClipRenderKind
  browserAvailable?: boolean
  currentRender?: boolean
  pending: boolean
  disabled: boolean
  onRender: (kind: ClipRenderKind) => void
}) {
  const { t } = useTranslation('clips')
  const [chosen, setChosen] = useState<ClipRenderKind>()
  const kind = preferredClipRenderKind(chosen ?? lastKind, browserAvailable)
  const other = kind === 'browser' ? 'server' : 'browser'
  return (
    <>
      <Button
        variant="secondary"
        pending={pending}
        disabled={disabled}
        onClick={() => onRender(kind)}
      >
        {t(currentRender ? 'timeline.rerender' : 'timeline.render', {
          kind: t(`timeline.renderKinds.${kind}`),
        })}
      </Button>
      {browserAvailable && (
        <Button variant="ghost" disabled={disabled || pending} onClick={() => setChosen(other)}>
          {t('timeline.switchRenderKind', { kind: t(`timeline.renderKinds.${other}`) })}
        </Button>
      )}
    </>
  )
}
