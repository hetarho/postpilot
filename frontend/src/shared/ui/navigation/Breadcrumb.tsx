import { ChevronRight } from 'lucide-react'
import { safeInternalPath } from '@/shared/lib/navigation'
import { useNavigationContext, type NavigationLinkProps } from './context'

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}
export function Breadcrumb({ ariaLabel }: { ariaLabel: string }) {
  const navigation = useNavigationContext()
  if (!navigation) return null
  const Link = navigation.Link ?? NativeLink
  return (
    <nav aria-label={ariaLabel}>
      <ol className="text-content-secondary flex flex-wrap items-center gap-2 text-sm">
        {navigation.ancestors
          .filter((item) => safeInternalPath(item.href))
          .map((item) => (
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
    </nav>
  )
}
