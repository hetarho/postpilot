import { ArrowLeft } from 'lucide-react'
import { safeInternalPath } from '@/shared/lib/navigation'
import { buttonStyles } from '../button/buttonStyles'
import { useNavigationContext, type NavigationLinkProps } from './context'

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}
export function ContextualReturn() {
  const navigation = useNavigationContext()
  const destination = navigation?.returnTo ?? navigation?.ancestors.at(-1)
  if (!navigation || !destination || !safeInternalPath(destination.href)) return null
  const Link = navigation.Link ?? NativeLink
  return (
    <Link href={destination.href} className={buttonStyles({ variant: 'ghost' })}>
      <ArrowLeft aria-hidden="true" className="size-4" />
      {destination.label}
    </Link>
  )
}
