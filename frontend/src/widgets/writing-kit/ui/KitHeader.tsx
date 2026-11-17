import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { Badge, typographyStyles } from '@/shared/ui'
import type { KitPart, KitPartKey } from '../model/readiness'
import { KIT_PART_INPUTS, kitPartFrom } from './useWritingKit'
import { Ring } from './WritingKit'
import { useKitStatusText } from './useKitStatusText'

type KitHeaderProps = {
  part: KitPartKey
  /** The screen's own heading when it is narrower than the part (영상 지침). */
  title?: string
  aside?: ReactNode
  className?: string
}

/** One reader per part, each calling only its own part's input hook. */
const READERS = Object.fromEntries(
  (Object.keys(KIT_PART_INPUTS) as KitPartKey[]).map((key) => [
    key,
    function Reader({ children }: { children: (part: KitPart) => ReactNode }) {
      const { user } = useSession()
      const useInput = KIT_PART_INPUTS[key]
      return children(kitPartFrom(key, useInput(user?.id ?? '')))
    },
  ]),
) as Record<KitPartKey, (props: { children: (part: KitPart) => ReactNode }) => ReactNode>

/** Guide: the head of a kit screen — the same ring the home's graphic draws for this part, in
 *  its real state, beside the part's name and what it does to a post. So a screen opens on what
 *  the thing IS (and how ready it is) before any list. `aside` is the screen's own figure or
 *  action, at the row's end. It reads only its own part's lists. */
export function KitHeader(props: KitHeaderProps) {
  const Reader = READERS[props.part]
  // Keyed by the part, so a header never swaps which lists it reads while mounted.
  return <Reader key={props.part}>{(part) => <KitHead {...props} state={part} />}</Reader>
}

function KitHead({
  part: key,
  state: part,
  title,
  aside,
  className,
}: KitHeaderProps & { state: KitPart }) {
  const { t } = useTranslation('creation')
  const status = useKitStatusText()(part)
  return (
    <div className={className}>
      <div className="flex min-w-0 flex-wrap items-center gap-4">
        <Ring part={part} size="lg" />
        <div className="min-w-48 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
            <h1 className={typographyStyles({ variant: 'largeTitle', className: 'break-keep' })}>
              {title ?? t(`kit.parts.${key}`)}
            </h1>
            {status && <Badge tone={part.state === 'ready' ? 'accent' : 'neutral'}>{status}</Badge>}
          </div>
          <p className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}>
            {t(`kit.does.${key}`)}
          </p>
        </div>
        {aside}
      </div>
    </div>
  )
}
