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
