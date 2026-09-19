/** How long the HEIC decoder worker stays alive after its last file. Its WASM heap does
 *  not shrink after a 12 MP decode, so it is not kept for a whole session; the chunk is
 *  in the browser cache, so bringing it back for the next batch is cheap. */
export const HEIF_DECODER_IDLE_MS = 30_000
