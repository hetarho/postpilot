/** Whether the browser asks for reduced motion. Injectable so a test can say yes without owning
 *  `window.matchMedia`; absent (jsdom, a worker) means no such preference is known. */
export function prefersReducedMotion(
  matchMedia: ((query: string) => { matches: boolean }) | undefined = globalThis.matchMedia,
): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}
