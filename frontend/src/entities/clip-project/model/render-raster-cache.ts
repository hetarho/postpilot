import type { PreparedAsset } from './preview-assets'

/** A raster is decoded once while active, including every frame of a static caption. */
export class RenderRasterCache {
  private bitmaps = new Map<string, ImageBitmap>()
  constructor(private load: (asset: PreparedAsset) => Promise<ImageBitmap>) {}
  async get(asset: PreparedAsset) {
    let bitmap = this.bitmaps.get(asset.key)
    if (!bitmap) {
      bitmap = await this.load(asset)
      this.bitmaps.set(asset.key, bitmap)
    }
    return bitmap
  }
  retain(keys: Set<string>) {
    for (const [key, bitmap] of this.bitmaps) {
      if (!keys.has(key)) {
        bitmap.close()
        this.bitmaps.delete(key)
      }
    }
  }
}
