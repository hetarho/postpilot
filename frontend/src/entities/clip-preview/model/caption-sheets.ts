/** One run of a sequence-rendered caption's own frames, as the server drew them: a sprite
 *  sheet whose cells are the output frames from `frameOffset` on (CLIP-159). */
export interface CaptionFramePage {
  sheet: Uint8Array
  cellWidth: number
  cellHeight: number
  columns: number
  cells: number
  /** Where a cell sits on the canvas — the origin the server's own overlay uses. */
  x: number
  y: number
  /** The caption's first output frame, and the frame this run starts at. */
  firstFrame: number
  frameOffset: number
  /** The next frame offset to ask for, or -1 once the caption has no more. */
  nextOffset: number
}

export type CaptionFrameLoader = (
  instanceId: string,
  frameOffset: number,
  signal: AbortSignal,
) => Promise<CaptionFramePage>

export interface CaptionCell {
  bitmap: ImageBitmap
  x: number
  y: number
  width: number
  height: number
}

/** The sheets a browser render draws its sequence captions from. The encode walks the output
 *  frames in order, so a caption holds ONE decoded sheet at a time: the run that covers the
 *  frame being drawn, dropped as soon as the walk moves past it. A clip's captions are never
 *  all in memory at once, which is what lets a long clip render on a phone (CLIP-155). */
export class CaptionSheets {
  private held = new Map<string, { page: CaptionFramePage; bitmap: ImageBitmap }>()
  constructor(private load: CaptionFrameLoader) {}

  /** The cell for this caption at this OUTPUT frame, or undefined outside the caption's own
   *  frames. Throws the loader's failure, which refuses the render rather than delivering a
   *  caption that stands still (CLIP-155). */
  async cell(
    instanceId: string,
    outputFrame: number,
    signal: AbortSignal,
  ): Promise<CaptionCell | undefined> {
    let entry = this.held.get(instanceId)
    if (!entry || !covers(entry.page, outputFrame)) {
      const previous = entry
      const page = await this.load(instanceId, await this.offsetFor(entry, outputFrame), signal)
      if (!page.cells || page.cellWidth <= 0 || page.cellHeight <= 0 || page.columns <= 0)
        throw new Error('CLIP_CAPTION_FRAMES_UNAVAILABLE')
      if (!covers(page, outputFrame)) return undefined
      const bitmap = await createImageBitmap(
        new Blob([new Uint8Array(page.sheet)], { type: 'image/png' }),
      )
      previous?.bitmap.close()
      entry = { page, bitmap }
      this.held.set(instanceId, entry)
    }
    const { page, bitmap } = entry
    const cell = outputFrame - page.firstFrame - page.frameOffset
    const column = cell % page.columns
    const row = Math.floor(cell / page.columns)
    return {
      bitmap: await createImageBitmap(
        bitmap,
        column * page.cellWidth,
        row * page.cellHeight,
        page.cellWidth,
        page.cellHeight,
      ),
      x: page.x,
      y: page.y,
      width: page.cellWidth,
      height: page.cellHeight,
    }
  }

  /** A caption resumes where the walk is rather than starting over: the first run tells this
   *  side which output frame the caption opens on. */
  private async offsetFor(
    entry: { page: CaptionFramePage } | undefined,
    outputFrame: number,
  ): Promise<number> {
    if (!entry) return 0
    return Math.max(0, outputFrame - entry.page.firstFrame)
  }

  dispose() {
    for (const entry of this.held.values()) entry.bitmap.close()
    this.held.clear()
  }
}

function covers(page: CaptionFramePage, outputFrame: number) {
  const cell = outputFrame - page.firstFrame - page.frameOffset
  return cell >= 0 && cell < page.cells
}
