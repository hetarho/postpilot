import type { BrowserCompositionSnapshot } from './browser-composition'
import type { BrowserSourceAccess } from './browser-footage'
import type { BrowserMediaSourceAccess } from '@/shared/lib'
export interface ClipLocalCompositionRuntime {
  snapshot: BrowserCompositionSnapshot
  source: BrowserSourceAccess
  audioSource: (fingerprint: string, signal: AbortSignal) => Promise<BrowserMediaSourceAccess>
}
