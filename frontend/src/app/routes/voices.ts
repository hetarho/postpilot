import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { defaultVoice, loadVoices } from '@/entities/voice'
import { authenticatedRoute, writingGroupRoute, type RouterContext } from './tree'

// The three voice tabs all name the same specifier, so the bundler emits one chunk for the
// whole tab area rather than three: the tabs are one screen and are always reached
// together. VoiceLayout is lazy too but stays its own 2 kB chunk — it belongs to app/routes,
// not to the pages/voice slice, and sharing their chunk would mean moving it across layers.
const lazyVoice = <K extends 'VoiceAnalysisPage' | 'VoiceMaterialsPage'>(name: K) =>
  lazyRouteComponent(() => import('@/pages/voice'), name)

/** The tabs an old `/voice/<tab>` link may name, so the redirect keeps the user on the same
 *  screen of the default voice. Anything else lands on the profile tab. */
const LEGACY_VOICE_TABS = new Set(['materials'])

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
    case 'materials':
      throw redirect({ to: '/voices/$voiceId/materials', params, replace: true })
    default:
      throw redirect({ to: '/voices/$voiceId', params, replace: true })
  }
}

export const voicesRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/voices',
  component: lazyRouteComponent(() => import('@/pages/voices'), 'VoicesPage'),
})

// The layout of one voice: the three tabs keep their own addresses under `/voices/$voiceId` and
// share the tab row.
export const voiceLayoutRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/voices/$voiceId',
  component: lazyRouteComponent(() => import('./VoiceLayout'), 'VoiceLayout'),
})

export const voiceRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/',
  component: lazyVoice('VoiceAnalysisPage'),
})

export const voiceMaterialsRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/materials',
  component: lazyVoice('VoiceMaterialsPage'),
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

/** The directory, which is NOT a tab of one voice. */
export const voiceRoutes = [voicesRoute]
/** The tabs under `/voices/$voiceId`, which share the tab row. */
export const voiceTabRoutes = [voiceRoute, voiceMaterialsRoute]
/** The address the app had before voices were plural. */
export const legacyVoiceRoutes = [legacyVoiceRoute, legacyVoiceTabRoute]
