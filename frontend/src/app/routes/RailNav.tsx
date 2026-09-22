import type { ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import { clsx } from 'clsx'
import type { StageName } from '@/entities/model-catalog'
import { typographyStyles } from '@/shared/ui'

export interface RailItem {
  to: string
  label: string
  icon: ComponentType<{ className?: string }>
  search?: { stage?: StageName }
  /** `group` is a destination of the open group, drawn one step in from the primary row that
   *  opened it. */
  level: 'primary' | 'group'
  current: boolean
}

/** ONE sidebar holds both levels (owner decision 2026-09-22): the primary destinations, and,
 *  directly under whichever of them the address is inside, that group's own destinations. A
 *  second rail beside the first said the same thing twice and cost the page 12rem to do it, and
 *  the group's list had nowhere to come from until `DESTINATIONS` carried the group itself.
 *
 *  The two levels are told apart by DENSITY and INDENT, never by a different control (THEME-38):
 *  a group row is shorter, its glyph smaller, and it starts past the primary glyph so the column
 *  of names reads as a list under a heading rather than as two lists interleaved.
 *
 *  Selection is decided by the caller, not by each `Link` from its own href: 글 is current on
 *  `/voices`, which does not share its address, and 모델 비교 is current on an experiment reached
 *  from it. */
const ROW = {
  primary: 'flex min-h-11 items-center gap-3 rounded-md px-4',
  // 36px is THEME-23's fine-pointer floor for a menu row; the touch floor is added back for a
  // rail under a thumb. The type role does not shrink with the box — a destination's own name is
  // copy the user acts on — so the step down lives in the box and the glyph alone.
  group: 'flex min-h-9 items-center gap-2 rounded-md py-2 pr-3 pl-9 pointer-coarse:min-h-11',
} as const
const GLYPH = { primary: 'size-5 shrink-0', group: 'size-4 shrink-0' } as const
/** Folded, a rail keeps the destination and drops only its NAME: one 44px square per glyph at
 *  both levels, because the folded column is one width and a narrower row inside it would read as
 *  misaligned rather than as denser. The name is still the link's accessible name — it is
 *  `aria-label` at every width — and `title` hands it to a pointer as well. */
const FOLDED = 'group relative flex size-11 items-center justify-center rounded-md'

/** The name a folded row cannot show, drawn by the app rather than left to the browser's own
 *  `title` (design-language §7: no OS-drawn surface). It appears beside the glyph on hover and on
 *  keyboard focus, is `aria-hidden` because the link already carries the same string as its
 *  accessible name, and takes no pointer events so it can never sit between the pointer and the
 *  destination it names. */
const TOOLTIP = typographyStyles({
  variant: 'meta',
  className:
    'bg-surface-highest text-content-primary pointer-events-none absolute left-full z-30 ml-2 hidden rounded-md px-2 py-1 whitespace-nowrap shadow-lg group-hover:block group-focus-visible:block',
})

/** What tells the two levels apart at a glance, now that they share one column (owner report
 *  2026-09-22: the old pair of planes read as one list).
 *
 *  A PRIMARY row always carries a plane of its own, one step up from the rail, and a heavier
 *  weight: it is the head of the section under it, whether or not the address is inside it. A
 *  GROUP row carries no plane at rest — it sits on the rail itself, indented — so the column
 *  reads as headings with their lists rather than as one run of rows. Current still moves a row
 *  one plane further and turns it the current colour, and the group's current plane is the
 *  brightest thing in the rail because it is the most specific answer to "where am I". */
const PLANE = {
  primary: {
    rest: 'bg-surface-recessed font-medium hover:bg-surface-base hover:text-link-fg-hover active:bg-surface-base', // style-escape: the section head's own plane
    current: 'bg-surface-base text-link-fg-current font-semibold', // style-escape: current navigation emphasis
  },
  group: {
    rest: 'hover:bg-surface-recessed active:bg-surface-recessed hover:text-link-fg-hover',
    current: 'bg-surface-raised text-link-fg-current font-medium', // style-escape: current navigation emphasis
  },
} as const

/** One answer for both `activeProps` and `inactiveProps`, so the router's own prefix guess never
 *  reaches the DOM. */
const selected = (current: boolean) => ({
  'aria-current': current ? ('page' as const) : undefined,
})

export function RailNav({ items, collapsed }: { items: readonly RailItem[]; collapsed: boolean }) {
  return items.map((item) => (
    <Link
      key={`${item.level}:${item.to}`}
      to={item.to}
      // The one list holds both levels, and a group's home repeats its primary destination's
      // address (내 글 IS /posts). Nothing downstream — a test, a style, a future query — can tell
      // the two rows apart by href, so the row says which level it is.
      data-nav-level={item.level}
      search={item.search}
      aria-label={item.label}
      // The caller has already decided, so the router must not decide too. `exact` stops its own
      // prefix match — 모델 변경 (/ai-models) is NOT current on /ai-models/compare, where 모델 비교
      // is — and both props then carry the one answer, which is also how 글 stays marked on
      // /voices, an address it does not own.
      activeOptions={{ exact: true, includeSearch: false }}
      activeProps={selected(item.current)}
      inactiveProps={selected(item.current)}
      className={typographyStyles({
        variant: 'label',
        className: clsx(
          'text-link-fg whitespace-normal',
          collapsed ? FOLDED : ROW[item.level],
          item.current ? PLANE[item.level].current : PLANE[item.level].rest,
        ),
      })}
    >
      <item.icon aria-hidden="true" className={GLYPH[item.level]} />
      {collapsed ? (
        <span aria-hidden="true" className={TOOLTIP}>
          {item.label}
        </span>
      ) : (
        item.label
      )}
    </Link>
  ))
}
