import { buttonStyles } from '../button/buttonStyles'

/** The shared presentation for route and step returns. */
export function returnControlStyles(className?: string) {
  return buttonStyles({ variant: 'secondary', size: 'icon', className })
}
