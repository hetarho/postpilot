export type NavigationMenuState = 'closed' | 'open'
export type NavigationMenuEvent = 'open' | 'close' | 'toggle' | 'navigate'

export function navigationMenuTransition(
  state: NavigationMenuState,
  event: NavigationMenuEvent,
): NavigationMenuState {
  switch (event) {
    case 'open':
      return 'open'
    case 'close':
    case 'navigate':
      return 'closed'
    case 'toggle':
      return state === 'open' ? 'closed' : 'open'
  }
}
