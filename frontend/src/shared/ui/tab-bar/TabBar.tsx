import type { ComponentType } from 'react'
import { clsx } from 'clsx'
import { useNavigationContext, type NavigationLinkProps } from '../navigation/context'

export interface TabBarItem {
  href: string
  label: string
  icon: ComponentType<{ className?: string }>
  current: boolean
}

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}

/** Guide: the app's primary destinations as a bottom tab bar — the thumb's home row on a phone —
 *  that becomes a centred floating pill on the desk (from `lg:`). Every destination is visible at
 *  every width; there is no hamburger (THEME-26). A tab's label is the 13px meta step at every
 *  width — copy the owner acts on, so never smaller (THEME-19) — and the bar's padding is the
 *  home-indicator inset (THEME-31).
 *
 *  The current destination is told three ways: the accent ink, a soft pill behind its glyph and
 *  `aria-current="page"`. The shell that mounts this marks itself `data-tab-bar="shown"` so every
 *  dock in the page floats above the bar instead of under it. */
export function TabBar({ items, ariaLabel }: { items: readonly TabBarItem[]; ariaLabel: string }) {
  const Link = useNavigationContext()?.Link ?? NativeLink
  return (
    <nav
      aria-label={ariaLabel}
      className="fixed inset-x-0 bottom-0 z-30 lg:inset-x-auto lg:bottom-6 lg:left-1/2 lg:-translate-x-1/2"
    >
      <ul
        className={clsx(
          'bg-tab-bar-bg pb-safe-b grid auto-cols-[minmax(0,1fr)] grid-flow-col rounded-t-2xl px-1 shadow-lg backdrop-blur-xl',
          'lg:flex lg:gap-1 lg:rounded-full lg:p-1.5',
        )}
      >
        {items.map(({ href, label, icon: Icon, current }) => (
          <li key={href} className="min-w-0">
            <Link
              href={href}
              aria-current={current ? 'page' : undefined}
              className={clsx(
                'group duration-fast ease-standard text-meta flex h-16 min-w-0 flex-col items-center justify-center gap-1 rounded-xl leading-none font-semibold transition-colors',
                'lg:h-12 lg:flex-row lg:gap-2 lg:rounded-full lg:px-5',
                current
                  ? 'text-tab-bar-fg-current lg:bg-tab-bar-indicator'
                  : 'text-tab-bar-fg hover:text-content-primary active:text-content-primary lg:hover:bg-card-bg-hover',
              )}
            >
              <span
                aria-hidden="true"
                className={clsx(
                  'flex h-8 w-full max-w-14 items-center justify-center rounded-full transition-colors lg:h-auto lg:w-auto',
                  current && 'bg-tab-bar-indicator lg:bg-transparent',
                )}
              >
                <Icon className="size-5.5 lg:size-5" />
              </span>
              <span className="max-w-full truncate">{label}</span>
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  )
}
