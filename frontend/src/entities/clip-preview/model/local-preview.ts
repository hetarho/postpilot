import type { BrowserCompositionSnapshot } from './browser-composition'
import type { BrowserSourceAccess } from './browser-footage'
import type { BrowserMediaSourceAccess } from '@/shared/lib'
export interface ClipLocalCompositionRuntime {
  /** Private intent evidence; never a renderer/cache or authorization identity. */
  continuity?: { signal: AbortSignal; leaseEpoch: number; retainedUntil?: number }
  snapshot: BrowserCompositionSnapshot
  source: BrowserSourceAccess
  audioSource: (fingerprint: string, signal: AbortSignal) => Promise<BrowserMediaSourceAccess>
}
