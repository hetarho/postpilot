import { ChevronDown, ChevronRight } from 'lucide-react'
import { safeInternalPath } from '@/shared/lib/navigation'
import { Popover } from '../popover/Popover'
import { ContextualReturn } from './ContextualReturn'
import { useNavigationContext, type NavigationLinkProps } from './context'

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}
export function Breadcrumb({
  ariaLabel,
  compact = false,
}: {
  ariaLabel: string
  compact?: boolean
}) {
  const navigation = useNavigationContext()
  if (!navigation) return null
  const Link = navigation.Link ?? NativeLink
  const ancestors = navigation.ancestors.filter((item) => safeInternalPath(item.href))
  const contextualReturn =
    navigation.returnTo &&
    safeInternalPath(navigation.returnTo.href) &&
    navigation.returnTo.href !== ancestors.at(-1)?.href
  const hierarchy = (
    <ol className="text-content-secondary flex flex-wrap items-center gap-2 text-sm">
      {ancestors.map((item) => (
        <li key={item.href} className="flex items-center gap-2">
          <Link
            href={item.href}
            className="hover:text-content-primary inline-flex min-h-11 min-w-11 items-center rounded-sm px-2"
          >
            {item.label}
          </Link>
          <ChevronRight aria-hidden="true" className="size-4 shrink-0" />
        </li>
      ))}
      <li aria-current="page" className="text-content-primary">
        {navigation.current}
      </li>
    </ol>
  )
  if (compact)
    return (
      <nav aria-label={ariaLabel} className="flex min-w-0 justify-center">
        {ancestors.length > 0 || contextualReturn ? (
          <Popover
            label={`${ariaLabel}: ${navigation.current}`}
            triggerVariant="ghost"
            triggerClassName="min-w-0 max-w-full gap-1 px-1"
            className="max-w-full min-w-0"
            placement="below"
            align="start"
            triggerLabel={
              <>
                <span className="truncate">{navigation.current}</span>
                <ChevronDown aria-hidden="true" className="size-4 shrink-0" />
              </>
            }
          >
            {() => (
              <>
                {hierarchy}
                {contextualReturn && <ContextualReturn />}
              </>
            )}
          </Popover>
        ) : (
          <span aria-current="page" className="min-w-0 truncate text-sm font-medium">
            {navigation.current}
          </span>
        )}
      </nav>
    )
  return <nav aria-label={ariaLabel}>{hierarchy}</nav>
}
