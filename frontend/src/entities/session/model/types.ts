import type { PlanName } from '@/entities/plan/@x/session'

/** The account behind the current session.
 *
 *  Deliberately not the generated `User` message: consumers (the header, the guard)
 *  should speak the app's vocabulary, so a proto change is absorbed by this entity's
 *  api mapper instead of rippling into every screen. */
export interface SessionUser {
  id: string
  /** The account's tier, resolved with the session so master-only surfaces can be gated on
   *  boot without a second round-trip. Undefined when the server sent a tier this build does
   *  not know — which every gate must read as "not allowed", never as a default tier. */
  plan: PlanName | undefined
}
