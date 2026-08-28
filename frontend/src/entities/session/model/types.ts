/** The account behind the current session.
 *
 *  Deliberately not the generated `User` message: consumers (the header, the guard)
 *  should speak the app's vocabulary, so a proto change is absorbed by this entity's
 *  api mapper instead of rippling into every screen. */
export interface SessionUser {
  id: string
}
