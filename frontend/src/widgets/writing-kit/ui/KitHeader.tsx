import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { Badge, typographyStyles } from '@/shared/ui'
import type { KitPart, KitPartKey } from '../model/readiness'
import { KIT_PART_INPUTS, KIT_VIDEO_INPUTS, kitPartFrom } from './useWritingKit'
import { Ring } from './WritingKit'
import { useKitStatusText } from './useKitStatusText'

type KitHeaderProps = {
  part: KitPartKey
  /** The 영상 side of 템플릿 or 지침: its own name and line, its state read from the video lists. */
  video?: boolean
  /** The screen's own heading when it is narrower than the part. */
  title?: string
  aside?: ReactNode
  className?: string
}

type Reader = (props: { children: (part: KitPart) => ReactNode }) => ReactNode
function reader(
  key: KitPartKey,
  useInput: (ownerId: string) => ReturnType<typeof KIT_PART_INPUTS.voice>,
): Reader {
  return function Reader({ children }) {
    const { user } = useSession()
    return children(kitPartFrom(key, useInput(user?.id ?? '')))
  }
}
/** One reader per part (and per 영상 side), each calling only its own part's input hook. */
const READERS = Object.fromEntries(
  (Object.keys(KIT_PART_INPUTS) as KitPartKey[]).map((key) => [
    key,
    reader(key, KIT_PART_INPUTS[key]),
  ]),
) as Record<KitPartKey, Reader>
const VIDEO_READERS: Record<'template' | 'guideline', Reader> = {
  template: reader('template', KIT_VIDEO_INPUTS.template),
  guideline: reader('guideline', KIT_VIDEO_INPUTS.guideline),
}

/** Guide: the head of a kit screen — the same ring the home's graphic draws for this part, in
 *  its real state, beside the part's name and what it does to a post. So a screen opens on what
 *  the thing IS (and how ready it is) before any list. `aside` is the screen's own figure or
 *  action, at the row's end. It reads only its own part's lists. */
export function KitHeader(props: KitHeaderProps) {
  const video = props.video && (props.part === 'template' || props.part === 'guideline')
  const Reader = video ? VIDEO_READERS[props.part as 'template' | 'guideline'] : READERS[props.part]
  const key = `${props.part}:${video ? 'video' : 'post'}`
  // Keyed by the part and side, so a header never swaps which lists it reads while mounted.
  return <Reader key={key}>{(part) => <KitHead {...props} video={video} state={part} />}</Reader>
}

function KitHead({
  part: key,
  state: part,
  video,
  title,
  aside,
  className,
}: KitHeaderProps & { state: KitPart }) {
  const { t } = useTranslation('creation')
  const side = video ? (key as 'template' | 'guideline') : undefined
  const status = useKitStatusText()(part)
  return (
    <div className={className}>
      <div className="flex min-w-0 flex-wrap items-center gap-4">
        <Ring part={part} size="lg" />
        <div className="min-w-48 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
            <h1 className={typographyStyles({ variant: 'largeTitle', className: 'break-keep' })}>
              {title ?? (side ? t(`kit.videoParts.${side}`) : t(`kit.parts.${key}`))}
            </h1>
            {/* A long voice name gives way inside the chip, not past the screen's edge. */}
            {status && (
              <Badge
                tone={part.state === 'ready' ? 'accent' : 'neutral'}
                title={status}
                className="max-w-full min-w-0"
              >
                <span className="min-w-0 truncate">{status}</span>
              </Badge>
            )}
          </div>
          <p className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}>
            {side ? t(`kit.videoDoes.${side}`) : t(`kit.does.${key}`)}
          </p>
        </div>
        {aside}
      </div>
    </div>
  )
}
