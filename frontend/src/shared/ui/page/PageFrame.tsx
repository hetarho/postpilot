import {
  forwardRef,
  useCallback,
  useMemo,
  useState,
  type HTMLAttributes,
  type ReactNode,
} from 'react'
import { twMerge } from '@/shared/lib'
import { ContextualReturn } from '../navigation/ContextualReturn'
import { NavigationContext, useNavigationContext } from '../navigation/context'
import { useReturnClaim } from '../navigation/useReturnClaim'
import { pageStyles, type PageWidth } from './pageStyles'

/** The one frame of a screen: its column (`pageStyles`) and, at the column's top-left, the way
 *  back — the same round ← in the same place the writing guide wears it (THEME-75), so a screen
 *  never draws its own back link and never ends up with two.
 *
 *  The way back used to be drawn by the shell, in a fixed 5xl container above every child route,
 *  while each screen chose its own width: on a prose or board screen the arrow stood outside the
 *  content's edge, and a screen that wanted a return near its title drew a second one (the voice
 *  screen's `← 말투 목록`). Now the frame draws it inside the screen's own column and claims it
 *  from the shell, which keeps drawing it only for a screen not yet on a frame.
 *
 *  A screen whose large title sits in the frame's children puts that title in a `TitleRow`, which
 *  claims the ← from the frame in turn: the arrow then stands at the start of the title's row, and
 *  the frame draws its own row only when nothing below has taken it (a loading or failed state).
 *
 *  `actions` are the screen's secondary actions — 기본으로 설정, 삭제 — at the far end of the same
 *  row, where a directory card keeps them, instead of at the foot of the page. */
export const PageFrame = forwardRef<
  HTMLElement,
  {
    as?: 'main' | 'div'
    width?: PageWidth
    gutters?: boolean
    className?: string
    /** What this screen is — its heading, often with a control of its own (rename) — beside the
     *  ←, on the same row, so the row is not an arrow alone over an empty line. */
    title?: ReactNode
    actions?: ReactNode
    children?: ReactNode
  } & Omit<HTMLAttributes<HTMLElement>, 'className' | 'children' | 'title'>
>(function PageFrame(
  { as: Tag = 'main', width, gutters, className, title, actions, children, ...rest },
  ref,
) {
  const navigation = useNavigationContext()
  const showReturn = useReturnClaim()
  // A title row inside the frame that draws the ← itself.
  const [inner, setInner] = useState(0)
  const claimInner = useCallback(() => {
    setInner((count) => count + 1)
    return () => setInner((count) => count - 1)
  }, [])
  const framed = useMemo(
    () => (navigation ? { ...navigation, claimReturn: claimInner } : null),
    [navigation, claimInner],
  )
  const frameReturn = showReturn && inner === 0
  const row = frameReturn || !!actions || !!title
  return (
    <NavigationContext.Provider value={framed}>
      <Tag
        ref={ref as never}
        className={pageStyles({
          width,
          gutters,
          // A frame with a return starts where the writing guide's does.
          className: twMerge(className, (row || showReturn) && 'pt-3 sm:pt-6'),
        })}
        {...rest}
      >
        {row && (
          <div className="mb-4 flex min-h-11 min-w-0 flex-wrap items-center gap-2">
            {frameReturn && <ContextualReturn />}
            {title && <div className="min-w-0 flex-1">{title}</div>}
            {actions && (
              <div className="ml-auto flex min-w-0 flex-wrap items-center justify-end gap-2">
                {actions}
              </div>
            )}
          </div>
        )}
        {children}
      </Tag>
    </NavigationContext.Provider>
  )
})
