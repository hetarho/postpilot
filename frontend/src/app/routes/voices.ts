import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { defaultVoice, loadVoices } from '@/entities/voice'
import { authenticatedRoute, writingGroupRoute, type RouterContext } from './tree'

// The five voice tabs all name the same specifier, so the bundler emits one 23 kB chunk for
// the whole tab area rather than five: the tabs are one screen and are always reached
// together. VoiceLayout is lazy too but stays its own 2 kB chunk — it belongs to app/routes,
// not to the pages/voice slice, and sharing their chunk would mean moving it across layers.
const lazyVoice = <
  K extends
    | 'VoicePage'
    | 'VoiceVersionsPage'
    | 'VoiceImportPage'
    | 'VoiceRulesPage'
    | 'VoiceValidationsPage',
>(
  name: K,
) => lazyRouteComponent(() => import('@/pages/voice'), name)

/** The tabs an old `/voice/<tab>` link may name, so the redirect keeps the user on the same
 *  screen of the default voice. Anything else lands on the profile tab. */
const LEGACY_VOICE_TABS = new Set(['versions', 'import', 'rules', 'validations'])

/** Sends an old `/voice` address to the same tab of the account's default voice — read from the
 *  directory, never created here — or to the directory itself when there is none to show. Always
 *  throws (a redirect), so a caller's `await` is the whole guard. */
export async function redirectLegacyVoice(
  context: RouterContext & { user: { id: string } },
  tab: string,
): Promise<never> {
  const voices = await loadVoices(context.queryClient, context.transport, context.user.id).catch(
    () => [],
  )
  const target = defaultVoice(voices)
  if (!target) throw redirect({ to: '/voices', replace: true })
  const params = { voiceId: target.id }
  switch (LEGACY_VOICE_TABS.has(tab) ? tab : '') {
    case 'versions':
      throw redirect({ to: '/voices/$voiceId/versions', params, replace: true })
    case 'import':
      throw redirect({ to: '/voices/$voiceId/import', params, replace: true })
    case 'rules':
      throw redirect({ to: '/voices/$voiceId/rules', params, replace: true })
    case 'validations':
      throw redirect({ to: '/voices/$voiceId/validations', params, replace: true })
    default:
      throw redirect({ to: '/voices/$voiceId', params, replace: true })
  }
}

export const voicesRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/voices',
  component: lazyRouteComponent(() => import('@/pages/voices'), 'VoicesPage'),
})

// The layout of one voice: the five tabs keep their own addresses under `/voices/$voiceId` and
// share the tab row. The two detail screens further down stay OUTSIDE it — they are full-width
// review surfaces with their own back link, not a sixth tab.
export const voiceLayoutRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/voices/$voiceId',
  component: lazyRouteComponent(() => import('./VoiceLayout'), 'VoiceLayout'),
})

export const voiceRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/',
  component: lazyVoice('VoicePage'),
})

export const voiceVersionsRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/versions',
  component: lazyVoice('VoiceVersionsPage'),
})

export const voiceImportRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/import',
  component: lazyVoice('VoiceImportPage'),
})

export const voiceRulesRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/rules',
  component: lazyVoice('VoiceRulesPage'),
})

export const voiceValidationsRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/validations',
  component: lazyVoice('VoiceValidationsPage'),
})

export const voiceRuleComparisonRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/voices/$voiceId/rules/$id/compare',
  component: lazyRouteComponent(
    () => import('@/pages/voice-rule-comparison'),
    'VoiceRuleComparisonPage',
  ),
})

export const voiceValidationRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/voices/$voiceId/validations/$id',
  component: lazyRouteComponent(() => import('@/pages/voice-validation'), 'VoiceValidationPage'),
})

// The address the app had before voices were plural. Bookmarks and the empty-profile warning
// of an older draft still point here.
export const legacyVoiceRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice',
  beforeLoad: async ({ context }) => {
    await redirectLegacyVoice(context, '')
  },
})

export const legacyVoiceTabRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice/$',
  beforeLoad: async ({ context, params }) => {
    await redirectLegacyVoice(context, params._splat ?? '')
  },
})

/** The directory and the two full-width review screens, which are NOT tabs of one voice. */
export const voiceRoutes = [voicesRoute, voiceRuleComparisonRoute, voiceValidationRoute]
/** The five tabs under `/voices/$voiceId`, which share the tab row. */
export const voiceTabRoutes = [
  voiceRoute,
  voiceVersionsRoute,
  voiceImportRoute,
  voiceRulesRoute,
  voiceValidationsRoute,
]
/** The address the app had before voices were plural. */
export const legacyVoiceRoutes = [legacyVoiceRoute, legacyVoiceTabRoute]
