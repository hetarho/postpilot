/** Build-time environment. VITE_* values are baked into the bundle, so only ever
 *  public configuration belongs here — never a secret. */

/** Backend origin for built assets. In dev this is unused: Vite proxies '/api' to the
 *  backend (vite.config.ts), which also sidesteps CORS. */
export const API_URL = import.meta.env.VITE_API_URL ?? ''
