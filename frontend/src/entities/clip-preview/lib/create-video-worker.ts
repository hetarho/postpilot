/** The worker enters the pure model directly, without loading UI/API barrel exports. */
export function createClipVideoWorker() {
  return new Worker(new URL('./video.worker.ts', import.meta.url), { type: 'module' })
}
