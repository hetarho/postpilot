// libheif-js ships type declarations only for the raw Emscripten module, not for the
// small decoder API layered on top of it in the same bundle. This is the part we use.
declare module 'libheif-js/libheif-wasm/libheif-bundle.mjs' {
  /** Anything with an RGBA byte buffer of `width × height × 4` — an `ImageData`, or a
   *  plain object where `ImageData` does not exist (a worker on an older engine). */
  export interface RgbaTarget {
    readonly data: Uint8ClampedArray
    readonly width: number
    readonly height: number
  }

  export interface HeifImage {
    get_width(): number
    get_height(): number
    is_primary(): boolean
    /** Decodes into `target`; `callback` receives `target`, or `null` on failure. */
    display(target: RgbaTarget, callback: (result: RgbaTarget | null) => void): void
    free(): void
  }

  export class HeifDecoder {
    /** Empty when the buffer is not a HEIF container this build can parse. */
    decode(buffer: ArrayBuffer | Uint8Array): HeifImage[]
  }

  export interface Libheif {
    HeifDecoder: typeof HeifDecoder
    heif_get_version(): string
  }

  /** Synchronous: the WASM binary is embedded in this bundle, so nothing is fetched. */
  export default function createLibheif(options?: Record<string, unknown>): Libheif
}
