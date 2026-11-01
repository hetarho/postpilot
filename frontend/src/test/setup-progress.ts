/** Browser metadata for established route-test accounts; isolated auth adapters do not write it. */
export function seedCompletedSetup(ownerId: string): void {
  localStorage.setItem(
    'postpilot.setup.v1.' + encodeURIComponent(ownerId),
    JSON.stringify({
      version: 1,
      ownerId,
      completed: true,
      skipped: [],
      resume: 'welcome',
      target: '/',
    }),
  )
}

/** The same accounts have answered `다시 묻지 않기` on both creation offers (POST-121, CLIP-223), so
 *  a suite about something else never writes under an open sheet; a suite about the offer opts
 *  in with `creationOffers`. */
export function seedDismissedCreationOffers(ownerId: string): void {
  for (const kind of ['post', 'clip'])
    localStorage.setItem(`postpilot.creation-offer.v1.${kind}.${encodeURIComponent(ownerId)}`, '1')
}
